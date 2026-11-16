// Package app is the composition root: the one place that picks an implementation for each
// interface and wires the services. A microservice split changes this wiring, not the services.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"ptexplorer/internal/api"
	"ptexplorer/internal/config"
	"ptexplorer/internal/fetch"
	"ptexplorer/internal/ingest"
	"ptexplorer/internal/loader"
	"ptexplorer/internal/search"
	"ptexplorer/internal/store"
)

// Compile-time checks that each concrete type still satisfies the interface it is wired to.
var (
	_ loader.Fetcher     = (*fetch.Client)(nil)
	_ loader.Parser      = ingest.Parser{}
	_ loader.Publisher   = (*store.Holder)(nil)
	_ search.StoreReader = (*store.Holder)(nil)
	_ api.Searcher       = (*search.Service)(nil)
	_ api.StatusReader   = (*loader.Loader)(nil)
)

// App is the running application: an ingestion loader and an HTTP server.
type App struct {
	log    *slog.Logger
	loader *loader.Loader
	server *http.Server
}

// New builds every service and connects them through their interfaces.
func New(cfg config.Config, log *slog.Logger) *App {
	// Infrastructure: how files are fetched and parsed.
	fetcher := &fetch.Client{
		HTTP:      &http.Client{Timeout: cfg.FetchTimeout},
		CacheDir:  cfg.CacheDir,
		UserAgent: "ptexplorer/1.0 (price transparency take-home)",
	}

	// Shared state: ingestion publishes the store here and search reads it.
	stores := &store.Holder{}

	// Write side: loads files from the index into the store holder.
	ingestion := loader.New(
		loader.Config{IndexURL: cfg.IndexURL, Files: cfg.Files},
		loader.Deps{Fetcher: fetcher, Parser: ingest.Parser{}, Publisher: stores, Log: log},
	)

	// Read side: answers searches from whatever store is currently published.
	searcher := search.NewService(stores)

	// HTTP layer: depends only on the Searcher and StatusReader interfaces.
	router := api.NewRouter(api.Deps{
		Searcher:    searcher,
		Status:      ingestion,
		Log:         log,
		StaticDir:   cfg.StaticDir,
		CORSOrigins: cfg.CORSOrigins,
	})

	return &App{
		log:    log,
		loader: ingestion,
		server: &http.Server{
			Addr:              ":" + cfg.Port,
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       2 * time.Minute,
		},
	}
}

// Run starts ingestion in the background, serves HTTP until ctx is cancelled, then
// shuts down gracefully. The API answers right away; /api/status shows load progress.
func (a *App) Run(ctx context.Context) error {
	go a.loader.Run(ctx)

	serveErr := make(chan error, 1)
	go func() {
		a.log.Info("listening", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	a.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.server.Shutdown(shutdownCtx)
}
