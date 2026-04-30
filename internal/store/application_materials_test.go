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

	material, err := st.UpdateApplicationMaterialStatus(ctx, UpdateApplicationMaterialStatusParams{
		ID:     materialID,
		Status: "approved",
		Notes:  "Approved for application use.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if material.Status != "approved" {
		t.Fatalf("material.Status = %q, want approved", material.Status)
	}
	answerDocID, err := st.CreateDocument(ctx, CreateDocumentParams{
		Kind:      "application_answers",
		Format:    "markdown",
		Path:      "/tmp/answers.md",
		SHA256:    "answers",
		SizeBytes: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	answerMaterialID, err := st.CreateApplicationMaterial(ctx, CreateApplicationMaterialParams{
		ApplicationID:      app.ApplicationID,
		JobID:              jobID,
		CandidateProfileID: profileID,
		Kind:               "application_answers",
		DocumentID:         answerDocID,
		Status:             "draft",
		SourceEventID:      "ready-event",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateApplicationMaterialStatus(ctx, UpdateApplicationMaterialStatusParams{
		ID:     answerMaterialID,
		Status: "approved",
		Notes:  "Approved answers.",
	}); err != nil {
		t.Fatal(err)
	}

	handoff, err := st.ApproveApplicationForAutomation(ctx, app.ApplicationID)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.AutomationRun.ID == 0 {
		t.Fatal("automation run id = 0")
	}
	if handoff.Packet.Materials.Resume.ID != materialID {
		t.Fatalf("packet resume material = %d, want %d", handoff.Packet.Materials.Resume.ID, materialID)
	}
	if len(handoff.Packet.Materials.Answers) != 1 || handoff.Packet.Materials.Answers[0].ID != answerMaterialID {
		t.Fatalf("packet answers = %#v, want approved answer material %d", handoff.Packet.Materials.Answers, answerMaterialID)
	}

	started, err := st.StartAutomationRun(ctx, handoff.AutomationRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != "started" {
		t.Fatalf("started.Status = %q, want started", started.Status)
	}
	if err := st.AddAutomationLog(ctx, AutomationLogParams{RunID: started.ID, Message: "loaded packet"}); err != nil {
		t.Fatal(err)
	}
	review, err := st.MarkAutomationReviewRequired(ctx, started.ID, "https://example.test/apply")
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != "review_required" {
		t.Fatalf("review.Status = %q, want review_required", review.Status)
	}

	runs, err := st.APIAutomationRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || len(runs[0].Logs) != 1 {
		t.Fatalf("automation runs = %#v, want one run with one log", runs)
	}
	submitted, err := st.MarkAutomationSubmitted(ctx, started.ID, "https://example.test/submitted")
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Status != "submitted" {
		t.Fatalf("submitted.Status = %q, want submitted", submitted.Status)
	}
	retry, err := st.RetryAutomationRun(ctx, submitted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID == submitted.ID || retry.Status != "requested" {
		t.Fatalf("retry = %#v, want new requested run", retry)
	}
}
