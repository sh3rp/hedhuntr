package sources

import (
	"context"
	"encoding/json"
	"fmt"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

type Source interface {
	Name() string
	Type() string
	Fetch(context.Context) ([]events.JobDiscoveredPayload, error)
}

func Build(configs []config.SourceConfig) ([]Source, error) {
	var built []Source

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		switch cfg.Type {
		case "greenhouse":
			source, err := newGreenhouse(cfg)
			if err != nil {
				return nil, err
			}
			built = append(built, source)
		case "static":
			source, err := newStatic(cfg)
			if err != nil {
				return nil, err
			}
			built = append(built, source)
		default:
			return nil, fmt.Errorf("unsupported source type %q for source %q", cfg.Type, cfg.Name)
		}
	}

	if len(built) == 0 {
		return nil, fmt.Errorf("no enabled sources configured")
	}

	return built, nil
}

func decodeSettings[T any](cfg config.SourceConfig) (T, error) {
	var settings T
	if len(cfg.Settings) == 0 {
		return settings, fmt.Errorf("source %q settings are required", cfg.Name)
	}
	if err := json.Unmarshal(cfg.Settings, &settings); err != nil {
		return settings, fmt.Errorf("decode settings for source %q: %w", cfg.Name, err)
	}
	return settings, nil
}
