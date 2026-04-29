package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/sources"
)

type Service struct {
	cfg     config.SourceProducer
	logger  *slog.Logger
	nc      *nats.Conn
	js      nats.JetStreamContext
	sources []sources.Source
}

type RunResult struct {
	Sources         int
	JobsSeen        int
	EventsPublished int
}

func NewService(cfg config.SourceProducer, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("initialize jetstream: %w", err)
	}
	if err := ensureStream(js, cfg.NATS.Stream); err != nil {
		nc.Close()
		return nil, err
	}

	sourceList, err := sources.Build(cfg.Sources)
	if err != nil {
		nc.Close()
		return nil, err
	}

	return &Service{
		cfg:     cfg,
		logger:  logger,
		nc:      nc,
		js:      js,
		sources: sourceList,
	}, nil
}

func (s *Service) Close() {
	if s.nc != nil {
		s.nc.Drain()
		s.nc.Close()
	}
}

func (s *Service) RunOnce(ctx context.Context) (RunResult, error) {
	var result RunResult

	for _, source := range s.sources {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		result.Sources++
		sourceLogger := s.logger.With("source", source.Name(), "type", source.Type())
		sourceLogger.Info("fetching source")

		jobs, err := source.Fetch(ctx)
		if err != nil {
			return result, fmt.Errorf("fetch source %q: %w", source.Name(), err)
		}

		result.JobsSeen += len(jobs)
		sourceLogger.Info("source fetched", "jobs", len(jobs))

		for _, job := range jobs {
			envelope := events.NewJobDiscovered(source.Name(), job)
			if err := s.publish(ctx, envelope); err != nil {
				return result, fmt.Errorf("publish job %q from source %q: %w", job.ExternalID, source.Name(), err)
			}
			result.EventsPublished++
		}
	}

	return result, nil
}

func (s *Service) publish(ctx context.Context, envelope events.Envelope[events.JobDiscoveredPayload]) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.NATS.PublishTimeout)
	defer cancel()

	ack, err := s.js.Publish(
		events.SubjectJobsDiscovered,
		data,
		nats.Context(publishCtx),
		nats.MsgId(envelope.IdempotencyKey),
	)
	if err != nil {
		return err
	}

	s.logger.Debug(
		"published job event",
		"subject", events.SubjectJobsDiscovered,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
		"idempotency_key", envelope.IdempotencyKey,
	)
	return nil
}

func ensureStream(js nats.JetStreamContext, streamName string) error {
	_, err := js.StreamInfo(streamName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("inspect jetstream stream %q: %w", streamName, err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"jobs.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create jetstream stream %q: %w", streamName, err)
	}
	return nil
}
