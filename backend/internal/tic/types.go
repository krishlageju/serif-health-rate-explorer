// Package tic holds the parts of the CMS Transparency in Coverage schemas this app reads.
// See https://github.com/CMSgov/price-transparency-guide.
package tic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Index is a table-of-contents file.
type Index struct {
	ReportingEntityName string               `json:"reporting_entity_name"`
	ReportingEntityType string               `json:"reporting_entity_type"`
	LastUpdatedOn       string               `json:"last_updated_on"`
	Version             string               `json:"version"`
	ReportingStructure  []ReportingStructure `json:"reporting_structure"`
}

// ReportingStructure groups plans with the in-network files that apply to them.
type ReportingStructure struct {
	ReportingPlans []ReportingPlan `json:"reporting_plans"`
	InNetworkFiles []FileLocation  `json:"in_network_files"`
}

// ReportingPlan identifies one health plan.
type ReportingPlan struct {
	PlanName       string     `json:"plan_name"`
	PlanIDType     string     `json:"plan_id_type"`
	PlanID         FlexString `json:"plan_id"`
	PlanMarketType string     `json:"plan_market_type"`
	IssuerName     string     `json:"issuer_name,omitempty"`
}

// FileLocation points to a file listed in the index.
type FileLocation struct {
	Description string `json:"description"`
	Location    string `json:"location"`
}

// ProviderReference is one entry of the top-level provider_references array
// of an in-network file. Rates point at it by ProviderGroupID.
type ProviderReference struct {
	ProviderGroupID FlexInt         `json:"provider_group_id"`
	NetworkName     []string        `json:"network_name"`
	ProviderGroups  []ProviderGroup `json:"provider_groups"`
	Location        string          `json:"location"`
}

// ProviderGroup is one TIN and the NPIs billing under it.
type ProviderGroup struct {
	NPI []FlexInt `json:"npi"`
	TIN TIN       `json:"tin"`
}

// TIN is a tax identifier (usually an EIN) and its business name.
type TIN struct {
	Type         string     `json:"type"`
	Value        FlexString `json:"value"`
	BusinessName string     `json:"business_name"`
}

// InNetworkItem is one element of the in_network array: a billing code and
// all negotiated rates for it.
type InNetworkItem struct {
	NegotiationArrangement string           `json:"negotiation_arrangement"`
	Name                   string           `json:"name"`
	BillingCodeType        string           `json:"billing_code_type"`
	BillingCodeTypeVersion FlexString       `json:"billing_code_type_version"`
	BillingCode            FlexString       `json:"billing_code"`
	Description            string           `json:"description"`
	NegotiatedRates        []NegotiatedRate `json:"negotiated_rates"`
}

// NegotiatedRate links prices to providers, either by reference id or inline.
type NegotiatedRate struct {
	ProviderReferences []FlexInt         `json:"provider_references"`
	ProviderGroups     []ProviderGroup   `json:"provider_groups"`
	NegotiatedPrices   []NegotiatedPrice `json:"negotiated_prices"`
}

// NegotiatedPrice is one price with its billing class, setting and place of service.
type NegotiatedPrice struct {
	NegotiatedType        string    `json:"negotiated_type"`
	NegotiatedRate        FlexFloat `json:"negotiated_rate"`
	ExpirationDate        string    `json:"expiration_date"`
	ServiceCode           []string  `json:"service_code"`
	BillingClass          string    `json:"billing_class"`
	Setting               string    `json:"setting"`
	BillingCodeModifier   []string  `json:"billing_code_modifier"`
	AdditionalInformation string    `json:"additional_information"`
}

// FlexString accepts a JSON string or number. Publishers are inconsistent
// about whether codes and ids are quoted.
type FlexString string

// UnmarshalJSON accepts a string, a number or null.
func (s *FlexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*s = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = FlexString(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("expected string or number, got %s", b)
	}
	*s = FlexString(n.String())
	return nil
}

// FlexInt accepts a JSON integer or a numeric string.
type FlexInt int64

// UnmarshalJSON accepts an integer, a numeric string or null.
func (i *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*i = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return fmt.Errorf("invalid integer %s", b)
		}
		n = int64(f)
	}
	*i = FlexInt(n)
	return nil
}

// FlexFloat accepts a JSON number or a numeric string.
type FlexFloat float64

// UnmarshalJSON accepts a number, a numeric string or null.
func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid number %s", b)
	}
	*f = FlexFloat(v)
	return nil
}
