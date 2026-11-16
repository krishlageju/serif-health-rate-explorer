// Package store keeps normalized rate data in memory with lookup indexes
// for searching by billing code, NPI and TIN/EIN.
package store

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"ptexplorer/internal/tic"
)

// Source is one loaded in-network file and the plans that use it.
type Source struct {
	ID                  int                 `json:"id"`
	URL                 string              `json:"url"`
	FileName            string              `json:"file_name"`
	Description         string              `json:"description"`
	Plans               []tic.ReportingPlan `json:"plans"`
	ReportingEntityName string              `json:"reporting_entity_name"`
	LastUpdatedOn       string              `json:"last_updated_on"`
}

// Provider is one provider_groups entry: a single TIN with its NPIs.
type Provider struct {
	SourceID     int
	GroupID      int64 // provider_group_id in the source file, 0 for inline groups
	TINType      string
	TIN          string // as published
	TINDigits    string // digits only, used for lookups
	BusinessName string
	NPIs         []int64
	Networks     []string
}

// RateRow is one negotiated price, flattened from
// in_network -> negotiated_rates -> negotiated_prices.
type RateRow struct {
	SourceID               int
	BillingCodeType        string
	BillingCodeTypeVersion string
	BillingCode            string
	Name                   string
	Description            string
	NegotiationArrangement string
	NegotiatedType         string
	Rate                   float64
	ExpirationDate         string
	BillingClass           string
	Setting                string
	ServiceCodes           []string
	Modifiers              []string
	AdditionalInfo         string
	// Providers holds indexes into the store's provider list. Rows from the
	// same negotiated_rate share this slice; treat it as read-only.
	Providers []int32
	// OtherNames lists the other in_network item names this exact price was
	// published under. Set when duplicates are merged at build time.
	OtherNames []string
	// Listings is how many raw price entries were merged into this row.
	Listings int
}

