package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type SourceProducer struct {
	NATS    NATSConfig     `json:"nats"`
	Sources []SourceConfig `json:"sources"`
}

type NATSConfig struct {
	URL               string        `json:"url"`
	Stream            string        `json:"stream"`
	PublishTimeout    time.Duration `json:"-"`
	PublishTimeoutRaw string        `json:"publish_timeout"`
}

type SourceConfig struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Enabled  bool            `json:"enabled"`
	Settings json.RawMessage `json:"settings"`
}

func LoadSourceProducer(path string) (SourceProducer, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return SourceProducer{}, err
	}

	var cfg SourceProducer
	if err := json.Unmarshal(content, &cfg); err != nil {
		return SourceProducer{}, err
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
			return SourceProducer{}, fmt.Errorf("parse nats.publish_timeout: %w", err)
		}
		cfg.NATS.PublishTimeout = timeout
	}

	if len(cfg.Sources) == 0 {
		return SourceProducer{}, fmt.Errorf("at least one source is required")
	}

	for i, source := range cfg.Sources {
		if source.Name == "" {
			return SourceProducer{}, fmt.Errorf("sources[%d].name is required", i)
		}
		if source.Type == "" {
			return SourceProducer{}, fmt.Errorf("sources[%d].type is required", i)
		}
	}

	return cfg, nil
}
