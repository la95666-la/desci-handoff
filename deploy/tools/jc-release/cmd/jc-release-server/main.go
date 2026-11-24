// Command jc-release-server serves the release API and admin Web UI.
//
// Usage:
//
//	jc-release-server -config /etc/jc-release/server.yaml
package main

import (
	"context"
	"embed"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joincare/jc-release/internal/server"
)

//go:embed web/*
var webFS embed.FS

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "/etc/jc-release/server.yaml", "path to server yaml config")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := server.LoadConfig(configPath)
	if err != nil {
		logger.Error("load config failed", "err", err)
		os.Exit(1)
	}
	store, err := server.NewStorage(cfg.DataDir)
	if err != nil {
		logger.Error("init storage failed", "err", err)
		os.Exit(1)
	}
	if err := server.SetWebAssets(webFS, "web"); err != nil {
		logger.Error("embed web assets failed", "err", err)
		os.Exit(1)
	}

	srv := server.New(cfg, store, logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}
