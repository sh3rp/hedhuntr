package matchingworker

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
	"hedhuntr/internal/matcher"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.MatchingWorker
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
}

type RunResult struct {
	MessagesProcessed int
	JobsMatched       int
	ApplicationsReady int
}

func NewService(ctx context.Context, cfg config.MatchingWorker, logger *slog.Logger) (*Service, error) {
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
		events.SubjectJobsParsed,
		cfg.Consumer.Durable,
		nats.BindStream(cfg.NATS.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectJobsParsed, err)
	}

	return &Service{cfg: cfg, logger: logger, nc: nc, js: js, sub: sub, store: st}, nil
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
			ready, err := s.processMessage(ctx, msg)
			if err != nil {
				s.logger.Error("process parsed job event", "error", err, "subject", msg.Subject)
				msg.Nak()
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}
			result.MessagesProcessed++
			result.JobsMatched++
			if ready {
				result.ApplicationsReady++
			}
		}
	}

	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) (bool, error) {
	var envelope events.Envelope[events.JobParsedPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return false, fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventJobParsed {
		return false, fmt.Errorf("unexpected event type %q", envelope.EventType)
	}

	profile, err := s.store.GetCandidateProfile(ctx, s.cfg.Matching.CandidateProfileID)
	if err != nil {
		return false, err
	}
	job, err := s.store.GetJobForMatching(ctx, envelope.Payload.JobID)
	if err != nil {
		return false, err
	}

	match := matcher.Score(profile, job.MatcherJob())
	if err := s.store.SaveJobMatch(ctx, store.SaveJobMatchParams{
		JobID:              job.ID,
		CandidateProfileID: profile.ID,
		Score:              match.Score,
		MatchedSkills:      match.MatchedSkills,
		MissingSkills:      match.MissingSkills,
		Notes:              match.Notes,
	}); err != nil {
		return false, err
	}

	matchedAt := time.Now().UTC()
	jobMatched := events.NewJobMatched(envelope.Source, envelope.CorrelationID, events.JobMatchedPayload{
		JobID:              job.ID,
		CandidateProfileID: profile.ID,
		Score:              match.Score,
		MatchedSkills:      match.MatchedSkills,
		MissingSkills:      match.MissingSkills,
		Notes:              match.Notes,
		MatchedAt:          matchedAt,
	})
	if err := s.publish(ctx, events.SubjectJobsMatched, jobMatched); err != nil {
		return false, fmt.Errorf("publish jobs.matched: %w", err)
	}

	ready := match.Score >= s.cfg.Matching.ReadyThreshold
	if ready {
		if err := s.store.MarkApplicationReady(ctx, job.ID, profile.ID, match.Score); err != nil {
			return false, err
		}
		applicationReady := events.NewApplicationReady(envelope.Source, envelope.CorrelationID, events.ApplicationReadyPayload{
			JobID:              job.ID,
			CandidateProfileID: profile.ID,
			MatchScore:         match.Score,
			ReadyAt:            matchedAt,
		})
		if err := s.publish(ctx, events.SubjectApplicationsReady, applicationReady); err != nil {
			return false, fmt.Errorf("publish applications.ready: %w", err)
		}
	}

	s.logger.Info("matched job", "job_id", job.ID, "profile_id", profile.ID, "score", match.Score, "ready", ready)
	return ready, nil
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
