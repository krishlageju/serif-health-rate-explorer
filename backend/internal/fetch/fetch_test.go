package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestCachedDownloadsOnce checks the first call downloads and later calls reuse the cache.
func TestCachedDownloadsOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), CacheDir: t.TempDir()}
	var lastDone int64
	path, cached, err := c.Cached(context.Background(), srv.URL+"/rates.json", func(done, _ int64) { lastDone = done })
	if err != nil || cached {
		t.Fatalf("first call: cached=%v err=%v", cached, err)
	}
	if lastDone != 11 {
		t.Errorf("progress reported %d bytes, want 11", lastDone)
	}
	if b, _ := os.ReadFile(path); string(b) != `{"ok":true}` {
		t.Errorf("cached content = %q", b)
	}

	path2, cached, err := c.Cached(context.Background(), srv.URL+"/rates.json", nil)
	if err != nil || !cached || path2 != path {
		t.Fatalf("second call: path=%s cached=%v err=%v", path2, cached, err)
	}
	if hits.Load() != 1 {
		t.Errorf("server hit %d times, want 1", hits.Load())
	}
}

// TestCachedFailureLeavesNoFile checks a failed download never leaves a cache entry behind.
func TestCachedFailureLeavesNoFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	dir := t.TempDir()
	c := &Client{HTTP: srv.Client(), CacheDir: dir}
	if _, _, err := c.Cached(context.Background(), srv.URL+"/rates.json", nil); err == nil {
		t.Fatal("expected error for 403")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("cache dir has %d entries after failure, want 0", len(entries))
	}
}

// TestLocalPaths checks local paths and file:// URLs are used directly, without caching.
func TestLocalPaths(t *testing.T) {
	f := filepath.Join(t.TempDir(), "local.json")
	os.WriteFile(f, []byte("{}"), 0o644)
	c := &Client{HTTP: http.DefaultClient, CacheDir: t.TempDir()}
	for _, loc := range []string{f, "file://" + f} {
		path, _, err := c.Cached(context.Background(), loc, nil)
		if err != nil || path != f {
			t.Errorf("Cached(%q) = %q, %v", loc, path, err)
		}
		rc, size, err := c.Open(context.Background(), loc)
		if err != nil || size != 2 {
			t.Errorf("Open(%q) size=%d err=%v", loc, size, err)
			continue
		}
		rc.Close()
	}
}
