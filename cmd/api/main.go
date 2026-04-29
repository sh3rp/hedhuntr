package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hedhuntr/internal/api"
	"hedhuntr/internal/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/api.example.json", "path to API config")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadAPI(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiServer, err := api.NewServer(ctx, cfg)
	if err != nil {
		logger.Error("initialize api", "error", err)
		os.Exit(1)
	}
	defer apiServer.Close()
	if err := apiServer.StartRealtime(ctx, logger); err != nil {
		logger.Warn("api realtime bridge unavailable", "error", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.HTTP.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("api stopped")
}
