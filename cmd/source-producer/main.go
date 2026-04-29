package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/producer"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "configs/source-producer.example.json", "path to source producer config")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadSourceProducer(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := producer.NewService(cfg, logger)
	if err != nil {
		logger.Error("initialize service", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	started := time.Now()
	result, err := service.RunOnce(ctx)
	if err != nil {
		logger.Error("source producer failed", "error", err, "duration", time.Since(started).String())
		os.Exit(1)
	}

	logger.Info(
		"source producer completed",
		"sources", result.Sources,
		"jobs_seen", result.JobsSeen,
		"events_published", result.EventsPublished,
		"duration", fmt.Sprintf("%s", time.Since(started).Round(time.Millisecond)),
	)
}
