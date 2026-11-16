// Command server runs the price transparency API (and serves the built UI if present).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"ptexplorer/internal/app"
	"ptexplorer/internal/config"
)

// main reads config, builds the app and runs it until Ctrl+C or SIGTERM.
func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting", "index", cfg.IndexURL, "files", cfg.Files, "port", cfg.Port)
	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
