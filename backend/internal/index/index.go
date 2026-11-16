// Package index reads a Transparency in Coverage table-of-contents file and
// decides which in-network files to load.
package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"ptexplorer/internal/tic"
)

// SourceFile is one in-network file listed in the index, with every plan
// that points at it.
type SourceFile struct {
	URL         string              `json:"url"`
	FileName    string              `json:"file_name"`
	Description string              `json:"description"`
	Plans       []tic.ReportingPlan `json:"plans"`
}

// Parse decodes an index file and rejects one with no reporting structures.
func Parse(r io.Reader) (*tic.Index, error) {
	var idx tic.Index
	if err := json.NewDecoder(r).Decode(&idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	if len(idx.ReportingStructure) == 0 {
		return nil, fmt.Errorf("index has no reporting_structure entries")
	}
	return &idx, nil
}

// InNetworkFiles lists the unique in-network file URLs in index order. A URL
// listed under several reporting structures gets all of their plans.
func InNetworkFiles(idx *tic.Index) []SourceFile {
	var files []SourceFile
	pos := map[string]int{}
	for _, rs := range idx.ReportingStructure {
		for _, f := range rs.InNetworkFiles {
			loc := strings.TrimSpace(f.Location)
			if loc == "" {
				continue
			}
			i, ok := pos[loc]
			if !ok {
				i = len(files)
				pos[loc] = i
				files = append(files, SourceFile{URL: loc, FileName: FileName(loc), Description: f.Description})
			}
			files[i].Plans = append(files[i].Plans, rs.ReportingPlans...)
		}
	}
	return files
}

// Select picks files by spec: "first" (or empty), "all", or a comma-separated
// list of case-insensitive substrings matched against the URL.
func Select(files []SourceFile, spec string) ([]SourceFile, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("index lists no in-network files")
	}
	spec = strings.TrimSpace(strings.ToLower(spec))
	switch spec {
	case "", "first":
		return files[:1], nil
	case "all":
		return files, nil
	}
	var out []SourceFile
	for _, f := range files {
		u := strings.ToLower(f.URL)
		for _, part := range strings.Split(spec, ",") {
			if p := strings.TrimSpace(part); p != "" && strings.Contains(u, p) {
				out = append(out, f)
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no in-network file matches %q", spec)
	}
	return out, nil
}

// FileName returns the unescaped last path segment of a URL or path.
func FileName(loc string) string {
	if u, err := url.Parse(loc); err == nil && u.Path != "" {
		if p, err := url.PathUnescape(path.Base(u.Path)); err == nil {
			return p
		}
		return path.Base(u.Path)
	}
	return path.Base(loc)
}