// CodeSummary describes one searchable code, used for autocomplete.
type CodeSummary struct {
	Code        string `json:"code"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rows        int    `json:"rows"`         // rates where this is the billing code
	Variants    int    `json:"variants"`     // distinct names under this code
	RelatedRows int    `json:"related_rows"` // rates that mention it in additional_information
}

// Stats are totals for the whole store.
type Stats struct {
	RawRows   int `json:"raw_rows"` // price entries read from the files
	Rows      int `json:"rows"`     // unique rates after merging duplicates
	Providers int `json:"providers"`
	NPIs      int `json:"npis"`
	TINs      int `json:"tins"`
	Codes     int `json:"codes"`
}

// Store is the immutable, indexed rate data that searches run against.
type Store struct {
	sources   []Source
	rows      []RateRow
	providers []Provider

	byCode map[string][]int32 // billing code -> rows
	byAddl map[string][]int32 // code in additional_information -> rows
	byNPI  map[int64][]int32  // NPI -> providers
	byTIN  map[string][]int32 // TIN digits -> providers

	codes     []CodeSummary
	codeNames map[string]string // CPT/HCPCS code -> item name
	stats     Stats
}

// Sources returns the loaded files.
func (s *Store) Sources() []Source { return s.sources }

// Row returns the rate row with the given id.
func (s *Store) Row(id int32) *RateRow { return &s.rows[id] }

// Provider returns the provider group with the given id.
func (s *Store) Provider(id int32) *Provider { return &s.providers[id] }

// Stats returns totals for the whole store.
func (s *Store) Stats() Stats { return s.stats }

// CodeName returns the published name of a CPT/HCPCS code, if one of the
// loaded files lists it as a billing code.
func (s *Store) CodeName(code string) string { return s.codeNames[NormalizeCode(code)] }

// Source returns the source with the given id, or nil if it is out of range.
func (s *Store) Source(id int) *Source {
	if id < 0 || id >= len(s.sources) {
		return nil
	}
	return &s.sources[id]
}

// FileData is the normalized content of one in-network file. Provider
// indexes in Rows refer to positions in Providers.
type FileData struct {
	Meta      map[string]string
	Providers []Provider
	Rows      []RateRow
}

// Builder collects files and builds an immutable Store.
type Builder struct {
	sources   []Source
	rows      []RateRow
	providers []Provider
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder { return &Builder{} }

// AddFile adds one file's data and returns its source id.
func (b *Builder) AddFile(src Source, data *FileData) int {
	id := len(b.sources)
	src.ID = id
	if src.LastUpdatedOn == "" {
		src.LastUpdatedOn = data.Meta["last_updated_on"]
	}
	if src.ReportingEntityName == "" {
		src.ReportingEntityName = data.Meta["reporting_entity_name"]
	}
	b.sources = append(b.sources, src)

	offset := int32(len(b.providers))
	for _, p := range data.Providers {
		p.SourceID = id
		b.providers = append(b.providers, p)
	}

	// Rows from one negotiated_rate share a provider slice. Remap each
	// distinct slice once so they keep sharing memory after the merge.
	remapped := map[*int32][]int32{}
	for _, r := range data.Rows {
		r.SourceID = id
		if len(r.Providers) > 0 {
			key := &r.Providers[0]
			ids, ok := remapped[key]
			if !ok {
				ids = make([]int32, len(r.Providers))
				for i, p := range r.Providers {
					ids[i] = p + offset
				}
				remapped[key] = ids
			}
			r.Providers = ids
		}
		b.rows = append(b.rows, r)
	}
	return id
}

// codeLike matches values in additional_information that look like a
// billing code (e.g. "45380"), as opposed to free text.
var codeLike = regexp.MustCompile(`^[A-Z0-9]{3,7}$`)

// Build merges duplicate rows and builds the lookup indexes and autocomplete list.
func (b *Builder) Build() *Store {
	s := &Store{
		sources:   b.sources,
		rows:      dedupe(b.rows),
		providers: b.providers,
		byCode:    map[string][]int32{},
		byAddl:    map[string][]int32{},
		byNPI:     map[int64][]int32{},
		byTIN:     map[string][]int32{},
	}

	type codeKey struct{ code, typ string }
	codeAgg := map[codeKey]*CodeSummary{}
	names := map[codeKey]map[string]struct{}{}
	related := map[string]int{}

	for i := range s.rows {
		r := &s.rows[i]
		id := int32(i)
		s.byCode[r.BillingCode] = append(s.byCode[r.BillingCode], id)

		k := codeKey{r.BillingCode, r.BillingCodeType}
		cs, ok := codeAgg[k]
		if !ok {
			cs = &CodeSummary{Code: r.BillingCode, Type: r.BillingCodeType, Name: r.Name, Description: r.Description}
			codeAgg[k] = cs
			names[k] = map[string]struct{}{}
		}
		cs.Rows++
		names[k][r.Name] = struct{}{}
		for _, n := range r.OtherNames {
			names[k][n] = struct{}{}
		}

		if addl := NormalizeCode(r.AdditionalInfo); addl != "" && addl != r.BillingCode && codeLike.MatchString(addl) {
			s.byAddl[addl] = append(s.byAddl[addl], id)
			related[addl]++
		}
	}

	tins := map[string]struct{}{}
	for i := range s.providers {
		p := &s.providers[i]
		id := int32(i)
		for _, npi := range p.NPIs {
			s.byNPI[npi] = append(s.byNPI[npi], id)
		}
		if p.TINDigits != "" {
			s.byTIN[p.TINDigits] = append(s.byTIN[p.TINDigits], id)
			tins[p.TINDigits] = struct{}{}
		}
	}

	// Build the autocomplete list. Codes only seen in additional_information
	// get their own entry so they are still discoverable.
	direct := map[string]bool{}
	s.codeNames = map[string]string{}
	for k, cs := range codeAgg {
		if k.typ != "RC" && cs.Name != "" {
			s.codeNames[k.code] = cs.Name
		}
		cs.Variants = len(names[k])
		cs.RelatedRows = related[k.code]
		direct[k.code] = true
		s.codes = append(s.codes, *cs)
	}
	for code, n := range related {
		if !direct[code] {
			s.codes = append(s.codes, CodeSummary{Code: code, RelatedRows: n,
				Description: "Referenced by facility revenue-code rates"})
		}
	}
	sort.Slice(s.codes, func(i, j int) bool {
		if s.codes[i].Code != s.codes[j].Code {
			return s.codes[i].Code < s.codes[j].Code
		}
		return s.codes[i].Type < s.codes[j].Type
	})

	s.stats = Stats{
		RawRows:   len(b.rows),
		Rows:      len(s.rows),
		Providers: len(s.providers),
		NPIs:      len(s.byNPI),
		TINs:      len(tins),
		Codes:     len(s.codes),
	}
	return s
}

// dedupe merges prices identical in everything but the item name. Some files repeat one
// price under many unrelated names (Fidelis: 180k entries → 7.7k distinct rates).
func dedupe(rows []RateRow) []RateRow {
	out := make([]RateRow, 0, len(rows)/4)
	seen := map[string]int{}
	var kb strings.Builder
	for _, r := range rows {
		kb.Reset()
		fmt.Fprintf(&kb, "%d|%s|%s|%s|%g|%s|%s|%s|%s|%s|%s|%s|%s|", r.SourceID, r.BillingCodeType, r.BillingCode,
			r.BillingCodeTypeVersion, r.Rate, r.NegotiatedType, r.NegotiationArrangement, r.ExpirationDate,
			r.BillingClass, r.Setting, r.AdditionalInfo, strings.Join(r.ServiceCodes, ","), strings.Join(r.Modifiers, ","))
		for _, p := range r.Providers {
			kb.WriteString(strconv.Itoa(int(p)))
			kb.WriteByte(',')
		}
		key := kb.String()
		if i, ok := seen[key]; ok {
			m := &out[i]
			m.Listings++
			if r.Name != m.Name && !slices.Contains(m.OtherNames, r.Name) {
				m.OtherNames = append(m.OtherNames, r.Name)
			}
			continue
		}
		r.Listings = 1
		r.OtherNames = nil
		seen[key] = len(out)
		out = append(out, r)
	}
	return slices.Clip(out)
}

// NormalizeCode upper-cases and trims a billing code.
func NormalizeCode(c string) string {
	return strings.ToUpper(strings.TrimSpace(c))
}

// DigitsOnly strips everything but 0-9, used for EIN/TIN comparison.
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
