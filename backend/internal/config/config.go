// Package config reads settings from environment variables.
package config

import (
	"os"
	"strings"
	"time"
)

const DefaultIndexURL = "https://www.centene.com/content/dam/centene/Centene%20Corporate/json/DOCUMENT/2026-08-31_fidelis_index.json"

// Config is every setting the server reads from the environment.
type Config struct {
	Port         string        // PORT
	IndexURL     string        // INDEX_URL: URL or local path of the table-of-contents file
	Files        string        // TIC_FILES: "first", "all", or comma-separated URL substrings
	CacheDir     string        // CACHE_DIR: where downloaded rate files are kept
	StaticDir    string        // STATIC_DIR: built frontend to serve (optional)
	FetchTimeout time.Duration // FETCH_TIMEOUT: per download, e.g. "10m"
	CORSOrigins  []string      // CORS_ORIGINS: comma-separated browser origins allowed to call the API
}

// Load reads Config from the environment, using defaults for anything unset.
func Load() Config {
	c := Config{
		Port:         env("PORT", "8080"),
		IndexURL:     env("INDEX_URL", DefaultIndexURL),
		Files:        env("TIC_FILES", "first"),
		CacheDir:     env("CACHE_DIR", "./data/cache"),
		StaticDir:    env("STATIC_DIR", "../frontend/dist"),
		FetchTimeout: 15 * time.Minute,
	}
	if v := os.Getenv("FETCH_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.FetchTimeout = d
		}
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}
	return c
}

// env returns the environment variable key, or def when it is empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
