// Package api is the HTTP layer (Gin). It validates requests and encodes responses;
// the actual work happens in services it receives through interfaces.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ptexplorer/internal/index"
	"ptexplorer/internal/loader"
	"ptexplorer/internal/search"
	"ptexplorer/internal/store"
)

// Searcher answers rate searches. *search.Service implements it in-process; an HTTP
// client for a separate search service could implement it instead.
type Searcher interface {
	Search(ctx context.Context, q store.Query) (*search.Page, error)
	Suggest(ctx context.Context, q string, limit int) ([]store.CodeSummary, error)
	LoadedSources(ctx context.Context) ([]store.Source, error)
}

// StatusReader reports ingestion progress. *loader.Loader implements it.
type StatusReader interface {
	Status() loader.Status
	IndexFiles() []index.SourceFile
}

// Deps are everything the router needs, passed in by the composition root (internal/app).
type Deps struct {
	Searcher    Searcher
	Status      StatusReader
	Log         *slog.Logger
	StaticDir   string   // built frontend to serve; empty or missing means API only
	CORSOrigins []string // origins allowed to call the API from a browser; empty means same-origin only
}

// Server holds the handlers' dependencies.
type Server struct {
	searcher Searcher
	status   StatusReader
}

// NewRouter builds the Gin engine. If StaticDir holds a built frontend, it is served
// for every non-API path.
func NewRouter(d Deps) *gin.Engine {
	s := &Server{searcher: d.Searcher, status: d.Status}
	r := gin.New()
	// The app never reads client IPs; trusting no proxies also silences Gin's startup warning.
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery(), requestLogger(d.Log))
	if len(d.CORSOrigins) > 0 {
		r.Use(cors(d.CORSOrigins))
	}

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	api.GET("/status", s.getStatus)
	api.GET("/sources", s.sources)
	api.GET("/search", s.search)
	api.GET("/codes", s.codes)

	frontend := hasFrontend(d.StaticDir)
	if frontend {
		r.Static("/assets", filepath.Join(d.StaticDir, "assets"))
		d.Log.Info("serving frontend", "dir", d.StaticDir)
	}
	r.NoRoute(func(c *gin.Context) {
		switch {
		case strings.HasPrefix(c.Request.URL.Path, "/api/"):
			writeError(c, http.StatusNotFound, "not_found", "Unknown API route.", "")
		case frontend:
			serveFrontendFile(c, d.StaticDir)
		default:
			c.Status(http.StatusNotFound)
		}
	})
	return r
}

// hasFrontend reports whether dir contains a built UI (an index.html file).
func hasFrontend(dir string) bool {
	if dir == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !st.IsDir()
}

// serveFrontendFile serves the requested file from dir, or index.html for page routes.
func serveFrontendFile(c *gin.Context, dir string) {
	f := filepath.Join(dir, filepath.Clean("/"+c.Request.URL.Path)) // Clean blocks "../" escapes
	if st, err := os.Stat(f); err == nil && !st.IsDir() {
		c.File(f)
		return
	}
	c.File(filepath.Join(dir, "index.html"))
}

// cors lets browsers on the listed origins call the API, for when the UI is hosted separately.
func cors(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if o := c.GetHeader("Origin"); o != "" && slices.Contains(origins, o) {
			c.Header("Access-Control-Allow-Origin", o)
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Accept, Content-Type")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// requestLogger logs each API request with status and duration (status polling is skipped).
func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.URL.Path == "/api/status" {
			return
		}
		log.Info("request", "method", c.Request.Method, "path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery, "status", c.Writer.Status(), "took", time.Since(start).Round(time.Microsecond))
	}
}

// apiError is the body of every error response.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// writeError aborts the request with a JSON body: {error: {code, message, field}}.
func writeError(c *gin.Context, status int, code, msg, field string) {
	c.AbortWithStatusJSON(status, gin.H{"error": apiError{Code: code, Message: msg, Field: field}})
}

// getStatus handles GET /api/status: ingestion state and per-file progress.
func (s *Server) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.status.Status())
}

// writeNotReady writes a 503 that explains why data isn't available, with the current status.
func (s *Server) writeNotReady(c *gin.Context) {
	status := s.status.Status()
	msg := "Rate data is still loading. Try again shortly."
	if status.State == loader.StateError {
		msg = "Rate data failed to load: " + status.Message
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"error":  apiError{Code: "not_ready", Message: msg},
		"status": status,
	})
}

// writeServiceError maps an error from a service to an HTTP response.
func (s *Server) writeServiceError(c *gin.Context, err error) {
	if errors.Is(err, search.ErrNotReady) {
		s.writeNotReady(c)
		return
	}
	writeError(c, http.StatusInternalServerError, "internal", "Something went wrong. Try again.", "")
}

// sourceFile is an index file plus whether it was loaded.
type sourceFile struct {
	index.SourceFile
	Loaded   bool `json:"loaded"`
	SourceID *int `json:"source_id,omitempty"`
}

// sources handles GET /api/sources: index metadata and every in-network file with its plans.
func (s *Server) sources(c *gin.Context) {
	loadedID := map[string]int{}
	if loaded, err := s.searcher.LoadedSources(c.Request.Context()); err == nil {
		for _, src := range loaded {
			loadedID[src.URL] = src.ID
		}
	}
	files := []sourceFile{}
	for _, f := range s.status.IndexFiles() {
		d := sourceFile{SourceFile: f}
		if id, ok := loadedID[f.URL]; ok {
			d.Loaded = true
			d.SourceID = &id
		}
		files = append(files, d)
	}
	c.JSON(http.StatusOK, gin.H{"index": s.status.Status().Index, "files": files})
}

// codes handles GET /api/codes: autocomplete suggestions for a code or name fragment.
func (s *Server) codes(c *gin.Context) {
	// No limit by default: the whole code list is about 1-2k entries, so
	// returning every match is cheap. Clients may still pass ?limit=N.
	limit, ok := intParam(c, "limit", 0, 1, 100000)
	if !ok {
		return
	}
	codes, err := s.searcher.Suggest(c.Request.Context(), c.Query("q"), limit)
	if err != nil {
		s.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"codes": codes})
}
