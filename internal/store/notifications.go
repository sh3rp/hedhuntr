package store

import (
	"context"
	"fmt"
)

type UpsertNotificationChannelParams struct {
	Name       string
	Type       string
	Enabled    bool
	WebhookURL string
}

type UpsertNotificationRuleParams struct {
	Name         string
	EventSubject string
	Enabled      bool
	MinScore     *int
}

type RecordNotificationDeliveryParams struct {
	ChannelName  string
	ChannelType  string
	EventID      string
	EventSubject string
	Status       string
	StatusCode   int
	Error        string
	ResponseBody string
}

func (s *Store) UpsertNotificationChannel(ctx context.Context, params UpsertNotificationChannelParams) error {
	enabled := 0
	if params.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO notification_channels(name, type, enabled, webhook_url)
VALUES(?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
	type = excluded.type,
	enabled = excluded.enabled,
	webhook_url = excluded.webhook_url,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.Name,
		params.Type,
		enabled,
		nullIfEmpty(params.WebhookURL),
	)
	if err != nil {
		return fmt.Errorf("upsert notification channel: %w", err)
	}
	return nil
}

func (s *Store) UpsertNotificationRule(ctx context.Context, params UpsertNotificationRuleParams) error {
	enabled := 0
	if params.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO notification_rules(name, event_subject, enabled, min_score)
VALUES(?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
	event_subject = excluded.event_subject,
	enabled = excluded.enabled,
	min_score = excluded.min_score,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.Name,
		params.EventSubject,
		enabled,
		intPtrOrNil(params.MinScore),
	)
	if err != nil {
		return fmt.Errorf("upsert notification rule: %w", err)
	}
	return nil
}

func (s *Store) RecordNotificationDelivery(ctx context.Context, params RecordNotificationDeliveryParams) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO notification_deliveries(
	channel_name,
	channel_type,
	event_id,
	event_subject,
	status,
	status_code,
	error,
	response_body
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		params.ChannelName,
		params.ChannelType,
		params.EventID,
		params.EventSubject,
		params.Status,
		params.StatusCode,
		nullIfEmpty(params.Error),
		nullIfEmpty(params.ResponseBody),
	)
	if err != nil {
		return fmt.Errorf("record notification delivery: %w", err)
	}
	return nil
}

func (s *Store) CountNotificationDeliveries(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM notification_deliveries").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
