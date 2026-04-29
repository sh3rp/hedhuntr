package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type NotificationWorker struct {
	NATS          NATSConfig         `json:"nats"`
	SQLite        SQLiteConfig       `json:"sqlite"`
	Consumer      ConsumerConfig     `json:"consumer"`
	Notifications NotificationConfig `json:"notifications"`
}

type NotificationConfig struct {
	MinScore                int                         `json:"min_score"`
	NotifyJobsMatched       bool                        `json:"notify_jobs_matched"`
	NotifyApplicationsReady bool                        `json:"notify_applications_ready"`
	Channels                []NotificationChannelConfig `json:"channels"`
}

type NotificationChannelConfig struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

func LoadNotificationWorker(path string) (NotificationWorker, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return NotificationWorker{}, err
	}

	var cfg NotificationWorker
	if err := json.Unmarshal(content, &cfg); err != nil {
		return NotificationWorker{}, err
	}

	if cfg.NATS.URL == "" {
		cfg.NATS.URL = "nats://127.0.0.1:4222"
	}
	if cfg.NATS.Stream == "" {
		cfg.NATS.Stream = "JOBS"
	}
	if cfg.NATS.PublishTimeoutRaw == "" {
		cfg.NATS.PublishTimeout = 5 * time.Second
	} else {
		timeout, err := time.ParseDuration(cfg.NATS.PublishTimeoutRaw)
		if err != nil {
			return NotificationWorker{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "notification-worker"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 10
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return NotificationWorker{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 128
	}
	if cfg.Notifications.MinScore <= 0 {
		cfg.Notifications.MinScore = 70
	}
	if !cfg.Notifications.NotifyJobsMatched && !cfg.Notifications.NotifyApplicationsReady {
		cfg.Notifications.NotifyJobsMatched = true
		cfg.Notifications.NotifyApplicationsReady = true
	}
	for i, channel := range cfg.Notifications.Channels {
		if channel.Name == "" {
			return NotificationWorker{}, fmt.Errorf("notifications.channels[%d].name is required", i)
		}
		if channel.Type == "" {
			return NotificationWorker{}, fmt.Errorf("notifications.channels[%d].type is required", i)
		}
	}

	return cfg, nil
}
