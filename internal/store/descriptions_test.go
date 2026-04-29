package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"hedhuntr/internal/events"
)

func TestUpdateFetchedDescription(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "description.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	payload := events.JobDiscoveredPayload{
		Source:         "static-test",
		ExternalID:     "job-1",
		Title:          "Backend Engineer",
		Company:        "ExampleCo",
		Location:       "Remote",
		SourceURL:      "https://example.com/jobs/1",
		ApplicationURL: "https://example.com/apply/1",
		DiscoveredAt:   time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
	}
	envelope := events.NewJobDiscovered("static-test", payload)
	raw, err := json.Marshal(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	result, err := st.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, envelope, raw)
	if err != nil {
		t.Fatal(err)
	}

	job, err := st.GetJobForDescriptionFetch(ctx, result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.ApplicationURL != payload.ApplicationURL {
		t.Fatalf("ApplicationURL = %q, want %q", job.ApplicationURL, payload.ApplicationURL)
	}

	if err := st.UpdateFetchedDescription(ctx, UpdateFetchedDescriptionParams{
		JobID:     result.JobID,
		RawText:   "Fetched job description",
		RawHTML:   "<p>Fetched job description</p>",
		FetchedAt: time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	rawText, status, err := st.GetJobDescriptionText(ctx, result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if rawText != "Fetched job description" {
		t.Fatalf("rawText = %q, want fetched text", rawText)
	}
	if status != "description_fetched" {
		t.Fatalf("status = %q, want description_fetched", status)
	}
}
