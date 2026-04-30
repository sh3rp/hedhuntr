package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hedhuntr/internal/events"
	"hedhuntr/internal/profile"
)

func TestInterviewTrackingPersistence(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "interviews.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	appID := seedInterviewApplication(t, ctx, st)
	interview, err := st.CreateInterview(ctx, CreateInterviewParams{
		ApplicationID:   appID,
		Stage:           "technical",
		ScheduledAt:     "2026-05-05T15:00:00Z",
		DurationMinutes: 60,
		Location:        "Zoom",
		Contacts:        []string{"Hiring Manager", "Engineering Lead"},
		Notes:           "Prepare system design examples.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if interview.ID == 0 || interview.Status != "scheduled" || interview.JobTitle != "Backend Engineer" {
		t.Fatalf("interview = %#v", interview)
	}

	task, err := st.CreateInterviewTask(ctx, CreateInterviewTaskParams{
		InterviewID: interview.ID,
		Title:       "Send availability",
		DueAt:       "2026-05-04T15:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == 0 || task.Status != "open" {
		t.Fatalf("task = %#v", task)
	}
	doneTask, err := st.UpdateInterviewTaskStatus(ctx, task.ID, UpdateInterviewTaskStatusParams{Status: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if doneTask.Status != "done" {
		t.Fatalf("doneTask.Status = %q, want done", doneTask.Status)
	}

	updated, err := st.UpdateInterview(ctx, interview.ID, UpdateInterviewParams{
		Status:  "completed",
		Outcome: "advance",
		Notes:   "Panel went well.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "completed" || updated.Outcome != "advance" {
		t.Fatalf("updated = %#v", updated)
	}

	interviews, err := st.ListInterviews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(interviews) != 1 || len(interviews[0].Tasks) != 1 || interviews[0].Tasks[0].Status != "done" {
		t.Fatalf("interviews = %#v, want one interview with one task", interviews)
	}
}

func seedInterviewApplication(t *testing.T, ctx context.Context, st *Store) int64 {
	t.Helper()
	profileID, err := st.UpsertFullCandidateProfile(ctx, profile.Profile{
		Name:            "Alex Example",
		Skills:          []string{"Go", "SQLite"},
		PreferredTitles: []string{"Backend Engineer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := st.SaveDiscoveredJob(ctx, "jobs.discovered", events.Envelope[events.JobDiscoveredPayload]{
		EventID:        "interview-event-1",
		EventType:      events.EventJobDiscovered,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		Source:         "test",
		CorrelationID:  "interview-corr",
		IdempotencyKey: "interview:test:job",
		Payload: events.JobDiscoveredPayload{
			Source:       "test",
			ExternalID:   "interview-job",
			Title:        "Backend Engineer",
			Company:      "Acme",
			SourceURL:    "https://example.test/jobs/interview",
			DiscoveredAt: time.Now().UTC(),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveJobMatch(ctx, SaveJobMatchParams{
		JobID:              result.JobID,
		CandidateProfileID: profileID,
		Score:              91,
		MatchedSkills:      []string{"Go"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkApplicationReady(ctx, result.JobID, profileID, 91); err != nil {
		t.Fatal(err)
	}
	app, err := st.GetApplicationReadyContext(ctx, result.JobID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	return app.ApplicationID
}
