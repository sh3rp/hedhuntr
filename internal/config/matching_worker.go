package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type MatchingWorker struct {
	NATS     NATSConfig     `json:"nats"`
	SQLite   SQLiteConfig   `json:"sqlite"`
	Consumer ConsumerConfig `json:"consumer"`
	Matching MatchingConfig `json:"matching"`
}

type MatchingConfig struct {
	CandidateProfileID int64 `json:"candidate_profile_id"`
	ReadyThreshold     int   `json:"ready_threshold"`
}

func LoadMatchingWorker(path string) (MatchingWorker, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return MatchingWorker{}, err
	}

	var cfg MatchingWorker
	if err := json.Unmarshal(content, &cfg); err != nil {
		return MatchingWorker{}, err
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
			return MatchingWorker{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "matching-worker"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 10
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return MatchingWorker{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 128
	}
	if cfg.Matching.ReadyThreshold <= 0 {
		cfg.Matching.ReadyThreshold = 70
	}

	return cfg, nil
}
