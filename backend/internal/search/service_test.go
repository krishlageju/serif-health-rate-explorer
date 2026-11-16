package search

import (
	"context"
	"errors"
	"os"
	"testing"

	"ptexplorer/internal/ingest"
	"ptexplorer/internal/store"
)

// TestNotReady checks every method reports ErrNotReady before a store is published.
func TestNotReady(t *testing.T) {
	s := NewService(&store.Holder{})
	if _, err := s.Search(context.Background(), store.Query{Code: "99213"}); !errors.Is(err, ErrNotReady) {
		t.Errorf("Search err = %v", err)
	}
	if _, err := s.Suggest(context.Background(), "9", 0); !errors.Is(err, ErrNotReady) {
		t.Errorf("Suggest err = %v", err)
	}
	if _, err := s.LoadedSources(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Errorf("LoadedSources err = %v", err)
	}
}

// TestSearchShapesResults checks results are mapped with matched providers listed first.
func TestSearchShapesResults(t *testing.T) {
	f, err := os.Open("../../testdata/in-network.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := ingest.Parser{}.Parse(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	b := store.NewBuilder()
	b.AddFile(store.Source{URL: "fixture", FileName: "in-network.json"}, res.Data)
	holder := &store.Holder{}
	holder.Publish(b.Build())

	page, err := NewService(holder).Search(context.Background(),
		store.Query{Code: "99213", IncludeRelated: true, NPI: 1234567890, TIN: "444444444", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Results) != 2 {
		t.Fatalf("total=%d results=%d", page.Total, len(page.Results))
	}
	first := page.Results[0]
	if !first.Providers[0].Matched || first.Providers[0].BusinessName != "Gamma Group" {
		t.Errorf("matched provider should come first: %+v", first.Providers)
	}
	if first.Source == nil || first.Source.FileName != "in-network.json" {
		t.Errorf("source = %+v", first.Source)
	}
}
