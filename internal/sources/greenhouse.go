package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

const greenhouseBaseURL = "https://boards-api.greenhouse.io/v1/boards"

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

type greenhouseSource struct {
	name           string
	boardToken     string
	company        string
	includeContent bool
	client         *http.Client
}

type greenhouseSettings struct {
	BoardToken     string `json:"board_token"`
	Company        string `json:"company"`
	IncludeContent bool   `json:"include_content"`
}

type greenhouseJobsResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	ID          int64                `json:"id"`
	Title       string               `json:"title"`
	AbsoluteURL string               `json:"absolute_url"`
	UpdatedAt   string               `json:"updated_at"`
	Content     string               `json:"content"`
	Location    greenhouseLocation   `json:"location"`
	Metadata    []greenhouseMetadata `json:"metadata"`
	Departments []greenhouseName     `json:"departments"`
	Offices     []greenhouseName     `json:"offices"`
}

type greenhouseLocation struct {
	Name string `json:"name"`
}

type greenhouseMetadata struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type greenhouseName struct {
	Name string `json:"name"`
}

func newGreenhouse(cfg config.SourceConfig) (Source, error) {
	settings, err := decodeSettings[greenhouseSettings](cfg)
	if err != nil {
		return nil, err
	}
	if settings.BoardToken == "" {
		return nil, fmt.Errorf("source %q requires settings.board_token", cfg.Name)
	}
	if settings.Company == "" {
		settings.Company = settings.BoardToken
	}

	return &greenhouseSource{
		name:           cfg.Name,
		boardToken:     settings.BoardToken,
		company:        settings.Company,
		includeContent: settings.IncludeContent,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}, nil
}

func (s *greenhouseSource) Name() string {
	return s.name
}

func (s *greenhouseSource) Type() string {
	return "greenhouse"
}

func (s *greenhouseSource) Fetch(ctx context.Context) ([]events.JobDiscoveredPayload, error) {
	endpoint, err := url.JoinPath(greenhouseBaseURL, s.boardToken, "jobs")
	if err != nil {
		return nil, err
	}

	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := reqURL.Query()
	if s.includeContent {
		query.Set("content", "true")
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "hedhuntr-source-producer/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("greenhouse returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed greenhouseJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode greenhouse response: %w", err)
	}

	jobs := make([]events.JobDiscoveredPayload, 0, len(parsed.Jobs))
	for _, job := range parsed.Jobs {
		raw, _ := json.Marshal(job)
		jobs = append(jobs, events.JobDiscoveredPayload{
			Source:         s.name,
			ExternalID:     fmt.Sprintf("%d", job.ID),
			Title:          strings.TrimSpace(job.Title),
			Company:        s.company,
			Location:       strings.TrimSpace(job.Location.Name),
			SourceURL:      strings.TrimSpace(job.AbsoluteURL),
			ApplicationURL: strings.TrimSpace(job.AbsoluteURL),
			Description:    textFromHTML(job.Content),
			PublishedAt:    parseGreenhouseTime(job.UpdatedAt),
			DiscoveredAt:   time.Now().UTC(),
			Raw:            raw,
		})
	}

	return jobs, nil
}

func parseGreenhouseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-07:00"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func textFromHTML(value string) string {
	if value == "" {
		return ""
	}
	noTags := htmlTagPattern.ReplaceAllString(value, " ")
	unescaped := html.UnescapeString(noTags)
	return strings.Join(strings.Fields(unescaped), " ")
}
