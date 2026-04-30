package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type AutomationWorker struct {
	NATS       NATSConfig       `json:"nats"`
	SQLite     SQLiteConfig     `json:"sqlite"`
	Consumer   ConsumerConfig   `json:"consumer"`
	Automation AutomationConfig `json:"automation"`
}

type AutomationConfig struct {
	Mode            string   `json:"mode"`
	AllowedAdapters []string `json:"allowed_adapters"`
}

func LoadAutomationWorker(path string) (AutomationWorker, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return AutomationWorker{}, err
	}
	var cfg AutomationWorker
	if err := json.Unmarshal(content, &cfg); err != nil {
		return AutomationWorker{}, err
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
			return AutomationWorker{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "automation-worker"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 5
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return AutomationWorker{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Automation.Mode == "" {
		cfg.Automation.Mode = "packet-only"
	}
	if len(cfg.Automation.AllowedAdapters) == 0 {
		cfg.Automation.AllowedAdapters = []string{"greenhouse", "lever", "ashby", "workday", "generic"}
	}
	return cfg, nil
}
