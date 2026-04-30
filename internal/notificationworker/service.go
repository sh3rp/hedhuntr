package notificationworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/notification"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg      config.NotificationWorker
	logger   *slog.Logger
	nc       *nats.Conn
	js       nats.JetStreamContext
	subs     []*nats.Subscription
	store    *store.Store
	sender   notification.Sender
	channels []notification.Channel
}

type RunResult struct {
	MessagesProcessed   int
	NotificationsSent   int
	NotificationsFailed int
}

func NewService(ctx context.Context, cfg config.NotificationWorker, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	st, err := store.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	channels := notification.ChannelsFromConfig(cfg.Notifications.Channels)
	for _, channel := range channels {
		if err := st.UpsertNotificationChannel(ctx, store.UpsertNotificationChannelParams{
			Name:       channel.Name,
			Type:       channel.Type,
			Enabled:    channel.Enabled,
			WebhookURL: channel.WebhookURL,
		}); err != nil {
			st.Close()
			return nil, err
		}
	}
	if err := syncRules(ctx, st, cfg.Notifications); err != nil {
		st.Close()
		return nil, err
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

	subjects := []string{events.SubjectJobsMatched, events.SubjectApplicationsReady}
	subs := make([]*nats.Subscription, 0, len(subjects))
	for _, subject := range subjects {
		sub, err := js.PullSubscribe(
			subject,
			cfg.Consumer.Durable+"-"+durableSuffix(subject),
			nats.BindStream(cfg.NATS.Stream),
			nats.ManualAck(),
		)
		if err != nil {
			st.Close()
			nc.Close()
			return nil, fmt.Errorf("subscribe to %s: %w", subject, err)
		}
		subs = append(subs, sub)
	}

	return &Service{
		cfg:      cfg,
		logger:   logger,
		nc:       nc,
		js:       js,
		subs:     subs,
		store:    st,
		sender:   notification.NewSender(cfg.NATS.PublishTimeout),
		channels: channels,
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

		progress := false
		for _, sub := range s.subs {
			if maxMessages > 0 && result.MessagesProcessed >= maxMessages {
				break
			}
			remaining := s.cfg.Consumer.BatchSize
			if maxMessages > 0 {
				left := maxMessages - result.MessagesProcessed
				if left < remaining {
					remaining = left
				}
			}
			messages, err := sub.Fetch(remaining, nats.Context(ctx), nats.MaxWait(s.cfg.Consumer.FetchWait))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				return result, fmt.Errorf("fetch messages: %w", err)
			}
			progress = true

			for _, msg := range messages {
				sent, failed, err := s.processMessage(ctx, msg)
				if err != nil {
					s.logger.Error("process notification event", "error", err, "subject", msg.Subject)
					msg.Nak()
					continue
				}
				if err := msg.Ack(); err != nil {
					return result, fmt.Errorf("ack message: %w", err)
				}
				result.MessagesProcessed++
				result.NotificationsSent += sent
				result.NotificationsFailed += failed
			}
		}
		if !progress && maxMessages > 0 {
			return result, nil
		}
	}

	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) (int, int, error) {
	message, score, eventID, err := s.messageFromEvent(msg)
	if err != nil {
		return 0, 0, err
	}
	channels, shouldNotify, err := s.notificationSettings(ctx, msg.Subject, score)
	if err != nil {
		return 0, 0, err
	}
	if !shouldNotify {
		return 0, 0, nil
	}

	var sent int
	var failed int
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		result := s.sender.Send(ctx, channel, message)
		status := "sent"
		if result.Error != "" {
			status = "failed"
			failed++
		} else {
			sent++
		}
		if err := s.store.RecordNotificationDelivery(ctx, store.RecordNotificationDeliveryParams{
			ChannelName:  channel.Name,
			ChannelType:  channel.Type,
			EventID:      eventID,
			EventSubject: msg.Subject,
			Status:       status,
			StatusCode:   result.StatusCode,
			Error:        result.Error,
			ResponseBody: result.ResponseBody,
		}); err != nil {
			return sent, failed, err
		}
	}
	return sent, failed, nil
}

func (s *Service) notificationSettings(ctx context.Context, subject string, score int) ([]notification.Channel, bool, error) {
	storeChannels, err := s.store.ListNotificationChannels(ctx)
	if err != nil {
		return nil, false, err
	}
	channels := make([]notification.Channel, 0, len(storeChannels))
	for _, channel := range storeChannels {
		channels = append(channels, notification.Channel{
			Name:       channel.Name,
			Type:       channel.Type,
			Enabled:    channel.Enabled,
			WebhookURL: channel.WebhookURL,
		})
	}
	rules, err := s.store.ListNotificationRules(ctx)
	if err != nil {
		return nil, false, err
	}
	for _, rule := range rules {
		if rule.EventSubject != subject || !rule.Enabled {
			continue
		}
		if rule.MinScore != nil && score < *rule.MinScore {
			return channels, false, nil
		}
		return channels, true, nil
	}
	return channels, false, nil
}

func (s *Service) messageFromEvent(msg *nats.Msg) (string, int, string, error) {
	switch msg.Subject {
	case events.SubjectJobsMatched:
		var envelope events.Envelope[events.JobMatchedPayload]
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			return "", 0, "", err
		}
		if envelope.EventType != events.EventJobMatched {
			return "", 0, "", fmt.Errorf("unexpected event type %q", envelope.EventType)
		}
		return notification.FormatJobMatched(envelope.Payload), envelope.Payload.Score, envelope.EventID, nil
	case events.SubjectApplicationsReady:
		var envelope events.Envelope[events.ApplicationReadyPayload]
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			return "", 0, "", err
		}
		if envelope.EventType != events.EventApplicationReady {
			return "", 0, "", fmt.Errorf("unexpected event type %q", envelope.EventType)
		}
		return notification.FormatApplicationReady(envelope.Payload), envelope.Payload.MatchScore, envelope.EventID, nil
	default:
		return "", 0, "", fmt.Errorf("unsupported notification subject %q", msg.Subject)
	}
}

func syncRules(ctx context.Context, st *store.Store, cfg config.NotificationConfig) error {
	minScore := cfg.MinScore
	if err := st.UpsertNotificationRule(ctx, store.UpsertNotificationRuleParams{
		Name:         "jobs-matched",
		EventSubject: events.SubjectJobsMatched,
		Enabled:      cfg.NotifyJobsMatched,
		MinScore:     &minScore,
	}); err != nil {
		return err
	}
	return st.UpsertNotificationRule(ctx, store.UpsertNotificationRuleParams{
		Name:         "applications-ready",
		EventSubject: events.SubjectApplicationsReady,
		Enabled:      cfg.NotifyApplicationsReady,
	})
}

func durableSuffix(subject string) string {
	switch subject {
	case events.SubjectJobsMatched:
		return "jobs-matched"
	case events.SubjectApplicationsReady:
		return "applications-ready"
	default:
		return "events"
	}
}
