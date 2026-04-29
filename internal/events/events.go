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
	SubjectJobsDiscovered                = "jobs.discovered"
	SubjectJobsSaved                     = "jobs.saved"
	SubjectJobsDescriptionFetchRequested = "jobs.description.fetch.requested"
	SubjectJobsDescriptionFetched        = "jobs.description.fetched"
	SubjectJobsParsed                    = "jobs.parsed"

	EventJobDiscovered                = "JobDiscovered"
	EventJobSaved                     = "JobSaved"
	EventJobDescriptionFetchRequested = "JobDescriptionFetchRequested"
	EventJobDescriptionFetched        = "JobDescriptionFetched"
	EventJobParsed                    = "JobParsed"
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

type JobSavedPayload struct {
	JobID          int64     `json:"job_id"`
	Source         string    `json:"source"`
	ExternalID     string    `json:"external_id,omitempty"`
	Title          string    `json:"title"`
	Company        string    `json:"company"`
	SourceURL      string    `json:"source_url"`
	ApplicationURL string    `json:"application_url,omitempty"`
	Created        bool      `json:"created"`
	SavedAt        time.Time `json:"saved_at"`
}

type JobDescriptionFetchRequestedPayload struct {
	JobID          int64     `json:"job_id"`
	Source         string    `json:"source"`
	SourceURL      string    `json:"source_url"`
	ApplicationURL string    `json:"application_url,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
}

type JobDescriptionFetchedPayload struct {
	JobID          int64     `json:"job_id"`
	Source         string    `json:"source"`
	SourceURL      string    `json:"source_url"`
	ApplicationURL string    `json:"application_url,omitempty"`
	FetchedURL     string    `json:"fetched_url"`
	RawText        string    `json:"raw_text"`
	RawHTML        string    `json:"raw_html,omitempty"`
	FetchedAt      time.Time `json:"fetched_at"`
}

type JobParsedPayload struct {
	JobID            int64     `json:"job_id"`
	Source           string    `json:"source"`
	Skills           []string  `json:"skills"`
	Requirements     []string  `json:"requirements"`
	Responsibilities []string  `json:"responsibilities"`
	SalaryMin        *int      `json:"salary_min,omitempty"`
	SalaryMax        *int      `json:"salary_max,omitempty"`
	SalaryCurrency   string    `json:"salary_currency,omitempty"`
	SalaryPeriod     string    `json:"salary_period,omitempty"`
	RemotePolicy     string    `json:"remote_policy,omitempty"`
	Seniority        string    `json:"seniority,omitempty"`
	EmploymentType   string    `json:"employment_type,omitempty"`
	ParsedAt         time.Time `json:"parsed_at"`
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

func NewJobSaved(sourceName, correlationID string, payload JobSavedPayload) Envelope[JobSavedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("job-saved", sourceName, fmt.Sprintf("%d", payload.JobID), fmt.Sprintf("%t", payload.Created))
	return Envelope[JobSavedPayload]{
		EventID:        StableID("event", EventJobSaved, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobSaved,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewJobDescriptionFetchRequested(sourceName, correlationID string, payload JobDescriptionFetchRequestedPayload) Envelope[JobDescriptionFetchRequestedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("description-fetch-requested", sourceName, fmt.Sprintf("%d", payload.JobID))
	return Envelope[JobDescriptionFetchRequestedPayload]{
		EventID:        StableID("event", EventJobDescriptionFetchRequested, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobDescriptionFetchRequested,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewJobDescriptionFetched(sourceName, correlationID string, payload JobDescriptionFetchedPayload) Envelope[JobDescriptionFetchedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("description-fetched", sourceName, fmt.Sprintf("%d", payload.JobID), payload.FetchedURL)
	return Envelope[JobDescriptionFetchedPayload]{
		EventID:        StableID("event", EventJobDescriptionFetched, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobDescriptionFetched,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewJobParsed(sourceName, correlationID string, payload JobParsedPayload) Envelope[JobParsedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("job-parsed", sourceName, fmt.Sprintf("%d", payload.JobID))
	return Envelope[JobParsedPayload]{
		EventID:        StableID("event", EventJobParsed, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobParsed,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
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
