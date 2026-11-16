package search

import "ptexplorer/internal/store"

// Page is one page of search results. It is the contract between the search service
// and its callers, so it is plain data that serializes cleanly to JSON.
type Page struct {
	Total   int            `json:"total"`
	Summary *store.Summary `json:"summary"` // covers every match, not just this page
	Results []Rate         `json:"results"`
	Hint    *store.Hint    `json:"hint,omitempty"` // why the result is empty, when it is
}

// Rate is one negotiated price with its code, billing details, providers and source.
type Rate struct {
	ID                     int32      `json:"id"`
	Match                  string     `json:"match"` // billing_code or additional_information
	BillingCode            string     `json:"billing_code"`
	BillingCodeType        string     `json:"billing_code_type"`
	BillingCodeTypeVersion string     `json:"billing_code_type_version"`
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	NegotiationArrangement string     `json:"negotiation_arrangement"`
	NegotiatedType         string     `json:"negotiated_type"`
	Rate                   float64    `json:"rate"`
	ExpirationDate         string     `json:"expiration_date"`
	BillingClass           string     `json:"billing_class"`
	Setting                string     `json:"setting"`
	ServiceCodes           []string   `json:"service_codes"`
	Modifiers              []string   `json:"modifiers"`
	AdditionalInformation  string     `json:"additional_information,omitempty"`
	AdditionalInfoName     string     `json:"additional_information_name,omitempty"`
	Listings               int        `json:"listings"`    // raw price entries merged into this rate
	OtherNames             []string   `json:"other_names"` // other item names it was published under
	OtherNameCount         int        `json:"other_name_count"`
	Providers              []Provider `json:"providers"`
	Source                 *SourceRef `json:"source"`
}

// Provider is one provider group (a TIN and its NPIs) attached to a rate.
type Provider struct {
	ProviderGroupID int64    `json:"provider_group_id,omitempty"`
	TINType         string   `json:"tin_type"`
	TIN             string   `json:"tin"`
	BusinessName    string   `json:"business_name"`
	NPICount        int      `json:"npi_count"`
	NPIs            []int64  `json:"npis"`
	Networks        []string `json:"networks"`
	Matched         bool     `json:"matched"` // satisfies the query's NPI/EIN filter
}

// SourceRef points back to the file a rate came from.
type SourceRef struct {
	ID            int    `json:"id"`
	FileName      string `json:"file_name"`
	URL           string `json:"url"`
	PlanCount     int    `json:"plan_count"`
	LastUpdatedOn string `json:"last_updated_on"`
}
