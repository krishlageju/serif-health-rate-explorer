package index

import (
	"strings"
	"testing"
)

const indexJSON = `{
  "reporting_entity_name": "Test Payer",
  "reporting_structure": [
    {"reporting_plans": [{"plan_name": "Bronze", "plan_id": "A1"}],
     "in_network_files": [{"description": "in-network", "location": "https://x.test/files/rates%20one.json"}]},
    {"reporting_plans": [{"plan_name": "Gold", "plan_id": 42}],
     "in_network_files": [
       {"description": "in-network", "location": "https://x.test/files/rates%20one.json"},
       {"description": "in-network", "location": "https://x.test/files/rates-two.json"}]}
  ]
}`

// TestInNetworkFiles checks URL dedupe, file names and plan merging.
func TestInNetworkFiles(t *testing.T) {
	idx, err := Parse(strings.NewReader(indexJSON))
	if err != nil {
		t.Fatal(err)
	}
	files := InNetworkFiles(idx)
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2 (deduplicated)", len(files))
	}
	if files[0].FileName != "rates one.json" {
		t.Errorf("file name = %q", files[0].FileName)
	}
	if len(files[0].Plans) != 2 || files[0].Plans[1].PlanID != "42" {
		t.Errorf("plans for shared file = %+v", files[0].Plans)
	}
}

// TestSelect checks the first / all / substring file selection rules.
func TestSelect(t *testing.T) {
	idx, _ := Parse(strings.NewReader(indexJSON))
	files := InNetworkFiles(idx)
	cases := map[string]int{"": 1, "first": 1, "all": 2, "TWO": 1, "one, two": 2}
	for spec, want := range cases {
		got, err := Select(files, spec)
		if err != nil || len(got) != want {
			t.Errorf("Select(%q) = %d files, err %v; want %d", spec, len(got), err, want)
		}
	}
	if _, err := Select(files, "missing"); err == nil {
		t.Error("expected error for unmatched spec")
	}
}

// TestParseRejectsEmptyIndex checks that empty or invalid indexes are errors.
func TestParseRejectsEmptyIndex(t *testing.T) {
	if _, err := Parse(strings.NewReader(`{"reporting_structure": []}`)); err == nil {
		t.Error("expected error")
	}
	if _, err := Parse(strings.NewReader(`not json`)); err == nil {
		t.Error("expected error")
	}
}
