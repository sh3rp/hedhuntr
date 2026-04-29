package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Scheduler struct {
	SQLite               SQLiteConfig            `json:"sqlite"`
	SourceProducerConfig string                  `json:"source_producer_config"`
	PollInterval         time.Duration           `json:"-"`
	PollIntervalRaw      string                  `json:"poll_interval"`
	Sources              []ScheduledSourceConfig `json:"sources"`
}

type ScheduledSourceConfig struct {
	Name           string        `json:"name"`
	Enabled        bool          `json:"enabled"`
	Schedule       string        `json:"schedule"`
	Interval       string        `json:"interval"`
	Timeout        string        `json:"timeout"`
	IntervalParsed time.Duration `json:"-"`
	TimeoutParsed  time.Duration `json:"-"`
}

func LoadScheduler(path string) (Scheduler, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Scheduler{}, err
	}

	var cfg Scheduler
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Scheduler{}, err
	}

	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.SourceProducerConfig == "" {
		cfg.SourceProducerConfig = "configs/source-producer.example.json"
	}
	if cfg.PollIntervalRaw == "" {
		cfg.PollInterval = time.Minute
	} else {
		interval, err := time.ParseDuration(cfg.PollIntervalRaw)
		if err != nil {
			return Scheduler{}, fmt.Errorf("parse poll_interval: %w", err)
		}
		cfg.PollInterval = interval
	}

	if len(cfg.Sources) == 0 {
		return Scheduler{}, fmt.Errorf("at least one scheduled source is required")
	}

	for i := range cfg.Sources {
		source := &cfg.Sources[i]
		if source.Name == "" {
			return Scheduler{}, fmt.Errorf("sources[%d].name is required", i)
		}
		if source.Schedule == "" {
			source.Schedule = "hourly"
		}
		if source.Interval == "" {
			switch source.Schedule {
			case "hourly":
				source.Interval = time.Hour.String()
			case "daily":
				source.Interval = (24 * time.Hour).String()
			default:
				return Scheduler{}, fmt.Errorf("sources[%d].schedule %q is unsupported", i, source.Schedule)
			}
		}
		interval, err := time.ParseDuration(source.Interval)
		if err != nil {
			return Scheduler{}, fmt.Errorf("parse sources[%d].interval: %w", i, err)
		}
		if interval <= 0 {
			return Scheduler{}, fmt.Errorf("sources[%d].interval must be positive", i)
		}
		source.IntervalParsed = interval

		if source.Timeout == "" {
			source.Timeout = (2 * time.Minute).String()
		}
		timeout, err := time.ParseDuration(source.Timeout)
		if err != nil {
			return Scheduler{}, fmt.Errorf("parse sources[%d].timeout: %w", i, err)
		}
		if timeout <= 0 {
			return Scheduler{}, fmt.Errorf("sources[%d].timeout must be positive", i)
		}
		source.TimeoutParsed = timeout
	}

	return cfg, nil
}
