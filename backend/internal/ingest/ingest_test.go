package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"
)

const fixture = "../../testdata/in-network.json"

// TestLoadFixture checks stats, provider resolution and normalization on the fixture.
func TestLoadFixture(t *testing.T) {
	f, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	data, stats, skips, err := Load(context.Background(), f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := data.Meta["last_updated_on"]; got != "2026-01-01" {
		t.Errorf("last_updated_on = %q", got)
	}
	want := FileStats{Items: 4, Rates: 6, Prices: 6, ProviderGroups: 4, Skipped: 1, DanglingRefs: 1, RemoteRefs: 1, NoProviderRows: 1}
	if stats != want {
		t.Errorf("stats = %+v, want %+v", stats, want)
	}
	if len(skips) != 1 || !strings.Contains(skips[0], "in_network[3]") {
		t.Errorf("skip samples = %v", skips)
	}
	if len(data.Rows) != 6 {
		t.Fatalf("rows = %d, want 6", len(data.Rows))
	}

	// provider_references come after in_network in the fixture, so this
	// checks that references are resolved at the end.
	first := data.Rows[0]
	if len(first.Providers) != 1 || data.Providers[first.Providers[0]].BusinessName != "Alpha Medical" {
		t.Errorf("first row providers = %v", first.Providers)
	}
	alpha := data.Providers[first.Providers[0]]
	if len(alpha.NPIs) != 2 {
		t.Errorf("duplicate NPIs not removed: %v", alpha.NPIs)
	}
	if alpha.TINDigits != "111111111" {
		t.Errorf("TINDigits = %q", alpha.TINDigits)
	}

	// Reference 2 has two provider groups.
	if got := len(data.Rows[1].Providers); got != 2 {
		t.Errorf("row 1 providers = %d, want 2", got)
	}
	if data.Rows[1].Rate != 150 {
		t.Errorf("string rate not parsed: %v", data.Rows[1].Rate)
	}

	// Inline provider group with a string NPI.
	inline := data.Providers[data.Rows[2].Providers[0]]
	if inline.BusinessName != "Inline Clinic" || len(inline.NPIs) != 1 || inline.NPIs[0] != 1111111111 {
		t.Errorf("inline provider = %+v", inline)
	}

	// Codes and code types are normalized.
	last := data.Rows[5]
	if last.BillingCode != "G0121" || last.BillingCodeType != "HCPCS" {
		t.Errorf("code = %q type = %q", last.BillingCode, last.BillingCodeType)
	}
	if len(last.Providers) != 0 {
		t.Errorf("dangling reference should leave no providers")
	}
}

// TestLoadGzip checks that gzip input is detected and parsed.
func TestLoadGzip(t *testing.T) {
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(raw)
	zw.Close()

	data, _, _, err := Load(context.Background(), &buf)
	if err != nil {
		t.Fatalf("Load gzip: %v", err)
	}
	if len(data.Rows) != 6 {
		t.Errorf("rows = %d, want 6", len(data.Rows))
	}
}

// TestLoadTruncated checks that a cut-off file is an error, not partial data.
func TestLoadTruncated(t *testing.T) {
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = Load(context.Background(), bytes.NewReader(raw[:len(raw)/2]))
	if err == nil {
		t.Fatal("expected error for truncated file")
	}
}

// TestLoadNotJSON checks that a non-JSON response (e.g. an HTML error page) is an error.
func TestLoadNotJSON(t *testing.T) {
	_, _, _, err := Load(context.Background(), strings.NewReader("<html>Access denied</html>"))
	if err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}
