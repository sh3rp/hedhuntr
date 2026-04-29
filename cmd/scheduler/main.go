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
	"hedhuntr/internal/scheduler"
)

func main() {
	var configPath string
	var runOnce bool
	flag.StringVar(&configPath, "config", "configs/scheduler.example.json", "path to scheduler config")
	flag.BoolVar(&runOnce, "run-once", false, "run due sources once and exit")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.LoadScheduler(configPath)
	if err != nil {
		logger.Error("load config", "error", err, "path", configPath)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := scheduler.NewService(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize scheduler", "error", err)
		os.Exit(1)
	}
	defer service.Close()

	if runOnce {
		started := time.Now()
		result, err := service.RunOnce(ctx, time.Now().UTC())
		if err != nil {
			logger.Error("scheduler run failed", "error", err)
			os.Exit(1)
		}
		logger.Info(
			"scheduler run completed",
			"sources_evaluated", result.SourcesEvaluated,
			"sources_run", result.SourcesRun,
			"sources_skipped", result.SourcesSkipped,
			"sources_failed", result.SourcesFailed,
			"jobs_seen", result.JobsSeen,
			"events_published", result.EventsPublished,
			"duration", time.Since(started).Round(time.Millisecond).String(),
		)
		return
	}

	if err := service.RunLoop(ctx); err != nil && ctx.Err() == nil {
		logger.Error("scheduler stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("scheduler stopped")
}
