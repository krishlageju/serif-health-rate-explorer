package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"ptexplorer/internal/fetch"
	"ptexplorer/internal/index"
	"ptexplorer/internal/ingest"
	"ptexplorer/internal/loader"
	"ptexplorer/internal/search"
	"ptexplorer/internal/store"
)

var quietLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// wire connects the real loader and search service, as internal/app does, for an index path.
func wire(indexPath string) (*loader.Loader, Deps) {
	stores := &store.Holder{}
	ld := loader.New(
		loader.Config{IndexURL: indexPath, Files: "all"},
		loader.Deps{Fetcher: &fetch.Client{HTTP: http.DefaultClient}, Parser: ingest.Parser{}, Publisher: stores, Log: quietLog},
	)
	return ld, Deps{Searcher: search.NewService(stores), Status: ld, Log: quietLog}
}

// newLoadedDeps runs the real loader against the fixture via a local index file.
func newLoadedDeps(t *testing.T) Deps {
	t.Helper()
	fixture, err := filepath.Abs("../../testdata/in-network.json")
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(t.TempDir(), "index.json")
	idx := `{"reporting_entity_name":"Test Payer","reporting_structure":[{"reporting_plans":[{"plan_name":"Bronze","plan_id":"A1"}],"in_network_files":[{"description":"in-network","location":"` + fixture + `"}]}]}`
	if err := os.WriteFile(indexPath, []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	ld, deps := wire(indexPath)
	ld.Run(context.Background())
	if st := ld.Status(); st.State != loader.StateReady {
		t.Fatalf("loader state = %s (%s)", st.State, st.Message)
	}
	return deps
}

// get performs a GET against h and decodes the JSON response body.
func get(t *testing.T, h http.Handler, url string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: invalid JSON: %s", url, w.Body.String())
	}
	return w.Code, body
}

// TestSearchEndpoint checks a filtered search, validation errors and the codes/sources endpoints.
func TestSearchEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(newLoadedDeps(t))

	code, body := get(t, r, "/api/search?code=%2099213%20&npi=1234567890&ein=44-4444444")
	if code != http.StatusOK {
		t.Fatalf("status = %d body = %v", code, body)
	}
	if body["total"].(float64) != 2 {
		t.Errorf("total = %v", body["total"])
	}
	first := body["results"].([]any)[0].(map[string]any)
	prov := first["providers"].([]any)[0].(map[string]any)
	if prov["matched"] != true || prov["business_name"] != "Gamma Group" {
		t.Errorf("matched provider should be listed first: %v", prov)
	}
	if first["source"].(map[string]any)["plan_count"].(float64) != 1 {
		t.Errorf("source = %v", first["source"])
	}

	bad := map[string]string{
		"/api/search":                                 "code",
		"/api/search?code=99213&npi=12345":            "npi",
		"/api/search?code=99213&ein=12-34":            "ein",
		"/api/search?code=99213&sort=price":           "sort",
		"/api/search?code=99213&order=up":             "order",
		"/api/search?code=99213&limit=0":              "limit",
		"/api/search?code=99213&billing_class=x":      "billing_class",
		"/api/search?code=99213&include_related=nope": "include_related",
		"/api/search?code=<script>":                   "code",
	}
	for url, field := range bad {
		code, body := get(t, r, url)
		if code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", url, code)
			continue
		}
		if got := body["error"].(map[string]any)["field"]; got != field {
			t.Errorf("%s: field = %v, want %s", url, got, field)
		}
	}

	code, body = get(t, r, "/api/codes?q=992")
	if code != http.StatusOK || len(body["codes"].([]any)) == 0 {
		t.Errorf("codes: %d %v", code, body)
	}
	code, body = get(t, r, "/api/sources")
	if code != http.StatusOK || len(body["files"].([]any)) != 1 {
		t.Errorf("sources: %d %v", code, body)
	}
}

// TestSearchWhileLoading checks the 503 response before data is ready and after a failed load.
func TestSearchWhileLoading(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ld, deps := wire("/does/not/exist.json")
	r := NewRouter(deps)

	code, body := get(t, r, "/api/search?code=99213")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", code)
	}
	if body["error"].(map[string]any)["code"] != "not_ready" {
		t.Errorf("body = %v", body)
	}

	// A failed load reports an error state instead of crashing.
	ld.Run(context.Background())
	if st := ld.Status(); st.State != loader.StateError || st.Message == "" {
		t.Errorf("status after failed load = %+v", st)
	}
	code, _ = get(t, r, "/api/search?code=99213")
	if code != http.StatusServiceUnavailable {
		t.Errorf("status after failure = %d, want 503", code)
	}
}

// fakeSearcher is a stand-in Searcher, showing the router works with any implementation.
type fakeSearcher struct{ err error }

// Search returns one fixed rate, or the configured error.
func (f fakeSearcher) Search(_ context.Context, q store.Query) (*search.Page, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &search.Page{Total: 1, Results: []search.Rate{{BillingCode: q.Code, Rate: 42}}}, nil
}

// Suggest returns no codes, or the configured error.
func (f fakeSearcher) Suggest(context.Context, string, int) ([]store.CodeSummary, error) {
	return []store.CodeSummary{}, f.err
}

// LoadedSources returns no sources, or the configured error.
func (f fakeSearcher) LoadedSources(context.Context) ([]store.Source, error) { return nil, f.err }

// fakeStatus is a stand-in StatusReader with a fixed state.
type fakeStatus struct{ state loader.State }

// Status returns the fixed state.
func (f fakeStatus) Status() loader.Status { return loader.Status{State: f.state} }

// IndexFiles returns no files.
func (f fakeStatus) IndexFiles() []index.SourceFile { return nil }

// TestRouterWithFakes checks the HTTP layer against fake services: success, not ready, and failure.
func TestRouterWithFakes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name     string
		searcher fakeSearcher
		want     int
	}{
		{"ok", fakeSearcher{}, http.StatusOK},
		{"not ready", fakeSearcher{err: search.ErrNotReady}, http.StatusServiceUnavailable},
		{"unexpected error", fakeSearcher{err: errors.New("boom")}, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter(Deps{Searcher: tc.searcher, Status: fakeStatus{loader.StateLoading}, Log: quietLog})
			code, body := get(t, r, "/api/search?code=99213")
			if code != tc.want {
				t.Fatalf("status = %d, want %d (%v)", code, tc.want, body)
			}
			if tc.want == http.StatusOK && body["results"].([]any)[0].(map[string]any)["rate"].(float64) != 42 {
				t.Errorf("body = %v", body)
			}
		})
	}
}

// TestCORS checks allowed origins get CORS headers and others do not.
func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{Searcher: fakeSearcher{}, Status: fakeStatus{loader.StateReady}, Log: quietLog, CORSOrigins: []string{"https://ui.example"}})
	for origin, want := range map[string]string{"https://ui.example": "https://ui.example", "https://evil.example": ""} {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("origin %s: allow-origin = %q, want %q", origin, got, want)
		}
	}
}
