package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/producer"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg         config.Scheduler
	producerCfg config.SourceProducer
	store       *store.Store
	logger      *slog.Logger
}

type RunResult struct {
	SourcesEvaluated int
	SourcesRun       int
	SourcesSkipped   int
	SourcesFailed    int
	JobsSeen         int
	EventsPublished  int
}

func NewService(ctx context.Context, cfg config.Scheduler, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	producerCfg, err := config.LoadSourceProducer(cfg.SourceProducerConfig)
	if err != nil {
		return nil, fmt.Errorf("load source producer config: %w", err)
	}

	st, err := store.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	service := &Service{
		cfg:         cfg,
		producerCfg: producerCfg,
		store:       st,
		logger:      logger,
	}
	if err := service.syncSources(ctx); err != nil {
		st.Close()
		return nil, err
	}

	return service, nil
}

func (s *Service) Close() error {
	return s.store.Close()
}

func (s *Service) RunOnce(ctx context.Context, now time.Time) (RunResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	sources, err := s.store.ListEnabledJobSources(ctx)
	if err != nil {
		return RunResult{}, err
	}

	due := DueSources(sources, now)
	result := RunResult{
		SourcesEvaluated: len(sources),
		SourcesSkipped:   len(sources) - len(due),
	}

	for _, source := range due {
		sourceResult, err := s.runSource(ctx, source, now)
		if err != nil {
			result.SourcesFailed++
			s.logger.Error("scheduled source failed", "source", source.Name, "error", err)
			continue
		}
		result.SourcesRun++
		result.JobsSeen += sourceResult.JobsSeen
		result.EventsPublished += sourceResult.EventsPublished
	}

	return result, nil
}

func (s *Service) RunLoop(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if _, err := s.RunOnce(ctx, time.Now().UTC()); err != nil {
			s.logger.Error("scheduler tick failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) syncSources(ctx context.Context) error {
	producerSources := make(map[string]config.SourceConfig, len(s.producerCfg.Sources))
	for _, source := range s.producerCfg.Sources {
		producerSources[source.Name] = source
	}

	for _, scheduled := range s.cfg.Sources {
		producerSource, ok := producerSources[scheduled.Name]
		if !ok {
			return fmt.Errorf("scheduled source %q is not present in source producer config", scheduled.Name)
		}

		enabled := scheduled.Enabled && producerSource.Enabled
		if _, err := s.store.UpsertJobSource(ctx, store.UpsertJobSourceParams{
			Name:            scheduled.Name,
			Type:            producerSource.Type,
			Enabled:         enabled,
			Schedule:        scheduled.Schedule,
			IntervalSeconds: int(scheduled.IntervalParsed.Seconds()),
			TimeoutSeconds:  int(scheduled.TimeoutParsed.Seconds()),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) runSource(ctx context.Context, source store.JobSource, startedAt time.Time) (producer.RunResult, error) {
	runID, err := s.store.BeginSourceRun(ctx, source, startedAt)
	if err != nil {
		return producer.RunResult{}, err
	}

	sourceCfg, err := s.sourceProducerConfig(source.Name)
	if err != nil {
		s.completeRun(ctx, runID, "failed", producer.RunResult{}, err)
		return producer.RunResult{}, err
	}

	timeout := time.Duration(source.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	service, err := producer.NewService(sourceCfg, s.logger.With("component", "source-producer", "source", source.Name))
	if err != nil {
		s.completeRun(ctx, runID, "failed", producer.RunResult{}, err)
		return producer.RunResult{}, err
	}
	defer service.Close()

	result, err := service.RunOnce(runCtx)
	if err != nil {
		s.completeRun(ctx, runID, "failed", result, err)
		return result, err
	}

	if err := s.completeRun(ctx, runID, "succeeded", result, nil); err != nil {
		return result, err
	}

	s.logger.Info(
		"scheduled source completed",
		"source", source.Name,
		"jobs_seen", result.JobsSeen,
		"events_published", result.EventsPublished,
	)
	return result, nil
}

func (s *Service) completeRun(ctx context.Context, runID int64, status string, result producer.RunResult, runErr error) error {
	errorText := ""
	if runErr != nil {
		errorText = runErr.Error()
	}
	return s.store.CompleteSourceRun(ctx, store.CompleteSourceRunParams{
		RunID:           runID,
		Status:          status,
		JobsSeen:        result.JobsSeen,
		EventsPublished: result.EventsPublished,
		Error:           errorText,
		FinishedAt:      time.Now().UTC(),
	})
}

func (s *Service) sourceProducerConfig(sourceName string) (config.SourceProducer, error) {
	cfg := s.producerCfg
	cfg.Sources = nil

	for _, source := range s.producerCfg.Sources {
		if source.Name == sourceName {
			source.Enabled = true
			cfg.Sources = []config.SourceConfig{source}
			return cfg, nil
		}
	}
	return config.SourceProducer{}, fmt.Errorf("source %q is not configured", sourceName)
}

func DueSources(sources []store.JobSource, now time.Time) []store.JobSource {
	var due []store.JobSource
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		if source.LastRunAt == nil {
			due = append(due, source)
			continue
		}
		interval := time.Duration(source.IntervalSeconds) * time.Second
		if interval <= 0 {
			continue
		}
		if !source.LastRunAt.Add(interval).After(now) {
			due = append(due, source)
		}
	}
	return due
}
