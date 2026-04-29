package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hedhuntr/internal/events"
)

func TestSaveParsedJob(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "parsed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	payload := events.JobDiscoveredPayload{
		Source:       "static-test",
		ExternalID:   "job-1",
		Title:        "Senior Backend Engineer",
		Company:      "ExampleCo",
		Location:     "Remote",
		SourceURL:    "https://example.com/jobs/1",
		Description:  "Build services in Go. Salary $140k - $180k.",
		DiscoveredAt: time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
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

	min := 140000
	max := 180000
	if err := st.SaveParsedJob(ctx, SaveParsedJobParams{
		JobID:          result.JobID,
		Skills:         []string{"Go", "NATS"},
		Requirements:   []string{"5 years experience"},
		SalaryMin:      &min,
		SalaryMax:      &max,
		SalaryCurrency: "USD",
		SalaryPeriod:   "year",
		RemotePolicy:   "remote",
		Seniority:      "senior",
		EmploymentType: "full_time",
		ParsedAt:       time.Date(2026, 4, 28, 13, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := st.GetParsedJobSnapshot(ctx, result.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Status != "parsed" {
		t.Fatalf("Status = %q, want parsed", snapshot.Status)
	}
	if !strings.Contains(snapshot.SkillsJSON, "Go") {
		t.Fatalf("SkillsJSON = %q, want Go", snapshot.SkillsJSON)
	}
	if snapshot.RemotePolicy != "remote" {
		t.Fatalf("RemotePolicy = %q, want remote", snapshot.RemotePolicy)
	}
	if !snapshot.SalaryMin.Valid || snapshot.SalaryMin.Int64 != 140000 {
		t.Fatalf("SalaryMin = %#v, want 140000", snapshot.SalaryMin)
	}
}
