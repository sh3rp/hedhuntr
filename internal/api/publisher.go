package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

type EventPublisher interface {
	Publish(ctx context.Context, subject string, envelope any) error
}

type JetStreamPublisher struct {
	cfg    config.API
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
}

func NewJetStreamPublisher(cfg config.API, logger *slog.Logger) *JetStreamPublisher {
	if logger == nil {
		logger = slog.Default()
	}
	return &JetStreamPublisher{cfg: cfg, logger: logger}
}

func (p *JetStreamPublisher) Start(ctx context.Context) error {
	nc, err := nats.Connect(p.cfg.NATS.URL, nats.Name("hedhuntr-api-publisher"))
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return fmt.Errorf("initialize jetstream: %w", err)
	}
	if err := broker.EnsureJobsStream(js, p.cfg.NATS.Stream); err != nil {
		nc.Close()
		return err
	}
	p.nc = nc
	p.js = js
	go func() {
		<-ctx.Done()
		p.Close()
	}()
	p.logger.Info("api event publisher started", "nats_url", p.cfg.NATS.URL, "stream", p.cfg.NATS.Stream)
	return nil
}

func (p *JetStreamPublisher) Close() {
	if p.nc != nil {
		p.nc.Close()
		p.nc = nil
	}
}

func (p *JetStreamPublisher) Publish(ctx context.Context, subject string, envelope any) error {
	if p.js == nil {
		return fmt.Errorf("api event publisher is not started")
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal %s envelope: %w", subject, err)
	}
	publishCtx := ctx
	cancel := func() {}
	if p.cfg.NATS.PublishTimeout > 0 {
		publishCtx, cancel = context.WithTimeout(ctx, p.cfg.NATS.PublishTimeout)
	}
	defer cancel()
	_, err = p.js.Publish(subject, data, nats.Context(publishCtx), nats.MsgId(idempotencyKeyOf(envelope)))
	if err != nil {
		return fmt.Errorf("publish %s: %w", subject, err)
	}
	return nil
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
