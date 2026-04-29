package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type API struct {
	HTTP      HTTPConfig      `json:"http"`
	NATS      NATSConfig      `json:"nats"`
	SQLite    SQLiteConfig    `json:"sqlite"`
	WebSocket WebSocketConfig `json:"websocket"`
	Realtime  RealtimeConfig  `json:"realtime"`
}

type HTTPConfig struct {
	Addr           string   `json:"addr"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type WebSocketConfig struct {
	PingInterval    time.Duration `json:"-"`
	PingIntervalRaw string        `json:"ping_interval"`
}

type RealtimeConfig struct {
	Enabled  bool     `json:"enabled"`
	Subjects []string `json:"subjects"`
}

func LoadAPI(path string) (API, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return API{}, err
	}

	var cfg API
	if err := json.Unmarshal(content, &cfg); err != nil {
		return API{}, err
	}
	if cfg.HTTP.Addr == "" {
		cfg.HTTP.Addr = "127.0.0.1:8080"
	}
	if cfg.NATS.URL == "" {
		cfg.NATS.URL = "nats://127.0.0.1:4222"
	}
	if cfg.NATS.Stream == "" {
		cfg.NATS.Stream = "JOBS"
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = "hedhuntr.db"
	}
	if cfg.WebSocket.PingIntervalRaw == "" {
		cfg.WebSocket.PingInterval = 25 * time.Second
	} else {
		interval, err := time.ParseDuration(cfg.WebSocket.PingIntervalRaw)
		if err != nil {
			return API{}, fmt.Errorf("parse websocket.ping_interval: %w", err)
		}
		cfg.WebSocket.PingInterval = interval
	}
	if cfg.WebSocket.PingInterval <= 0 {
		cfg.WebSocket.PingInterval = 25 * time.Second
	}
	if len(cfg.Realtime.Subjects) == 0 {
		cfg.Realtime.Subjects = []string{
			"jobs.saved",
			"jobs.description.fetched",
			"jobs.parsed",
			"jobs.matched",
			"applications.ready",
			"applications.materials.drafted",
			"applications.automation.approved",
			"automation.run.requested",
			"automation.run.started",
			"automation.run.review_required",
			"automation.run.failed",
			"notifications.>",
		}
	}

	return cfg, nil
}
