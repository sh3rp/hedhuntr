package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/descriptionfetcher"
)

func main() {
	var configPath string
	var maxMessages int
	flag.StringVar(&configPath, "config", "configs/description-fetcher.example.json", "path to description fetcher config")
	flag.IntVar(&maxMessages, "max-messages", 0, "process this many messages then exit; 0 runs until interrupted")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadDescriptionFetcher(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := descriptionfetcher.NewService(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize description fetcher", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	started := time.Now()
	result, err := service.Run(ctx, maxMessages)
	if err != nil && ctx.Err() == nil {
		logger.Error("description fetcher failed", "error", err, "duration", time.Since(started).String())
		os.Exit(1)
	}

	logger.Info(
		"description fetcher stopped",
		"messages_processed", result.MessagesProcessed,
		"descriptions_fetched", result.DescriptionsFetched,
		"duration", time.Since(started).Round(time.Millisecond).String(),
	)
}
