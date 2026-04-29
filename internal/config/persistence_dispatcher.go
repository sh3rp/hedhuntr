package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type PersistenceDispatcher struct {
	NATS     NATSConfig     `json:"nats"`
	SQLite   SQLiteConfig   `json:"sqlite"`
	Consumer ConsumerConfig `json:"consumer"`
}

type SQLiteConfig struct {
	Path string `json:"path"`
}

type ConsumerConfig struct {
	Durable      string        `json:"durable"`
	BatchSize    int           `json:"batch_size"`
	FetchWait    time.Duration `json:"-"`
	FetchWaitRaw string        `json:"fetch_wait"`
	MaxInFlight  int           `json:"max_in_flight"`
}

func LoadPersistenceDispatcher(path string) (PersistenceDispatcher, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return PersistenceDispatcher{}, err
	}

	var cfg PersistenceDispatcher
	if err := json.Unmarshal(content, &cfg); err != nil {
		return PersistenceDispatcher{}, err
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
			return PersistenceDispatcher{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}

	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}

	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "persistence-dispatcher"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 10
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return PersistenceDispatcher{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 128
	}

	return cfg, nil
}
