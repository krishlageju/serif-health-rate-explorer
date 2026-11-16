package store

import (
	"cmp"
	"fmt"
	"sort"
	"strings"
)

// Sort is the column results are ordered by. Each matches a column in the UI.
type Sort string

const (
	SortRate     Sort = "rate"     // negotiated rate
	SortCode     Sort = "code"     // billing code, then name
	SortBilling  Sort = "billing"  // billing class, then place of service
	SortProvider Sort = "provider" // first provider's business name
)

// ValidSort reports whether s names a sortable column.
func ValidSort(s string) bool {
	switch Sort(s) {
	case SortRate, SortCode, SortBilling, SortProvider:
		return true
	}
	return false
}

// Query is a normalized search request. Code is required; zero values mean
// "no filter" for everything else.
type Query struct {
	Code           string
	CodeType       string // CPT, HCPCS, RC, ...
	NPI            int64
	TIN            string // digits only
	BillingClass   string // professional | institutional
	IncludeRelated bool   // also match additional_information
	Sort           Sort
	Desc           bool
	Limit          int
	Offset         int
}

// HasProviderFilter reports whether the query filters by NPI or EIN.
func (q Query) HasProviderFilter() bool { return q.NPI != 0 || q.TIN != "" }

type MatchKind string

const (
	MatchBillingCode    MatchKind = "billing_code"
	MatchAdditionalInfo MatchKind = "additional_information"
)

// Hit is one matching row and how it matched.
type Hit struct {
	RowID int32
	Match MatchKind
}

