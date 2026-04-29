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
	SubjectJobsDiscovered                 = "jobs.discovered"
	SubjectJobsSaved                      = "jobs.saved"
	SubjectJobsDescriptionFetchRequested  = "jobs.description.fetch.requested"
	SubjectJobsDescriptionFetched         = "jobs.description.fetched"
	SubjectJobsParsed                     = "jobs.parsed"
	SubjectJobsMatched                    = "jobs.matched"
	SubjectApplicationsReady              = "applications.ready"
	SubjectApplicationsMaterialsDrafted   = "applications.materials.drafted"
	SubjectApplicationsAutomationApproved = "applications.automation.approved"
	SubjectAutomationRunRequested         = "automation.run.requested"
	SubjectAutomationRunStarted           = "automation.run.started"
	SubjectAutomationRunReviewRequired    = "automation.run.review_required"
	SubjectAutomationRunFailed            = "automation.run.failed"

	EventJobDiscovered                 = "JobDiscovered"
	EventJobSaved                      = "JobSaved"
	EventJobDescriptionFetchRequested  = "JobDescriptionFetchRequested"
	EventJobDescriptionFetched         = "JobDescriptionFetched"
	EventJobParsed                     = "JobParsed"
	EventJobMatched                    = "JobMatched"
	EventApplicationReady              = "ApplicationReady"
	EventApplicationMaterialsDrafted   = "ApplicationMaterialsDrafted"
	EventApplicationAutomationApproved = "ApplicationAutomationApproved"
	EventAutomationRunRequested        = "AutomationRunRequested"
	EventAutomationRunStarted          = "AutomationRunStarted"
	EventAutomationRunReviewRequired   = "AutomationRunReviewRequired"
	EventAutomationRunFailed           = "AutomationRunFailed"
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

type JobMatchedPayload struct {
	JobID              int64     `json:"job_id"`
	CandidateProfileID int64     `json:"candidate_profile_id"`
	Score              int       `json:"score"`
	MatchedSkills      []string  `json:"matched_skills"`
	MissingSkills      []string  `json:"missing_skills"`
	Notes              []string  `json:"notes"`
	MatchedAt          time.Time `json:"matched_at"`
}

type ApplicationReadyPayload struct {
	JobID              int64     `json:"job_id"`
	CandidateProfileID int64     `json:"candidate_profile_id"`
	MatchScore         int       `json:"match_score"`
	ReadyAt            time.Time `json:"ready_at"`
}

type ApplicationMaterialsDraftedPayload struct {
	JobID              int64     `json:"job_id"`
	ApplicationID      int64     `json:"application_id"`
	CandidateProfileID int64     `json:"candidate_profile_id"`
	ResumeSourceID     int64     `json:"resume_source_id"`
	ResumeVersionID    int64     `json:"resume_version_id"`
	ResumeDocumentID   int64     `json:"resume_document_id"`
	CoverLetterID      int64     `json:"cover_letter_id"`
	CoverLetterDocID   int64     `json:"cover_letter_document_id"`
	Status             string    `json:"status"`
	DraftedAt          time.Time `json:"drafted_at"`
}

type ApplicationAutomationApprovedPayload struct {
	ApplicationID         int64     `json:"application_id"`
	JobID                 int64     `json:"job_id"`
	CandidateProfileID    int64     `json:"candidate_profile_id"`
	AutomationRunID       int64     `json:"automation_run_id"`
	ResumeMaterialID      int64     `json:"resume_material_id"`
	CoverLetterMaterialID *int64    `json:"cover_letter_material_id,omitempty"`
	ApprovedAt            time.Time `json:"approved_at"`
}

type AutomationRunRequestedPayload struct {
	AutomationRunID       int64     `json:"automation_run_id"`
	ApplicationID         int64     `json:"application_id"`
	JobID                 int64     `json:"job_id"`
	CandidateProfileID    int64     `json:"candidate_profile_id"`
	ResumeMaterialID      int64     `json:"resume_material_id"`
	CoverLetterMaterialID *int64    `json:"cover_letter_material_id,omitempty"`
	RequestedAt           time.Time `json:"requested_at"`
}

type AutomationRunStatusPayload struct {
	AutomationRunID int64     `json:"automation_run_id"`
	ApplicationID   int64     `json:"application_id"`
	JobID           int64     `json:"job_id"`
	Status          string    `json:"status"`
	Message         string    `json:"message,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
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

func NewJobMatched(sourceName, correlationID string, payload JobMatchedPayload) Envelope[JobMatchedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("job-matched", sourceName, fmt.Sprintf("%d", payload.JobID), fmt.Sprintf("%d", payload.CandidateProfileID))
	return Envelope[JobMatchedPayload]{
		EventID:        StableID("event", EventJobMatched, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventJobMatched,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewApplicationReady(sourceName, correlationID string, payload ApplicationReadyPayload) Envelope[ApplicationReadyPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("application-ready", sourceName, fmt.Sprintf("%d", payload.JobID), fmt.Sprintf("%d", payload.CandidateProfileID))
	return Envelope[ApplicationReadyPayload]{
		EventID:        StableID("event", EventApplicationReady, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventApplicationReady,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewApplicationMaterialsDrafted(sourceName, correlationID string, payload ApplicationMaterialsDraftedPayload) Envelope[ApplicationMaterialsDraftedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("application-materials-drafted", sourceName, fmt.Sprintf("%d", payload.JobID), fmt.Sprintf("%d", payload.CandidateProfileID), fmt.Sprintf("%d", payload.ResumeSourceID))
	return Envelope[ApplicationMaterialsDraftedPayload]{
		EventID:        StableID("event", EventApplicationMaterialsDrafted, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventApplicationMaterialsDrafted,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewApplicationAutomationApproved(sourceName, correlationID string, payload ApplicationAutomationApprovedPayload) Envelope[ApplicationAutomationApprovedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("application-automation-approved", sourceName, fmt.Sprintf("%d", payload.ApplicationID), fmt.Sprintf("%d", payload.AutomationRunID))
	return Envelope[ApplicationAutomationApprovedPayload]{
		EventID:        StableID("event", EventApplicationAutomationApproved, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventApplicationAutomationApproved,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewAutomationRunRequested(sourceName, correlationID string, payload AutomationRunRequestedPayload) Envelope[AutomationRunRequestedPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("automation-run-requested", sourceName, fmt.Sprintf("%d", payload.AutomationRunID))
	return Envelope[AutomationRunRequestedPayload]{
		EventID:        StableID("event", EventAutomationRunRequested, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      EventAutomationRunRequested,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         sourceName,
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
	}
}

func NewAutomationRunStatus(eventType, sourceName, correlationID string, payload AutomationRunStatusPayload) Envelope[AutomationRunStatusPayload] {
	now := time.Now().UTC()
	idempotencyKey := StableID("automation-run-status", eventType, sourceName, fmt.Sprintf("%d", payload.AutomationRunID), payload.Status)
	return Envelope[AutomationRunStatusPayload]{
		EventID:        StableID("event", eventType, sourceName, idempotencyKey, now.Format(time.RFC3339Nano)),
		EventType:      eventType,
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
	// Fallback to a strict hash of essential fields to avoid collision while minimizing "fuzziness"
	return fmt.Sprintf("%s:%s",
		normalize(job.Source),
		StableID("job-fallback", job.Company, job.Title, job.Location, job.SourceURL),
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
