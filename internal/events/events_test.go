package events

import "testing"

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
