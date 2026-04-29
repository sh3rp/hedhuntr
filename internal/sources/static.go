package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

type staticSource struct {
	name string
	jobs []staticJob
}

type staticSettings struct {
	Jobs []staticJob `json:"jobs"`
}

type staticJob struct {
	ExternalID     string   `json:"external_id"`
	Title          string   `json:"title"`
	Company        string   `json:"company"`
	Location       string   `json:"location"`
	RemotePolicy   string   `json:"remote_policy"`
	EmploymentType string   `json:"employment_type"`
	SourceURL      string   `json:"source_url"`
	ApplicationURL string   `json:"application_url"`
	Description    string   `json:"description"`
	DetectedSkills []string `json:"detected_skills"`
}

func newStatic(cfg config.SourceConfig) (Source, error) {
	settings, err := decodeSettings[staticSettings](cfg)
	if err != nil {
		return nil, err
	}
	if len(settings.Jobs) == 0 {
		return nil, fmt.Errorf("source %q requires at least one static job", cfg.Name)
	}

	for i, job := range settings.Jobs {
		if strings.TrimSpace(job.Title) == "" {
			return nil, fmt.Errorf("source %q jobs[%d].title is required", cfg.Name, i)
		}
		if strings.TrimSpace(job.Company) == "" {
			return nil, fmt.Errorf("source %q jobs[%d].company is required", cfg.Name, i)
		}
		if strings.TrimSpace(job.SourceURL) == "" {
			return nil, fmt.Errorf("source %q jobs[%d].source_url is required", cfg.Name, i)
		}
	}

	return &staticSource{name: cfg.Name, jobs: settings.Jobs}, nil
}

func (s *staticSource) Name() string {
	return s.name
}

func (s *staticSource) Type() string {
	return "static"
}

func (s *staticSource) Fetch(context.Context) ([]events.JobDiscoveredPayload, error) {
	now := time.Now().UTC()
	jobs := make([]events.JobDiscoveredPayload, 0, len(s.jobs))

	for _, job := range s.jobs {
		raw, _ := json.Marshal(job)
		jobs = append(jobs, events.JobDiscoveredPayload{
			Source:         s.name,
			ExternalID:     strings.TrimSpace(job.ExternalID),
			Title:          strings.TrimSpace(job.Title),
			Company:        strings.TrimSpace(job.Company),
			Location:       strings.TrimSpace(job.Location),
			RemotePolicy:   strings.TrimSpace(job.RemotePolicy),
			EmploymentType: strings.TrimSpace(job.EmploymentType),
			SourceURL:      strings.TrimSpace(job.SourceURL),
			ApplicationURL: strings.TrimSpace(job.ApplicationURL),
			Description:    strings.TrimSpace(job.Description),
			DetectedSkills: job.DetectedSkills,
			DiscoveredAt:   now,
			Raw:            raw,
		})
	}

	return jobs, nil
}
