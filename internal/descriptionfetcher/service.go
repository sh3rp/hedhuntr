package descriptionfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.DescriptionFetcher
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
	client *http.Client
}

type RunResult struct {
	MessagesProcessed   int
	DescriptionsFetched int
}

func NewService(ctx context.Context, cfg config.DescriptionFetcher, logger *slog.Logger) (*Service, error) {
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
		events.SubjectJobsDescriptionFetchRequested,
		cfg.Consumer.Durable,
		nats.BindStream(cfg.NATS.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectJobsDescriptionFetchRequested, err)
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
		nc:     nc,
		js:     js,
		sub:    sub,
		store:  st,
		client: &http.Client{Timeout: cfg.HTTPClient.Timeout},
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
				s.logger.Error("process description fetch request", "error", err, "subject", msg.Subject)
				msg.Nak()
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}
			result.MessagesProcessed++
			result.DescriptionsFetched++
		}
	}

	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) error {
	var envelope events.Envelope[events.JobDescriptionFetchRequestedPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventJobDescriptionFetchRequested {
		return fmt.Errorf("unexpected event type %q", envelope.EventType)
	}

	job, err := s.store.GetJobForDescriptionFetch(ctx, envelope.Payload.JobID)
	if err != nil {
		return err
	}

	fetchURL := chooseFetchURL(envelope.Payload.ApplicationURL, envelope.Payload.SourceURL, job.ApplicationURL, job.SourceURL)
	if fetchURL == "" {
		return fmt.Errorf("job %d has no URL to fetch", envelope.Payload.JobID)
	}

	rawHTML, rawText, err := s.fetchDescription(ctx, fetchURL)
	if err != nil {
		return err
	}
	fetchedAt := time.Now().UTC()

	storeHTML := ""
	if s.cfg.HTTPClient.StoreRawHTML {
		storeHTML = rawHTML
	}
	if err := s.store.UpdateFetchedDescription(ctx, store.UpdateFetchedDescriptionParams{
		JobID:     envelope.Payload.JobID,
		RawText:   rawText,
		RawHTML:   storeHTML,
		FetchedAt: fetchedAt,
	}); err != nil {
		return err
	}

	fetched := events.NewJobDescriptionFetched(envelope.Source, envelope.CorrelationID, events.JobDescriptionFetchedPayload{
		JobID:          envelope.Payload.JobID,
		Source:         envelope.Payload.Source,
		SourceURL:      envelope.Payload.SourceURL,
		ApplicationURL: envelope.Payload.ApplicationURL,
		FetchedURL:     fetchURL,
		RawText:        rawText,
		RawHTML:        storeHTML,
		FetchedAt:      fetchedAt,
	})
	if err := s.publish(ctx, events.SubjectJobsDescriptionFetched, fetched); err != nil {
		return fmt.Errorf("publish description fetched: %w", err)
	}

	s.logger.Info("fetched job description", "job_id", envelope.Payload.JobID, "url", fetchURL, "text_bytes", len(rawText))
	return nil
}

func (s *Service) fetchDescription(ctx context.Context, fetchURL string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.8")
	req.Header.Set("User-Agent", s.cfg.HTTPClient.UserAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("fetch %s returned status %d: %s", fetchURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, s.cfg.HTTPClient.MaxBodyBytes))
	if err != nil {
		return "", "", err
	}
	raw := string(body)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/plain") {
		return "", strings.TrimSpace(raw), nil
	}
	return raw, TextFromHTML(raw), nil
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

func chooseFetchURL(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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
