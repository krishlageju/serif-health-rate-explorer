// Package search is the read side: it answers rate searches against the current store.
// The HTTP layer talks to it through an interface, so it could later run as its own service.
package search

import (
	"context"
	"errors"

	"ptexplorer/internal/store"
)

// ErrNotReady means no rate data has been published yet (still loading, or loading failed).
var ErrNotReady = errors.New("rate data is not loaded yet")

const (
	maxNPIsReturned  = 25 // NPIs listed per provider group; the full count is still returned
	maxNamesReturned = 30 // merged item names listed per rate; the full count is still returned
)

// StoreReader gives access to the latest published store. *store.Holder implements it.
type StoreReader interface {
	Current() *store.Store
}

// Service runs searches in-process. It holds no state of its own beyond the store reader.
type Service struct {
	stores StoreReader
}

// NewService creates a search service that reads from the given store holder.
func NewService(stores StoreReader) *Service {
	return &Service{stores: stores}
}

// Search runs q and returns one page of results shaped for the API.
// ctx is unused in-process but lets a remote implementation honor cancellation.
func (s *Service) Search(_ context.Context, q store.Query) (*Page, error) {
	st := s.stores.Current()
	if st == nil {
		return nil, ErrNotReady
	}
	res := st.Search(q)
	page := &Page{Total: res.Total, Summary: res.Summary, Hint: res.Hint, Results: make([]Rate, 0, len(res.Hits))}
	for _, h := range res.Hits {
		page.Results = append(page.Results, toRate(st, h, q))
	}
	return page, nil
}

// Suggest returns codes matching a prefix or name fragment; limit <= 0 returns all.
func (s *Service) Suggest(_ context.Context, q string, limit int) ([]store.CodeSummary, error) {
	st := s.stores.Current()
	if st == nil {
		return nil, ErrNotReady
	}
	return st.Suggest(q, limit), nil
}

// LoadedSources lists the files that made it into the published store.
func (s *Service) LoadedSources(_ context.Context) ([]store.Source, error) {
	st := s.stores.Current()
	if st == nil {
		return nil, ErrNotReady
	}
	return st.Sources(), nil
}

// toRate converts a hit to its API shape, listing providers that match the filter first.
func toRate(st *store.Store, h store.Hit, q store.Query) Rate {
	r := st.Row(h.RowID)
	out := Rate{
		ID:                     h.RowID,
		Match:                  string(h.Match),
		BillingCode:            r.BillingCode,
		BillingCodeType:        r.BillingCodeType,
		BillingCodeTypeVersion: r.BillingCodeTypeVersion,
		Name:                   r.Name,
		Description:            r.Description,
		NegotiationArrangement: r.NegotiationArrangement,
		NegotiatedType:         r.NegotiatedType,
		Rate:                   r.Rate,
		ExpirationDate:         r.ExpirationDate,
		BillingClass:           r.BillingClass,
		Setting:                r.Setting,
		ServiceCodes:           nonNil(r.ServiceCodes),
		Modifiers:              nonNil(r.Modifiers),
		AdditionalInformation:  r.AdditionalInfo,
		AdditionalInfoName:     st.CodeName(r.AdditionalInfo),
		Listings:               r.Listings,
		OtherNames:             nonNil(r.OtherNames[:min(len(r.OtherNames), maxNamesReturned)]),
		OtherNameCount:         len(r.OtherNames),
		Providers:              make([]Provider, 0, len(r.Providers)),
	}

	var rest []Provider
	for _, pid := range r.Providers {
		p := st.Provider(pid)
		pv := Provider{
			ProviderGroupID: p.GroupID,
			TINType:         p.TINType,
			TIN:             p.TIN,
			BusinessName:    p.BusinessName,
			NPICount:        len(p.NPIs),
			NPIs:            p.NPIs[:min(len(p.NPIs), maxNPIsReturned)],
			Networks:        nonNil(p.Networks),
			Matched:         st.ProviderMatches(pid, q),
		}
		if pv.Matched {
			out.Providers = append(out.Providers, pv)
		} else {
			rest = append(rest, pv)
		}
	}
	out.Providers = append(out.Providers, rest...)

	if src := st.Source(r.SourceID); src != nil {
		out.Source = &SourceRef{ID: src.ID, FileName: src.FileName, URL: src.URL, PlanCount: len(src.Plans), LastUpdatedOn: src.LastUpdatedOn}
	}
	return out
}

// nonNil returns an empty slice instead of nil, so JSON encodes [] rather than null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
