package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ParserWorker struct {
	NATS     NATSConfig     `json:"nats"`
	SQLite   SQLiteConfig   `json:"sqlite"`
	Consumer ConsumerConfig `json:"consumer"`
	Parser   ParserConfig   `json:"parser"`
}

type ParserConfig struct {
	ExtraSkills []string `json:"extra_skills"`
}

func LoadParserWorker(path string) (ParserWorker, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ParserWorker{}, err
	}

	var cfg ParserWorker
	if err := json.Unmarshal(content, &cfg); err != nil {
		return ParserWorker{}, err
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
			return ParserWorker{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}

	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "parser-worker"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 10
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return ParserWorker{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 128
	}

	return cfg, nil
}
