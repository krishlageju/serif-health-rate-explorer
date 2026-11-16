package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ptexplorer/internal/search"
	"ptexplorer/internal/store"
)

const (
	defaultLimit = 25
	maxLimit     = 200
	maxOffset    = 1 << 30
)

var codePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9.\-]{0,14}$`)

// queryEcho is the normalized query, echoed back so clients see how input was interpreted.
type queryEcho struct {
	Code           string `json:"code"`
	CodeType       string `json:"code_type,omitempty"`
	NPI            string `json:"npi,omitempty"`
	EIN            string `json:"ein,omitempty"`
	BillingClass   string `json:"billing_class,omitempty"`
	IncludeRelated bool   `json:"include_related"`
	Sort           string `json:"sort"`
	Order          string `json:"order"`
}

// searchResponse is the body of GET /api/search: the normalized query plus one page of results.
type searchResponse struct {
	Query  queryEcho `json:"query"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
	*search.Page
}

// search handles GET /api/search: validate params, ask the searcher, encode the page.
func (s *Server) search(c *gin.Context) {
	q, echo, ok := parseSearchQuery(c)
	if !ok {
		return
	}
	page, err := s.searcher.Search(c.Request.Context(), q)
	if err != nil {
		s.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, searchResponse{Query: echo, Limit: q.Limit, Offset: q.Offset, Page: page})
}

// parseSearchQuery validates query params. On failure it writes a 400.
func parseSearchQuery(c *gin.Context) (store.Query, queryEcho, bool) {
	var q store.Query
	bad := func(field, msg string) (store.Query, queryEcho, bool) {
		writeError(c, http.StatusBadRequest, "invalid_request", msg, field)
		return q, queryEcho{}, false
	}

	q.Code = store.NormalizeCode(c.Query("code"))
	if q.Code == "" {
		return bad("code", "A billing code is required.")
	}
	if !codePattern.MatchString(q.Code) {
		return bad("code", "Billing code may only contain letters, digits, '.' and '-' (max 15 characters).")
	}

	if v := strings.TrimSpace(c.Query("npi")); v != "" {
		if len(v) != 10 || store.DigitsOnly(v) != v {
			return bad("npi", "NPI must be exactly 10 digits.")
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		q.NPI = n
	}
	if v := strings.TrimSpace(c.Query("ein")); v != "" {
		d := store.DigitsOnly(v)
		if len(d) != 9 || strings.Trim(v, "0123456789- ") != "" {
			return bad("ein", "EIN must be 9 digits, e.g. 12-3456789.")
		}
		q.TIN = d
	}

	q.CodeType = strings.ToUpper(strings.TrimSpace(c.Query("code_type")))
	q.BillingClass = strings.ToLower(strings.TrimSpace(c.Query("billing_class")))
	if q.BillingClass != "" && q.BillingClass != "professional" && q.BillingClass != "institutional" {
		return bad("billing_class", "billing_class must be 'professional' or 'institutional'.")
	}

	q.IncludeRelated = true
	if v := c.Query("include_related"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return bad("include_related", "include_related must be true or false.")
		}
		q.IncludeRelated = b
	}

	q.Sort = store.SortRate
	if v := strings.ToLower(strings.TrimSpace(c.Query("sort"))); v != "" {
		if !store.ValidSort(v) {
			return bad("sort", "sort must be one of rate, code, billing, provider.")
		}
		q.Sort = store.Sort(v)
	}
	switch strings.ToLower(strings.TrimSpace(c.Query("order"))) {
	case "", "asc":
	case "desc":
		q.Desc = true
	default:
		return bad("order", "order must be asc or desc.")
	}

	var ok bool
	if q.Limit, ok = intParam(c, "limit", defaultLimit, 1, maxLimit); !ok {
		return q, queryEcho{}, false
	}
	if q.Offset, ok = intParam(c, "offset", 0, 0, maxOffset); !ok {
		return q, queryEcho{}, false
	}

	echo := queryEcho{
		Code: q.Code, CodeType: q.CodeType, BillingClass: q.BillingClass,
		IncludeRelated: q.IncludeRelated, Sort: string(q.Sort), Order: "asc",
	}
	if q.Desc {
		echo.Order = "desc"
	}
	if q.NPI != 0 {
		echo.NPI = strconv.FormatInt(q.NPI, 10)
	}
	if q.TIN != "" {
		echo.EIN = store.FormatEIN(q.TIN)
	}
	return q, echo, true
}

// intParam reads an optional integer query param, returning def when it is absent.
// If the value is not an integer in [lo, hi] it writes a 400 and returns ok=false.
func intParam(c *gin.Context, name string, def, lo, hi int) (n int, ok bool) {
	v := strings.TrimSpace(c.Query(name))
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		writeError(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("%s must be an integer between %d and %d.", name, lo, hi), name)
		return 0, false
	}
	return n, true
}
