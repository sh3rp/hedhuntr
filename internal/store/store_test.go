package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"hedhuntr/internal/events"
)

func TestSaveDiscoveredJobCreatesAndUpdatesJob(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	discoveredAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	payload := events.JobDiscoveredPayload{
		Source:         "static-test",
		ExternalID:     "job-1",
		Title:          "Backend Engineer",
		Company:        "ExampleCo",
		Location:       "Remote",
		SourceURL:      "https://example.com/jobs/1?utm_source=test",
		ApplicationURL: "https://example.com/apply/1",
		Description:    "Build Go services.",
		DetectedSkills: []string{"Go", "NATS"},
		DiscoveredAt:   discoveredAt,
	}
	envelope := events.NewJobDiscovered("static-test", payload)
	raw, err := json.Marshal(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, envelope, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true")
	}
	if result.JobID == 0 {
		t.Fatal("JobID = 0, want non-zero")
	}

	payload.Title = "Senior Backend Engineer"
	second := events.NewJobDiscovered("static-test", payload)
	second.IdempotencyKey = envelope.IdempotencyKey
	second.CorrelationID = envelope.CorrelationID

	result, err = store.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Created {
		t.Fatal("Created = true, want false for duplicate")
	}
}