// Summary is stats over every match, not just the returned page.
type Summary struct {
	Count       int     `json:"count"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Median      float64 `json:"median"`
	Mean        float64 `json:"mean"`
	Providers   int     `json:"providers"` // distinct TINs
	DirectRows  int     `json:"direct_rows"`
	RelatedRows int     `json:"related_rows"`
}

// Hint explains an empty result so the UI can say something useful.
type Hint struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is a page of hits plus the total, summary and empty-result hint.
type Result struct {
	Total   int
	Summary *Summary
	Hits    []Hit
	Hint    *Hint
}

// Search finds rates for q.Code, applies filters, then sorts and pages.
// The summary covers every match, not just the returned page.
func (s *Store) Search(q Query) Result {
	var cands []Hit
	for _, id := range s.byCode[q.Code] {
		cands = append(cands, Hit{id, MatchBillingCode})
	}
	if q.IncludeRelated {
		for _, id := range s.byAddl[q.Code] {
			cands = append(cands, Hit{id, MatchAdditionalInfo})
		}
	}
	if len(cands) == 0 {
		msg := fmt.Sprintf("No negotiated rates for code %s in the loaded files.", q.Code)
		if !q.IncludeRelated && len(s.byAddl[q.Code]) > 0 {
			msg = fmt.Sprintf("Code %s is not a billing code here, but %d facility rates reference it. Turn on related rates to see them.", q.Code, len(s.byAddl[q.Code]))
		}
		return Result{Hint: &Hint{"code_not_found", msg}}
	}

	var allowed map[int32]struct{}
	if q.HasProviderFilter() {
		var hint *Hint
		allowed, hint = s.allowedProviders(q)
		if hint != nil {
			return Result{Hint: hint}
		}
	}

	hits := make([]Hit, 0, len(cands))
	for _, h := range cands {
		r := &s.rows[h.RowID]
		if q.CodeType != "" && r.BillingCodeType != q.CodeType {
			continue
		}
		if q.BillingClass != "" && !strings.EqualFold(r.BillingClass, q.BillingClass) {
			continue
		}
		if allowed != nil && !anyAllowed(r.Providers, allowed) {
			continue
		}
		hits = append(hits, h)
	}

	if len(hits) == 0 {
		if allowed != nil {
			return Result{Hint: &Hint{"no_rates_for_provider", fmt.Sprintf(
				"The provider is in network, but has no negotiated rate for code %s with the selected filters.", q.Code)}}
		}
		return Result{Hint: &Hint{"filtered_out", fmt.Sprintf(
			"Code %s has %d rates, but none match the selected filters.", q.Code, len(cands))}}
	}

	sum := s.summarize(hits)
	s.sortHits(hits, q.Sort, q.Desc)

	total := len(hits)
	start := min(q.Offset, total)
	end := total
	if q.Limit > 0 {
		end = min(start+q.Limit, total)
	}
	return Result{Total: total, Summary: sum, Hits: hits[start:end]}
}

// allowedProviders returns provider ids that match the NPI and/or TIN filter.
// With both set, the NPI and TIN must belong to the same provider group.
func (s *Store) allowedProviders(q Query) (map[int32]struct{}, *Hint) {
	var npiSet, tinSet []int32
	if q.NPI != 0 {
		npiSet = s.byNPI[q.NPI]
		if len(npiSet) == 0 {
			return nil, &Hint{"npi_not_found", fmt.Sprintf("NPI %d is not listed for any provider in the loaded files.", q.NPI)}
		}
	}
	if q.TIN != "" {
		tinSet = s.byTIN[q.TIN]
		if len(tinSet) == 0 {
			return nil, &Hint{"ein_not_found", fmt.Sprintf("EIN %s is not listed for any provider in the loaded files.", FormatEIN(q.TIN))}
		}
	}

	allowed := map[int32]struct{}{}
	switch {
	case npiSet != nil && tinSet != nil:
		inTIN := map[int32]struct{}{}
		for _, id := range tinSet {
			inTIN[id] = struct{}{}
		}
		for _, id := range npiSet {
			if _, ok := inTIN[id]; ok {
				allowed[id] = struct{}{}
			}
		}
		if len(allowed) == 0 {
			return nil, &Hint{"npi_ein_mismatch", fmt.Sprintf(
				"NPI %d and EIN %s are both in the file, but never in the same provider group.", q.NPI, FormatEIN(q.TIN))}
		}
	case npiSet != nil:
		for _, id := range npiSet {
			allowed[id] = struct{}{}
		}
	default:
		for _, id := range tinSet {
			allowed[id] = struct{}{}
		}
	}
	return allowed, nil
}

// ProviderMatches reports whether a provider satisfies the query's NPI/TIN filter.
func (s *Store) ProviderMatches(id int32, q Query) bool {
	if !q.HasProviderFilter() {
		return false
	}
	p := &s.providers[id]
	if q.TIN != "" && p.TINDigits != q.TIN {
		return false
	}
	if q.NPI != 0 {
		for _, n := range p.NPIs {
			if n == q.NPI {
				return true
			}
		}
		return false
	}
	return true
}

// anyAllowed reports whether any of the provider ids is in the allowed set.
func anyAllowed(ids []int32, allowed map[int32]struct{}) bool {
	for _, id := range ids {
		if _, ok := allowed[id]; ok {
			return true
		}
	}
	return false
}

// summarize computes count, min, max, median, mean and distinct EINs over all hits.
func (s *Store) summarize(hits []Hit) *Summary {
	rates := make([]float64, len(hits))
	tins := map[string]struct{}{}
	sum := &Summary{Count: len(hits)}
	total := 0.0
	for i, h := range hits {
		r := &s.rows[h.RowID]
		rates[i] = r.Rate
		total += r.Rate
		if h.Match == MatchBillingCode {
			sum.DirectRows++
		} else {
			sum.RelatedRows++
		}
		for _, p := range r.Providers {
			key := s.providers[p].TINDigits
			if key == "" {
				key = fmt.Sprintf("p%d", p)
			}
			tins[key] = struct{}{}
		}
	}
	sort.Float64s(rates)
	sum.Min = rates[0]
	sum.Max = rates[len(rates)-1]
	sum.Mean = total / float64(len(rates))
	if n := len(rates); n%2 == 1 {
		sum.Median = rates[n/2]
	} else {
		sum.Median = (rates[n/2-1] + rates[n/2]) / 2
	}
	sum.Providers = len(tins)
	return sum
}

// providerName returns the lower-cased name of a row's first provider, for sorting.
func (s *Store) providerName(r *RateRow) string {
	if len(r.Providers) == 0 {
		return ""
	}
	return strings.ToLower(s.providers[r.Providers[0]].BusinessName)
}

// sortHits orders hits by one column. desc flips only that column; ties are
// always broken by rate (low to high), then by row id, so paging is stable.
func (s *Store) sortHits(hits []Hit, by Sort, desc bool) {
	primary := func(a, b *RateRow) int { return cmp.Compare(a.Rate, b.Rate) }
	switch by {
	case SortCode:
		primary = func(a, b *RateRow) int {
			return cmp.Or(cmp.Compare(a.BillingCode, b.BillingCode), cmp.Compare(a.Name, b.Name))
		}
	case SortBilling:
		primary = func(a, b *RateRow) int {
			return cmp.Or(
				cmp.Compare(a.BillingClass, b.BillingClass),
				cmp.Compare(strings.Join(a.ServiceCodes, ","), strings.Join(b.ServiceCodes, ",")),
				cmp.Compare(strings.Join(a.Modifiers, ","), strings.Join(b.Modifiers, ",")),
			)
		}
	case SortProvider:
		primary = func(a, b *RateRow) int { return cmp.Compare(s.providerName(a), s.providerName(b)) }
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := &s.rows[hits[i].RowID], &s.rows[hits[j].RowID]
		c := primary(a, b)
		if desc {
			c = -c
		}
		return cmp.Or(c, cmp.Compare(a.Rate, b.Rate), cmp.Compare(hits[i].RowID, hits[j].RowID)) < 0
	})
}

// Suggest returns codes for autocomplete: exact code first, then code
// prefix, then matches in the name or description. limit <= 0 returns all.
func (s *Store) Suggest(q string, limit int) []CodeSummary {
	q = NormalizeCode(q)
	if q == "" {
		return []CodeSummary{}
	}
	type scored struct {
		c     CodeSummary
		score int
	}
	var out []scored
	for _, c := range s.codes {
		switch {
		case c.Code == q:
			out = append(out, scored{c, 0})
		case strings.HasPrefix(c.Code, q):
			out = append(out, scored{c, 1})
		case len(q) >= 3 && (strings.Contains(strings.ToUpper(c.Name), q) || strings.Contains(strings.ToUpper(c.Description), q)):
			out = append(out, scored{c, 2})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score < out[j].score
		}
		return out[i].c.Rows+out[i].c.RelatedRows > out[j].c.Rows+out[j].c.RelatedRows
	})
	if limit <= 0 || limit > len(out) {
		limit = len(out)
	}
	res := make([]CodeSummary, 0, limit)
	for i := 0; i < limit; i++ {
		res = append(res, out[i].c)
	}
	return res
}

// FormatEIN formats 9 digits as NN-NNNNNNN; other input is returned as is.
func FormatEIN(d string) string {
	if len(d) == 9 {
		return d[:2] + "-" + d[2:]
	}
	return d
}
