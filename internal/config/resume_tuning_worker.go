package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ResumeTuningWorker struct {
	NATS      NATSConfig             `json:"nats"`
	SQLite    SQLiteConfig           `json:"sqlite"`
	Consumer  ConsumerConfig         `json:"consumer"`
	Documents DocumentStorageConfig  `json:"documents"`
	Tuning    ResumeTuningWorkerSpec `json:"tuning"`
}

type DocumentStorageConfig struct {
	RootDir string `json:"root_dir"`
}

type ResumeTuningWorkerSpec struct {
	ResumeSourceID int64 `json:"resume_source_id"`
	MaxHighlights  int   `json:"max_highlights"`
}

func LoadResumeTuningWorker(path string) (ResumeTuningWorker, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ResumeTuningWorker{}, err
	}

	var cfg ResumeTuningWorker
	if err := json.Unmarshal(content, &cfg); err != nil {
		return ResumeTuningWorker{}, err
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
			return ResumeTuningWorker{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "resume-tuning-worker"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 5
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return ResumeTuningWorker{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 64
	}
	if cfg.Documents.RootDir == "" {
		cfg.Documents.RootDir = "data/documents"
	}
	if cfg.Tuning.MaxHighlights <= 0 {
		cfg.Tuning.MaxHighlights = 6
	}

	return cfg, nil
}
