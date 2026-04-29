package parserworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/parser"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.ParserWorker
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
	parser parser.Parser
}

type RunResult struct {
	MessagesProcessed int
	JobsParsed        int
}

func NewService(ctx context.Context, cfg config.ParserWorker, logger *slog.Logger) (*Service, error) {
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
		events.SubjectJobsDescriptionFetched,
		cfg.Consumer.Durable,
		nats.BindStream(cfg.NATS.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectJobsDescriptionFetched, err)
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
		nc:     nc,
		js:     js,
		sub:    sub,
		store:  st,
		parser: parser.New(cfg.Parser.ExtraSkills),
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
			if err := s.processMessage(ctx, msg); err != nil {
				s.logger.Error("process description fetched event", "error", err, "subject", msg.Subject)
				msg.Nak()
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}
			result.MessagesProcessed++
			result.JobsParsed++
		}
	}

	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) error {
	var envelope events.Envelope[events.JobDescriptionFetchedPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventJobDescriptionFetched {
		return fmt.Errorf("unexpected event type %q", envelope.EventType)
	}

	job, err := s.store.GetJobForParsing(ctx, envelope.Payload.JobID)
	if err != nil {
		return err
	}
	description := envelope.Payload.RawText
	if description == "" {
		description = job.Description
	}
	parsed := s.parser.Parse(job.Title, description)
	parsedAt := time.Now().UTC()

	if err := s.store.SaveParsedJob(ctx, store.SaveParsedJobParams{
		JobID:            job.ID,
		Skills:           parsed.Skills,
		Requirements:     parsed.Requirements,
		Responsibilities: parsed.Responsibilities,
		SalaryMin:        parsed.SalaryMin,
		SalaryMax:        parsed.SalaryMax,
		SalaryCurrency:   parsed.SalaryCurrency,
		SalaryPeriod:     parsed.SalaryPeriod,
		RemotePolicy:     parsed.RemotePolicy,
		Seniority:        parsed.Seniority,
		EmploymentType:   parsed.EmploymentType,
		ParsedAt:         parsedAt,
	}); err != nil {
		return err
	}

	jobParsed := events.NewJobParsed(envelope.Source, envelope.CorrelationID, events.JobParsedPayload{
		JobID:            job.ID,
		Source:           job.Source,
		Skills:           parsed.Skills,
		Requirements:     parsed.Requirements,
		Responsibilities: parsed.Responsibilities,
		SalaryMin:        parsed.SalaryMin,
		SalaryMax:        parsed.SalaryMax,
		SalaryCurrency:   parsed.SalaryCurrency,
		SalaryPeriod:     parsed.SalaryPeriod,
		RemotePolicy:     parsed.RemotePolicy,
		Seniority:        parsed.Seniority,
		EmploymentType:   parsed.EmploymentType,
		ParsedAt:         parsedAt,
	})
	if err := s.publish(ctx, events.SubjectJobsParsed, jobParsed); err != nil {
		return fmt.Errorf("publish jobs.parsed: %w", err)
	}

	s.logger.Info("parsed job description", "job_id", job.ID, "skills", len(parsed.Skills), "requirements", len(parsed.Requirements))
	return nil
}

func (s *Service) publish(ctx context.Context, subject string, envelope any) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal %s envelope: %w", subject, err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.NATS.PublishTimeout)
	defer cancel()

	_, err = s.js.Publish(subject, data, nats.Context(publishCtx), nats.MsgId(idempotencyKeyOf(envelope)))
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
