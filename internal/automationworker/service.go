package automationworker

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
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.AutomationWorker
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
}

type RunResult struct {
	MessagesProcessed int
	RunsPrepared      int
	RunsFailed        int
}

func NewService(ctx context.Context, cfg config.AutomationWorker, logger *slog.Logger) (*Service, error) {
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
	sub, err := js.PullSubscribe(events.SubjectAutomationRunRequested, cfg.Consumer.Durable, nats.BindStream(cfg.NATS.Stream), nats.ManualAck())
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectAutomationRunRequested, err)
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
			prepared, err := s.processMessage(ctx, msg)
			if err != nil {
				s.logger.Error("process automation run requested event", "error", err, "subject", msg.Subject)
				msg.Nak()
				result.RunsFailed++
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}
			result.MessagesProcessed++
			if prepared {
				result.RunsPrepared++
			}
		}
	}
	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) (bool, error) {
	var envelope events.Envelope[events.AutomationRunRequestedPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return false, fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventAutomationRunRequested {
		return false, fmt.Errorf("unexpected event type %q", envelope.EventType)
	}

	run, err := s.store.StartAutomationRun(ctx, envelope.Payload.AutomationRunID)
	if err != nil {
		return false, err
	}
	if err := s.store.AddAutomationLog(ctx, store.AutomationLogParams{
		RunID:   run.ID,
		Message: "Automation run started in packet-only mode.",
		Details: map[string]any{"mode": s.cfg.Automation.Mode},
	}); err != nil {
		return false, err
	}
	if err := s.publishStatus(ctx, envelope, events.SubjectAutomationRunStarted, events.EventAutomationRunStarted, run, "Automation run started."); err != nil {
		return false, err
	}

	packet, err := s.store.AutomationPacket(ctx, run.ApplicationID)
	if err != nil {
		failed, markErr := s.store.MarkAutomationFailed(ctx, run.ID, err.Error())
		if markErr == nil {
			_ = s.publishStatus(ctx, envelope, events.SubjectAutomationRunFailed, events.EventAutomationRunFailed, failed, err.Error())
		}
		return false, err
	}
	if err := s.store.AddAutomationLog(ctx, store.AutomationLogParams{
		RunID:   run.ID,
		Message: "Approved application packet loaded. Final submission is blocked pending user review.",
		Details: map[string]any{
			"application_id": packet.ApplicationID,
			"job_title":      packet.Job.Title,
			"company":        packet.Job.Company,
			"resume_path":    packet.Materials.Resume.Path,
		},
	}); err != nil {
		return false, err
	}
	plan := BuildAdapterPlan(packet, s.cfg.Automation.AllowedAdapters)
	if err := s.store.AddAutomationLog(ctx, store.AutomationLogParams{
		RunID:   run.ID,
		Message: fmt.Sprintf("ATS adapter plan prepared: %s.", plan.Adapter),
		Details: map[string]any{
			"adapter":         plan.Adapter,
			"application_url": plan.ApplicationURL,
			"final_url":       plan.FinalURL,
			"steps":           plan.Steps,
			"materials":       plan.Materials,
			"review_only":     plan.ReviewOnly,
		},
	}); err != nil {
		return false, err
	}

	reviewRun, err := s.store.MarkAutomationReviewRequired(ctx, run.ID, plan.FinalURL)
	if err != nil {
		return false, err
	}
	if err := s.publishStatus(ctx, envelope, events.SubjectAutomationRunReviewRequired, events.EventAutomationRunReviewRequired, reviewRun, "User review required before final submission."); err != nil {
		return false, err
	}
	s.logger.Info("automation packet prepared", "run_id", run.ID, "application_id", run.ApplicationID, "adapter", plan.Adapter, "final_url", plan.FinalURL)
	return true, nil
}

func (s *Service) publishStatus(ctx context.Context, request events.Envelope[events.AutomationRunRequestedPayload], subject, eventType string, run store.AutomationRun, message string) error {
	payload := events.AutomationRunStatusPayload{
		AutomationRunID: run.ID,
		ApplicationID:   run.ApplicationID,
		JobID:           run.JobID,
		Status:          run.Status,
		Message:         message,
		OccurredAt:      time.Now().UTC(),
	}
	envelope := events.NewAutomationRunStatus(eventType, request.Source, request.CorrelationID, payload)
	return s.publish(ctx, subject, envelope)
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
