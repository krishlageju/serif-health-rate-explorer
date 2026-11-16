package store_test

import (
	"context"
	"os"
	"testing"

	"ptexplorer/internal/ingest"
	"ptexplorer/internal/store"
)

// buildFixture loads the test fixture into a store.
func buildFixture(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.Open("../../testdata/in-network.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, _, _, err := ingest.Load(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	b := store.NewBuilder()
	b.AddFile(store.Source{URL: "fixture", FileName: "in-network.json"}, data)
	return b.Build()
}

// TestDedupe checks that identical prices under different names are merged.
func TestDedupe(t *testing.T) {
	s := buildFixture(t)
	st := s.Stats()
	if st.RawRows != 6 || st.Rows != 5 {
		t.Fatalf("raw=%d rows=%d, want 6 and 5", st.RawRows, st.Rows)
	}
	res := s.Search(store.Query{Code: "490", IncludeRelated: true})
	if res.Total != 1 {
		t.Fatalf("490 total = %d, want 1 merged row", res.Total)
	}
	r := s.Row(res.Hits[0].RowID)
	if r.Listings != 2 || len(r.OtherNames) != 1 || r.OtherNames[0] != "COLONOSCOPY" {
		t.Errorf("merged row listings=%d other=%v", r.Listings, r.OtherNames)
	}
}

// TestSearch checks matching, provider filters and the hint for each empty case.
func TestSearch(t *testing.T) {
	s := buildFixture(t)
	tests := []struct {
		name      string
		q         store.Query
		wantTotal int
		wantHint  string
	}{
		{"direct and related", store.Query{Code: "99213", IncludeRelated: true}, 4, ""},
		{"direct only", store.Query{Code: "99213"}, 3, ""},
		{"related only code", store.Query{Code: "G0121", IncludeRelated: true}, 1, ""},
		{"unknown code", store.Query{Code: "00000", IncludeRelated: true}, 0, "code_not_found"},
		{"npi in two groups", store.Query{Code: "99213", IncludeRelated: true, NPI: 1234567890}, 3, ""},
		{"ein", store.Query{Code: "99213", IncludeRelated: true, TIN: "222222222"}, 2, ""},
		{"npi and ein same group", store.Query{Code: "99213", IncludeRelated: true, NPI: 1234567890, TIN: "444444444"}, 2, ""},
		{"npi and ein different groups", store.Query{Code: "99213", IncludeRelated: true, NPI: 1234567890, TIN: "222222222"}, 0, "npi_ein_mismatch"},
		{"unknown npi", store.Query{Code: "99213", NPI: 9999999999}, 0, "npi_not_found"},
		{"unknown ein", store.Query{Code: "99213", TIN: "999999999"}, 0, "ein_not_found"},
		{"provider without this code", store.Query{Code: "490", IncludeRelated: true, NPI: 1111111111}, 0, "no_rates_for_provider"},
		{"filters exclude all", store.Query{Code: "99213", BillingClass: "institutional"}, 0, "filtered_out"},
		{"code type filter", store.Query{Code: "99213", IncludeRelated: true, CodeType: "RC"}, 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := s.Search(tt.q)
			if res.Total != tt.wantTotal {
				t.Errorf("total = %d, want %d", res.Total, tt.wantTotal)
			}
			got := ""
			if res.Hint != nil {
				got = res.Hint.Code
			}
			if got != tt.wantHint {
				t.Errorf("hint = %q, want %q", got, tt.wantHint)
			}
		})
	}
}

// TestSummaryAndPaging checks that stats cover all hits while paging slices them.
func TestSummaryAndPaging(t *testing.T) {
	s := buildFixture(t)
	res := s.Search(store.Query{Code: "99213", IncludeRelated: true, Sort: store.SortRate, Desc: true, Limit: 2, Offset: 1})
	if res.Total != 4 || len(res.Hits) != 2 {
		t.Fatalf("total=%d page=%d", res.Total, len(res.Hits))
	}
	sum := res.Summary
	if sum.Min != 90 || sum.Max != 500 || sum.Median != 125.25 || sum.DirectRows != 3 || sum.RelatedRows != 1 {
		t.Errorf("summary = %+v", sum)
	}
	if got := s.Row(res.Hits[0].RowID).Rate; got != 150 {
		t.Errorf("second highest rate = %v, want 150", got)
	}
	if res.Hits[1].Match != store.MatchBillingCode {
		t.Errorf("match = %v", res.Hits[1].Match)
	}

	past := s.Search(store.Query{Code: "99213", IncludeRelated: true, Limit: 10, Offset: 50})
	if past.Total != 4 || len(past.Hits) != 0 {
		t.Errorf("offset past end: total=%d hits=%d", past.Total, len(past.Hits))
	}
}

// TestSuggest checks autocomplete ordering, name search and the no-limit option.
func TestSuggest(t *testing.T) {
	s := buildFixture(t)
	got := s.Suggest("9921", 5)
	if len(got) == 0 || got[0].Code != "99213" || got[0].RelatedRows != 1 {
		t.Fatalf("suggest = %+v", got)
	}
	if got := s.Suggest("office", 5); len(got) != 1 || got[0].Code != "99213" {
		t.Errorf("name search = %+v", got)
	}
	if all, capped := s.Suggest("al c", 0), s.Suggest("al c", 1); len(all) < 2 || len(capped) != 1 {
		t.Errorf("limit 0 should return all matches: all=%d capped=%d", len(all), len(capped))
	}
	if got := s.Suggest("  ", 5); len(got) != 0 {
		t.Errorf("blank query should return nothing")
	}
}

// TestSortColumns checks each sortable column in both directions.
func TestSortColumns(t *testing.T) {
	s := buildFixture(t)
	first := func(by store.Sort, desc bool) *store.RateRow {
		res := s.Search(store.Query{Code: "99213", IncludeRelated: true, Sort: by, Desc: desc})
		return s.Row(res.Hits[0].RowID)
	}
	if r := first(store.SortRate, false); r.Rate != 90 {
		t.Errorf("rate asc first = %v", r.Rate)
	}
	if r := first(store.SortRate, true); r.Rate != 500 {
		t.Errorf("rate desc first = %v", r.Rate)
	}
	// "490" sorts before "99213" as a string.
	if r := first(store.SortCode, false); r.BillingCode != "490" {
		t.Errorf("code asc first = %v", r.BillingCode)
	}
	if r := first(store.SortCode, true); r.BillingCode != "99213" {
		t.Errorf("code desc first = %v", r.BillingCode)
	}
	if r := first(store.SortBilling, false); r.BillingClass != "institutional" {
		t.Errorf("billing asc first = %v", r.BillingClass)
	}
	if r := first(store.SortBilling, true); r.BillingClass != "professional" || r.Rate != 150 {
		t.Errorf("billing desc first = %v %v (modifier 25 sorts after no modifier)", r.BillingClass, r.Rate)
	}
	if r := first(store.SortProvider, false); s.Provider(r.Providers[0]).BusinessName != "Alpha Medical" {
		t.Errorf("provider asc first = %v", s.Provider(r.Providers[0]).BusinessName)
	}
	if r := first(store.SortProvider, true); s.Provider(r.Providers[0]).BusinessName != "Inline Clinic" {
		t.Errorf("provider desc first = %v", s.Provider(r.Providers[0]).BusinessName)
	}
}
