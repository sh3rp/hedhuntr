package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hedhuntr/internal/events"
	"hedhuntr/internal/profile"
)

func TestApplicationReadyContextAndMaterials(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "materials.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	profileID, err := st.UpsertFullCandidateProfile(ctx, profile.Profile{
		Name:            "Alex Example",
		Skills:          []string{"Go", "SQLite"},
		PreferredTitles: []string{"Backend Engineer"},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := st.SaveDiscoveredJob(ctx, "jobs.discovered", events.Envelope[events.JobDiscoveredPayload]{
		EventID:        "event-1",
		EventType:      events.EventJobDiscovered,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		Source:         "test",
		CorrelationID:  "corr",
		IdempotencyKey: "test:job",
		Payload: events.JobDiscoveredPayload{
			Source:       "test",
			ExternalID:   "job",
			Title:        "Backend Engineer",
			Company:      "Acme",
			SourceURL:    "https://example.test/jobs/1",
			DiscoveredAt: time.Now().UTC(),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobID := result.JobID
	if err := st.SaveJobMatch(ctx, SaveJobMatchParams{
		JobID:              jobID,
		CandidateProfileID: profileID,
		Score:              88,
		MatchedSkills:      []string{"Go"},
		MissingSkills:      []string{"NATS"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkApplicationReady(ctx, jobID, profileID, 88); err != nil {
		t.Fatal(err)
	}

	app, err := st.GetApplicationReadyContext(ctx, jobID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	if app.ApplicationID == 0 || app.JobTitle != "Backend Engineer" || app.MatchScore != 88 {
		t.Fatalf("application context = %#v", app)
	}

	docID, err := st.CreateDocument(ctx, CreateDocumentParams{
		Kind:      "tailored_resume",
		Format:    "markdown",
		Path:      "/tmp/resume.md",
		SHA256:    "abc",
		SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	materialID, err := st.CreateApplicationMaterial(ctx, CreateApplicationMaterialParams{
		ApplicationID:      app.ApplicationID,
		JobID:              jobID,
		CandidateProfileID: profileID,
		Kind:               "resume",
		DocumentID:         docID,
		Status:             "draft",
		SourceEventID:      "ready-event",
	})
	if err != nil {
		t.Fatal(err)
	}
	if materialID == 0 {
		t.Fatal("materialID = 0")
	}
}
