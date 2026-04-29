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
	"hedhuntr/internal/resumetuningworker"
)

func main() {
	var configPath string
	var maxMessages int
	flag.StringVar(&configPath, "config", "configs/resume-tuning-worker.example.json", "path to resume tuning worker config")
	flag.IntVar(&maxMessages, "max-messages", 0, "process this many messages then exit; 0 runs until interrupted")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadResumeTuningWorker(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := resumetuningworker.NewService(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize resume tuning worker", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	started := time.Now()
	result, err := service.Run(ctx, maxMessages)
	if err != nil && ctx.Err() == nil {
		logger.Error("resume tuning worker failed", "error", err, "duration", time.Since(started).String())
		os.Exit(1)
	}

	logger.Info(
		"resume tuning worker stopped",
		"messages_processed", result.MessagesProcessed,
		"drafts_created", result.DraftsCreated,
		"duration", time.Since(started).Round(time.Millisecond).String(),
	)
}
