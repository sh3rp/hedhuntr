package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	SubjectJobsDiscovered = "jobs.discovered"
	EventJobDiscovered    = "JobDiscovered"
)

type Envelope[T any] struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	EventVersion   int       `json:"event_version"`
	OccurredAt     time.Time `json:"occurred_at"`
	Source         string    `json:"source"`
	CorrelationID  string    `json:"correlation_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Payload        T         `json:"payload"`
}

type JobDiscoveredPayload struct {
	Source         string          `json:"source"`
	ExternalID     string          `json:"external_id"`
	Title          string          `json:"title"`
	Company        string          `json:"company"`
	Location       string          `json:"location,omitempty"`
	RemotePolicy   string          `json:"remote_policy,omitempty"`
	EmploymentType string          `json:"employment_type,omitempty"`
	SourceURL      string          `json:"source_url"`
	ApplicationURL string          `json:"application_url,omitempty"`
	Description    string          `json:"description,omitempty"`
	DetectedSkills []string        `json:"detected_skills,omitempty"`
	PublishedAt    *time.Time      `json:"published_at,omitempty"`
	DiscoveredAt   time.Time       `json:"discovered_at"`
	Raw            json.RawMessage `json:"raw,omitempty"`
}

func NewJobDiscovered(sourceName string, payload JobDiscoveredPayload) Envelope[JobDiscoveredPayload] {
	now := time.Now().UTC()
	idempotencyKey := JobIdempotencyKey(payload)
	return Envelope[JobDiscoveredPayload]{
		EventID:        StableID("event", sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobDiscovered,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  StableID("correlation", sourceName, idempotencyKey),
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func JobIdempotencyKey(job JobDiscoveredPayload) string {
	if job.ExternalID != "" {
		return fmt.Sprintf("%s:%s", normalize(job.Source), normalize(job.ExternalID))
	}
	if job.ApplicationURL != "" {
		return fmt.Sprintf("%s:%s", normalize(job.Source), normalize(job.ApplicationURL))
	}
	return fmt.Sprintf(
		"%s:%s",
		normalize(job.Source),
		StableID("job", job.Company, job.Title, job.Location, job.SourceURL),
	)
}

func StableID(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(strings.ToLower(part))))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
