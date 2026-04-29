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

func TestCandidateProfileAndJobMatchPersistence(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "matching.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	minSalary := 120000
	profileID, err := st.UpsertCandidateProfile(ctx, UpsertCandidateProfileParams{
		Name:               "Test Candidate",
		Skills:             []string{"Go", "NATS", "SQLite"},
		PreferredTitles:    []string{"backend engineer"},
		PreferredLocations: []string{"remote"},
		RemotePreference:   "remote",
		MinSalary:          &minSalary,
	})
	if err != nil {
		t.Fatal(err)
	}

	payload := events.JobDiscoveredPayload{
		Source:       "static-test",
		ExternalID:   "job-1",
		Title:        "Senior Backend Engineer",
		Company:      "ExampleCo",
		Location:     "Remote",
		SourceURL:    "https://example.com/jobs/1",
		Description:  "Build Go services.",
		DiscoveredAt: time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
	}
	envelope := events.NewJobDiscovered("static-test", payload)
	raw, err := json.Marshal(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, envelope, raw)
	if err != nil {
		t.Fatal(err)
	}
	maxSalary := 180000
	if err := st.SaveParsedJob(ctx, SaveParsedJobParams{
		JobID:          saved.JobID,
		Skills:         []string{"Go", "NATS", "Docker"},
		SalaryMax:      &maxSalary,
		RemotePolicy:   "remote",
		EmploymentType: "full_time",
		ParsedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	profile, err := st.GetCandidateProfile(ctx, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "Test Candidate" {
		t.Fatalf("profile.Name = %q, want Test Candidate", profile.Name)
	}

	job, err := st.GetJobForMatching(ctx, saved.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Skills) != 3 {
		t.Fatalf("job.Skills = %#v, want 3 skills", job.Skills)
	}

	if err := st.SaveJobMatch(ctx, SaveJobMatchParams{
		JobID:              saved.JobID,
		CandidateProfileID: profileID,
		Score:              80,
		MatchedSkills:      []string{"Go", "NATS"},
		MissingSkills:      []string{"Docker"},
		Notes:              []string{"Good fit"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkApplicationReady(ctx, saved.JobID, profileID, 80); err != nil {
		t.Fatal(err)
	}

	snapshot, err := st.GetJobMatchSnapshot(ctx, saved.JobID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Score != 80 {
		t.Fatalf("Score = %d, want 80", snapshot.Score)
	}
	if !strings.Contains(snapshot.MatchedSkillsJSON, "Go") {
		t.Fatalf("MatchedSkillsJSON = %q, want Go", snapshot.MatchedSkillsJSON)
	}
	if snapshot.ApplicationStatus != "ready_to_apply" {
		t.Fatalf("ApplicationStatus = %q, want ready_to_apply", snapshot.ApplicationStatus)
	}
}
