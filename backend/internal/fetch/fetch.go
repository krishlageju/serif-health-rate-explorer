// Package fetch opens files by URL or local path, with an optional disk cache
// for large downloads.
package fetch

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ptexplorer/internal/index"
)

// Client opens files by URL or path and caches large downloads on disk.
type Client struct {
	HTTP      *http.Client
	CacheDir  string
	UserAgent string
}

// Progress is called as bytes arrive; total is -1 when unknown. It is an alias so callers
// can pass a plain func(done, total int64) without importing this package.
type Progress = func(done, total int64)

// isRemote reports whether loc is an http(s) URL rather than a local path.
func isRemote(loc string) bool {
	l := strings.ToLower(loc)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}

// localPath strips an optional file:// prefix.
func localPath(loc string) string {
	return strings.TrimPrefix(loc, "file://")
}

// Open returns a reader for loc without caching, plus its size (-1 if unknown).
func (c *Client) Open(ctx context.Context, loc string) (io.ReadCloser, int64, error) {
	if !isRemote(loc) {
		f, err := os.Open(localPath(loc))
		if err != nil {
			return nil, 0, err
		}
		size := int64(-1)
		if st, err := f.Stat(); err == nil {
			size = st.Size()
		}
		return f, size, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
	if err != nil {
		return nil, 0, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("GET %s: %w", loc, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("GET %s: unexpected status %s", loc, resp.Status)
	}
	return resp.Body, resp.ContentLength, nil
}

// Cached returns a local path for loc, downloading remote files into CacheDir once.
// Later calls reuse the cached copy; local paths are returned as is.
func (c *Client) Cached(ctx context.Context, loc string, progress Progress) (path string, cached bool, err error) {
	if !isRemote(loc) {
		return localPath(loc), true, nil
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create cache dir: %w", err)
	}
	dest := filepath.Join(c.CacheDir, cacheName(loc))
	if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
		return dest, true, nil
	}

	body, size, err := c.Open(ctx, loc)
	if err != nil {
		return "", false, err
	}
	defer body.Close()

	// Write to a temp file and rename, so a failed download never leaves a
	// partial file that looks like a valid cache entry.
	tmp, err := os.CreateTemp(c.CacheDir, "download-*.tmp")
	if err != nil {
		return "", false, err
	}
	defer os.Remove(tmp.Name())

	w := io.Writer(tmp)
	if progress != nil {
		w = &progressWriter{w: tmp, total: size, fn: progress}
	}
	if _, err := io.Copy(w, body); err != nil {
		tmp.Close()
		return "", false, fmt.Errorf("download %s: %w", loc, err)
	}
	if err := tmp.Close(); err != nil {
		return "", false, err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return "", false, err
	}
	return dest, false, nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// cacheName builds a stable, filesystem-safe cache file name from a URL.
func cacheName(loc string) string {
	sum := sha1.Sum([]byte(loc))
	return hex.EncodeToString(sum[:6]) + "-" + unsafeChars.ReplaceAllString(index.FileName(loc), "_")
}

// progressWriter reports bytes written, to show download progress.
type progressWriter struct {
	w     io.Writer
	done  int64
	total int64
	fn    Progress
}

// Write passes bytes through and reports the running total.
func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	p.fn(p.done, p.total)
	return n, err
}
