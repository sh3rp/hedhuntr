package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/config"
)

type EventBridge struct {
	cfg    config.API
	hub    *Hub
	logger *slog.Logger
	nc     *nats.Conn
	subs   []*nats.Subscription
}

type eventEnvelope struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func NewEventBridge(cfg config.API, hub *Hub, logger *slog.Logger) *EventBridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBridge{cfg: cfg, hub: hub, logger: logger}
}

func (b *EventBridge) Start(ctx context.Context) error {
	if !b.cfg.Realtime.Enabled {
		b.logger.Info("api realtime event bridge disabled")
		return nil
	}
	nc, err := nats.Connect(b.cfg.NATS.URL, nats.Name("hedhuntr-api-realtime"))
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	b.nc = nc

	for _, subject := range b.cfg.Realtime.Subjects {
		subject := strings.TrimSpace(subject)
		if subject == "" {
			continue
		}
		sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
			b.handleMessage(msg)
		})
		if err != nil {
			b.Close()
			return fmt.Errorf("subscribe %s: %w", subject, err)
		}
		b.subs = append(b.subs, sub)
	}
	if err := nc.Flush(); err != nil {
		b.Close()
		return fmt.Errorf("flush subscriptions: %w", err)
	}

	go func() {
		<-ctx.Done()
		b.Close()
	}()
	b.logger.Info("api realtime event bridge started", "nats_url", b.cfg.NATS.URL, "subjects", b.cfg.Realtime.Subjects)
	return nil
}

func (b *EventBridge) Close() {
	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	b.subs = nil
	if b.nc != nil {
		b.nc.Close()
		b.nc = nil
	}
}

func (b *EventBridge) handleMessage(msg *nats.Msg) {
	message, err := WSMessageFromNATS(msg.Subject, msg.Data)
	if err != nil {
		b.logger.Warn("drop realtime event", "subject", msg.Subject, "error", err)
		return
	}
	b.hub.Broadcast(message)
}

func WSMessageFromNATS(subject string, data []byte) (WSMessage, error) {
	var envelope eventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return WSMessage{}, fmt.Errorf("decode event envelope: %w", err)
	}
	payload := map[string]any{}
	if len(envelope.Payload) > 0 && string(envelope.Payload) != "null" {
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return WSMessage{}, fmt.Errorf("decode event payload: %w", err)
		}
	}
	return WSMessage{
		Type:       "event",
		Topic:      topicForSubject(subject),
		EventID:    envelope.EventID,
		EventType:  envelope.EventType,
		OccurredAt: envelope.OccurredAt.UTC().Format(time.RFC3339Nano),
		Payload:    payload,
	}, nil
}

func topicForSubject(subject string) string {
	switch {
	case strings.HasPrefix(subject, "jobs."):
		return "jobs"
	case strings.HasPrefix(subject, "applications."):
		return "applications"
	case strings.HasPrefix(subject, "notifications."):
		return "notifications"
	case strings.HasPrefix(subject, "automation."):
		return "automation"
	case strings.HasPrefix(subject, "source_runs."):
		return "source_runs"
	default:
		return "system"
	}
}
