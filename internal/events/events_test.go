package events

import (
	"testing"
	"time"
)

func TestJobIdempotencyKeyPrefersExternalID(t *testing.T) {
	job := JobDiscoveredPayload{
		Source:         "Greenhouse",
		ExternalID:     " 123 ",
		ApplicationURL: "https://example.com/apply",
	}

	got := JobIdempotencyKey(job)
	want := "greenhouse:123"
	if got != want {
		t.Fatalf("JobIdempotencyKey() = %q, want %q", got, want)
	}
}

func TestJobIdempotencyKeyUsesApplicationURLWhenExternalIDMissing(t *testing.T) {
	job := JobDiscoveredPayload{
		Source:         "Lever",
		ApplicationURL: " https://example.com/apply ",
	}

	got := JobIdempotencyKey(job)
	want := "lever:https://example.com/apply"
	if got != want {
		t.Fatalf("JobIdempotencyKey() = %q, want %q", got, want)
	}
}

func TestNewJobDescriptionFetched(t *testing.T) {
	envelope := NewJobDescriptionFetched("source", "correlation", JobDescriptionFetchedPayload{
		JobID:      42,
		Source:     "source",
		SourceURL:  "https://example.com/jobs/42",
		FetchedURL: "https://example.com/jobs/42",
		RawText:    "Job description",
		FetchedAt:  envelopeTime(),
	})

	if envelope.EventType != EventJobDescriptionFetched {
		t.Fatalf("EventType = %q, want %q", envelope.EventType, EventJobDescriptionFetched)
	}
	if envelope.IdempotencyKey == "" {
		t.Fatal("IdempotencyKey is empty")
	}
	if envelope.Payload.JobID != 42 {
		t.Fatalf("Payload.JobID = %d, want 42", envelope.Payload.JobID)
	}
}

func TestNewJobParsed(t *testing.T) {
	envelope := NewJobParsed("source", "correlation", JobParsedPayload{
		JobID:        42,
		Source:       "source",
		Skills:       []string{"Go"},
		RemotePolicy: "remote",
		ParsedAt:     envelopeTime(),
	})

	if envelope.EventType != EventJobParsed {
		t.Fatalf("EventType = %q, want %q", envelope.EventType, EventJobParsed)
	}
	if envelope.IdempotencyKey == "" {
		t.Fatal("IdempotencyKey is empty")
	}
	if envelope.Payload.Skills[0] != "Go" {
		t.Fatalf("Payload.Skills = %#v, want Go", envelope.Payload.Skills)
	}
}

func envelopeTime() time.Time {
	return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
}
