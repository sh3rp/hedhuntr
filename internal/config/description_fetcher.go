package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type DescriptionFetcher struct {
	NATS       NATSConfig       `json:"nats"`
	SQLite     SQLiteConfig     `json:"sqlite"`
	Consumer   ConsumerConfig   `json:"consumer"`
	HTTPClient HTTPClientConfig `json:"http_client"`
}

type HTTPClientConfig struct {
	Timeout      time.Duration `json:"-"`
	TimeoutRaw   string        `json:"timeout"`
	UserAgent    string        `json:"user_agent"`
	MaxBodyBytes int64         `json:"max_body_bytes"`
	StoreRawHTML bool          `json:"store_raw_html"`
}

func LoadDescriptionFetcher(path string) (DescriptionFetcher, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return DescriptionFetcher{}, err
	}

	var cfg DescriptionFetcher
	if err := json.Unmarshal(content, &cfg); err != nil {
		return DescriptionFetcher{}, err
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
			return DescriptionFetcher{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}

	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}

	if cfg.Consumer.Durable == "" {
		cfg.Consumer.Durable = "description-fetcher"
	}
	if cfg.Consumer.BatchSize <= 0 {
		cfg.Consumer.BatchSize = 10
	}
	if cfg.Consumer.FetchWaitRaw == "" {
		cfg.Consumer.FetchWait = 2 * time.Second
	} else {
		wait, err := time.ParseDuration(cfg.Consumer.FetchWaitRaw)
		if err != nil {
			return DescriptionFetcher{}, fmt.Errorf("parse consumer.fetch_wait: %w", err)
		}
		cfg.Consumer.FetchWait = wait
	}
	if cfg.Consumer.MaxInFlight <= 0 {
		cfg.Consumer.MaxInFlight = 128
	}

	if cfg.HTTPClient.TimeoutRaw == "" {
		cfg.HTTPClient.Timeout = 20 * time.Second
	} else {
		timeout, err := time.ParseDuration(cfg.HTTPClient.TimeoutRaw)
		if err != nil {
			return DescriptionFetcher{}, fmt.Errorf("parse http_client.timeout: %w", err)
		}
		cfg.HTTPClient.Timeout = timeout
	}
	if cfg.HTTPClient.UserAgent == "" {
		cfg.HTTPClient.UserAgent = "hedhuntr-description-fetcher/0.1"
	}
	if cfg.HTTPClient.MaxBodyBytes <= 0 {
		cfg.HTTPClient.MaxBodyBytes = 2_000_000
	}

	return cfg, nil
}
