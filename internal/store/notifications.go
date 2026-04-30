package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type NotificationChannel struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type NotificationRule struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	EventSubject string `json:"eventSubject"`
	Enabled      bool   `json:"enabled"`
	MinScore     *int   `json:"minScore,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type UpsertNotificationChannelParams struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhookUrl"`
}

type UpsertNotificationRuleParams struct {
	Name         string `json:"name"`
	EventSubject string `json:"eventSubject"`
	Enabled      bool   `json:"enabled"`
	MinScore     *int   `json:"minScore"`
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
	if strings.TrimSpace(params.Name) == "" {
		return fmt.Errorf("notification channel name is required")
	}
	params.Type = strings.ToLower(strings.TrimSpace(params.Type))
	switch params.Type {
	case "discord", "slack":
	default:
		return fmt.Errorf("unsupported notification channel type %q", params.Type)
	}
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
	if strings.TrimSpace(params.Name) == "" {
		return fmt.Errorf("notification rule name is required")
	}
	if strings.TrimSpace(params.EventSubject) == "" {
		return fmt.Errorf("notification rule event_subject is required")
	}
	if params.MinScore != nil && (*params.MinScore < 0 || *params.MinScore > 100) {
		return fmt.Errorf("notification rule min_score must be between 0 and 100")
	}
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

func (s *Store) ListNotificationChannels(ctx context.Context) ([]NotificationChannel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, type, enabled, COALESCE(webhook_url, ''), created_at, updated_at
FROM notification_channels
ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query notification channels: %w", err)
	}
	defer rows.Close()
	channels := []NotificationChannel{}
	for rows.Next() {
		var channel NotificationChannel
		var enabled int
		if err := rows.Scan(&channel.ID, &channel.Name, &channel.Type, &enabled, &channel.WebhookURL, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
			return nil, err
		}
		channel.Enabled = enabled == 1
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (s *Store) ListNotificationRules(ctx context.Context) ([]NotificationRule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, event_subject, enabled, min_score, created_at, updated_at
FROM notification_rules
ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query notification rules: %w", err)
	}
	defer rows.Close()
	rules := []NotificationRule{}
	for rows.Next() {
		var rule NotificationRule
		var enabled int
		var minScore sql.NullInt64
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.EventSubject, &enabled, &minScore, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rule.Enabled = enabled == 1
		if minScore.Valid {
			value := int(minScore.Int64)
			rule.MinScore = &value
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
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
