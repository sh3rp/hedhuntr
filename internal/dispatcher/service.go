package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.PersistenceDispatcher
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
}

type RunResult struct {
	MessagesProcessed        int
	JobsSaved                int
	DescriptionFetchRequests int
}

func NewService(ctx context.Context, cfg config.PersistenceDispatcher, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	st, err := store.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("initialize jetstream: %w", err)
	}
	if err := broker.EnsureJobsStream(js, cfg.NATS.Stream); err != nil {
		st.Close()
		nc.Close()
		return nil, err
	}

	sub, err := js.PullSubscribe(
		events.SubjectJobsDiscovered,
		cfg.Consumer.Durable,
		nats.BindStream(cfg.NATS.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectJobsDiscovered, err)
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
		nc:     nc,
		js:     js,
		sub:    sub,
		store:  st,
	}, nil
}

func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
	if s.nc != nil {
		s.nc.Drain()
		s.nc.Close()
	}
}

func (s *Service) Run(ctx context.Context, maxMessages int) (RunResult, error) {
	var result RunResult

	for maxMessages <= 0 || result.MessagesProcessed < maxMessages {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		remaining := s.cfg.Consumer.BatchSize
		if maxMessages > 0 {
			left := maxMessages - result.MessagesProcessed
			if left < remaining {
				remaining = left
			}
		}

		messages, err := s.sub.Fetch(remaining, nats.Context(ctx), nats.MaxWait(s.cfg.Consumer.FetchWait))
		if err != nil {
			if err == nats.ErrTimeout && maxMessages > 0 {
				return result, nil
			}
			if err == nats.ErrTimeout {
				continue
			}
			return result, fmt.Errorf("fetch messages: %w", err)
		}

		for _, msg := range messages {
			processed, err := s.processMessage(ctx, msg)
			if err != nil {
				s.logger.Error("process message", "error", err, "subject", msg.Subject)
				msg.Nak()
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}

			result.MessagesProcessed++
			result.JobsSaved++
			if processed.DescriptionFetchRequested {
				result.DescriptionFetchRequests++
			}
		}
	}

	return result, nil
}

type processedMessage struct {
	DescriptionFetchRequested bool
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) (processedMessage, error) {
	var envelope events.Envelope[events.JobDiscoveredPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return processedMessage{}, fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventJobDiscovered {
		return processedMessage{}, fmt.Errorf("unexpected event type %q", envelope.EventType)
	}
	if envelope.Payload.Source == "" {
		envelope.Payload.Source = envelope.Source
	}

	saveResult, err := s.store.SaveDiscoveredJob(ctx, msg.Subject, envelope, nil)
	if err != nil {
		return processedMessage{}, fmt.Errorf("save discovered job: %w", err)
	}

	savedAt := time.Now().UTC()
	jobSaved := events.NewJobSaved(envelope.Source, envelope.CorrelationID, events.JobSavedPayload{
		JobID:          saveResult.JobID,
		Source:         envelope.Payload.Source,
		ExternalID:     envelope.Payload.ExternalID,
		Title:          envelope.Payload.Title,
		Company:        envelope.Payload.Company,
		SourceURL:      envelope.Payload.SourceURL,
		ApplicationURL: envelope.Payload.ApplicationURL,
		Created:        saveResult.Created,
		SavedAt:        savedAt,
	})
	if err := s.publish(ctx, events.SubjectJobsSaved, jobSaved); err != nil {
		return processedMessage{}, fmt.Errorf("publish jobs.saved: %w", err)
	}

	processed := processedMessage{}
	if strings.TrimSpace(envelope.Payload.Description) == "" {
		fetchRequested := events.NewJobDescriptionFetchRequested(envelope.Source, envelope.CorrelationID, events.JobDescriptionFetchRequestedPayload{
			JobID:          saveResult.JobID,
			Source:         envelope.Payload.Source,
			SourceURL:      envelope.Payload.SourceURL,
			ApplicationURL: envelope.Payload.ApplicationURL,
			RequestedAt:    savedAt,
		})
		if err := s.publish(ctx, events.SubjectJobsDescriptionFetchRequested, fetchRequested); err != nil {
			return processedMessage{}, fmt.Errorf("publish description fetch request: %w", err)
		}
		processed.DescriptionFetchRequested = true
	}

	s.logger.Info(
		"persisted discovered job",
		"job_id", saveResult.JobID,
		"created", saveResult.Created,
		"source", envelope.Payload.Source,
		"title", envelope.Payload.Title,
		"company", envelope.Payload.Company,
	)
	return processed, nil
}

func (s *Service) publish(ctx context.Context, subject string, envelope any) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal %s envelope: %w", subject, err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.NATS.PublishTimeout)
	defer cancel()

	idempotencyKey := idempotencyKeyOf(envelope)
	_, err = s.js.Publish(subject, data, nats.Context(publishCtx), nats.MsgId(idempotencyKey))
	return err
}

func idempotencyKeyOf(envelope any) string {
	data, err := json.Marshal(envelope)
	if err != nil {
		return events.StableID("unknown-envelope")
	}
	var decoded struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.IdempotencyKey == "" {
		return events.StableID(string(data))
	}
	return decoded.IdempotencyKey
}
