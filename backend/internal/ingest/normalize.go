package ingest

import (
	"context"
	"fmt"
	"io"
	"strings"

	"ptexplorer/internal/store"
	"ptexplorer/internal/tic"
)

// FileStats counts what was read from one file.
type FileStats struct {
	Items          int `json:"items"`
	Rates          int `json:"rates"`
	Prices         int `json:"prices"`
	ProviderGroups int `json:"provider_groups"`
	Skipped        int `json:"skipped"`          // elements that failed to decode
	DanglingRefs   int `json:"dangling_refs"`    // provider_references ids with no match
	RemoteRefs     int `json:"remote_refs"`      // references stored at another URL (not followed)
	NoProviderRows int `json:"no_provider_rows"` // prices with no resolvable provider
}

// pendingRate is a negotiated_rate whose provider references are resolved after the
// whole file is read, since provider_references may come after in_network.
type pendingRate struct {
	firstRow, endRow int
	refs             []int64
	inline           []int32
}

// normalizer collects one file's providers and rows while it is parsed.
type normalizer struct {
	data    store.FileData
	stats   FileStats
	refs    map[int64][]int32 // provider_group_id -> local provider indexes
	pending []pendingRate
	skips   []string
}

const maxSkipSamples = 5

// Result is one parsed file: normalized data, counts, and a few sample skip errors.
type Result struct {
	Data        *store.FileData
	Stats       FileStats
	SkipSamples []string
}

// Parser is the in-process file parser. It satisfies loader.Parser; a remote parsing
// service could satisfy the same interface instead.
type Parser struct{}

// Parse reads one in-network file into normalized store data.
func (Parser) Parse(ctx context.Context, r io.Reader) (*Result, error) {
	data, stats, skips, err := Load(ctx, r)
	if err != nil {
		return nil, err
	}
	return &Result{Data: data, Stats: stats, SkipSamples: skips}, nil
}

// Load parses one in-network file into normalized store data.
func Load(ctx context.Context, r io.Reader) (*store.FileData, FileStats, []string, error) {
	n := &normalizer{
		data: store.FileData{Meta: map[string]string{}},
		refs: map[int64][]int32{},
	}
	err := Parse(ctx, r, Handler{
		OnMeta:              func(k, v string) { n.data.Meta[k] = v },
		OnProviderReference: n.addReference,
		OnItem:              n.addItem,
		OnSkip: func(section string, i int, err error) {
			n.stats.Skipped++
			if len(n.skips) < maxSkipSamples {
				n.skips = append(n.skips, fmt.Sprintf("%s[%d]: %v", section, i, err))
			}
		},
	})
	if err != nil {
		return nil, n.stats, n.skips, err
	}
	n.resolve()
	return &n.data, n.stats, n.skips, nil
}

// addProvider stores one TIN group (deduplicating NPIs) and returns its local index.
func (n *normalizer) addProvider(g tic.ProviderGroup, groupID int64, networks []string) int32 {
	npis := make([]int64, 0, len(g.NPI))
	seen := map[int64]bool{}
	for _, v := range g.NPI {
		if v > 0 && !seen[int64(v)] {
			seen[int64(v)] = true
			npis = append(npis, int64(v))
		}
	}
	tin := strings.TrimSpace(string(g.TIN.Value))
	n.data.Providers = append(n.data.Providers, store.Provider{
		GroupID:      groupID,
		TINType:      strings.ToLower(strings.TrimSpace(g.TIN.Type)),
		TIN:          tin,
		TINDigits:    store.DigitsOnly(tin),
		BusinessName: strings.TrimSpace(g.TIN.BusinessName),
		NPIs:         npis,
		Networks:     networks,
	})
	n.stats.ProviderGroups++
	return int32(len(n.data.Providers) - 1)
}

// addReference records the provider groups behind a provider_group_id.
func (n *normalizer) addReference(ref tic.ProviderReference) {
	if len(ref.ProviderGroups) == 0 {
		if ref.Location != "" {
			n.stats.RemoteRefs++
		}
		return
	}
	id := int64(ref.ProviderGroupID)
	for _, g := range ref.ProviderGroups {
		n.refs[id] = append(n.refs[id], n.addProvider(g, id, ref.NetworkName))
	}
}

// addItem flattens one in_network item into rows, one per negotiated price.
func (n *normalizer) addItem(item tic.InNetworkItem) {
	code := store.NormalizeCode(string(item.BillingCode))
	if code == "" {
		n.stats.Skipped++
		return
	}
	n.stats.Items++
	codeType := strings.ToUpper(strings.TrimSpace(item.BillingCodeType))

	for _, rate := range item.NegotiatedRates {
		n.stats.Rates++
		pr := pendingRate{firstRow: len(n.data.Rows)}
		for _, g := range rate.ProviderGroups {
			pr.inline = append(pr.inline, n.addProvider(g, 0, nil))
		}
		for _, ref := range rate.ProviderReferences {
			pr.refs = append(pr.refs, int64(ref))
		}
		for _, p := range rate.NegotiatedPrices {
			n.stats.Prices++
			n.data.Rows = append(n.data.Rows, store.RateRow{
				BillingCodeType:        codeType,
				BillingCodeTypeVersion: string(item.BillingCodeTypeVersion),
				BillingCode:            code,
				Name:                   strings.TrimSpace(item.Name),
				Description:            strings.TrimSpace(item.Description),
				NegotiationArrangement: item.NegotiationArrangement,
				NegotiatedType:         p.NegotiatedType,
				Rate:                   float64(p.NegotiatedRate),
				ExpirationDate:         p.ExpirationDate,
				BillingClass:           strings.ToLower(p.BillingClass),
				Setting:                strings.ToLower(p.Setting),
				ServiceCodes:           p.ServiceCode,
				Modifiers:              p.BillingCodeModifier,
				AdditionalInfo:         strings.TrimSpace(p.AdditionalInformation),
			})
		}
		pr.endRow = len(n.data.Rows)
		if pr.endRow > pr.firstRow {
			n.pending = append(n.pending, pr)
		}
	}
}

// resolve attaches provider indexes to rows. All rows of one negotiated_rate
// share one slice.
func (n *normalizer) resolve() {
	for _, pr := range n.pending {
		ids := append([]int32(nil), pr.inline...)
		for _, ref := range pr.refs {
			got, ok := n.refs[ref]
			if !ok {
				n.stats.DanglingRefs++
				continue
			}
			ids = append(ids, got...)
		}
		if len(ids) == 0 {
			n.stats.NoProviderRows += pr.endRow - pr.firstRow
			continue
		}
		for i := pr.firstRow; i < pr.endRow; i++ {
			n.data.Rows[i].Providers = ids
		}
	}
	n.pending = nil
	n.refs = nil
}
