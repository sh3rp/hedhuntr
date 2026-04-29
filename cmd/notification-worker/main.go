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
	"hedhuntr/internal/notificationworker"
)

func main() {
	var configPath string
	var maxMessages int
	flag.StringVar(&configPath, "config", "configs/notification-worker.example.json", "path to notification worker config")
	flag.IntVar(&maxMessages, "max-messages", 0, "process this many messages then exit; 0 runs until interrupted")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadNotificationWorker(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := notificationworker.NewService(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize notification worker", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	started := time.Now()
	result, err := service.Run(ctx, maxMessages)
	if err != nil && ctx.Err() == nil {
		logger.Error("notification worker failed", "error", err, "duration", time.Since(started).String())
		os.Exit(1)
	}

	logger.Info(
		"notification worker stopped",
		"messages_processed", result.MessagesProcessed,
		"notifications_sent", result.NotificationsSent,
		"notifications_failed", result.NotificationsFailed,
		"duration", time.Since(started).Round(time.Millisecond).String(),
	)
}
