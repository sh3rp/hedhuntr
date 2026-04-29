package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSchedulerSourceRunRecording(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	sourceID, err := st.UpsertJobSource(ctx, UpsertJobSourceParams{
		Name:            "example-static",
		Type:            "static",
		Enabled:         true,
		Schedule:        "hourly",
		IntervalSeconds: 3600,
		TimeoutSeconds:  120,
	})
	if err != nil {
		t.Fatal(err)
	}

	sources, err := st.ListEnabledJobSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("len(sources) = %d, want 1", len(sources))
	}
	if sources[0].ID != sourceID {
		t.Fatalf("source ID = %d, want %d", sources[0].ID, sourceID)
	}

	startedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	runID, err := st.BeginSourceRun(ctx, sources[0], startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if runID == 0 {
		t.Fatal("runID = 0, want non-zero")
	}

	if err := st.CompleteSourceRun(ctx, CompleteSourceRunParams{
		RunID:           runID,
		Status:          "succeeded",
		JobsSeen:        3,
		EventsPublished: 3,
		FinishedAt:      startedAt.Add(2 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	count, err := st.CountSourceRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("CountSourceRuns() = %d, want 1", count)
	}

	sources, err = st.ListEnabledJobSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sources[0].LastRunAt == nil {
		t.Fatal("LastRunAt = nil, want timestamp")
	}
	if sources[0].LastSuccessAt == nil {
		t.Fatal("LastSuccessAt = nil, want timestamp")
	}
}

func TestListEnabledJobSourcesSkipsDisabledSources(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	_, err = st.UpsertJobSource(ctx, UpsertJobSourceParams{
		Name:            "disabled",
		Type:            "static",
		Enabled:         false,
		Schedule:        "hourly",
		IntervalSeconds: 3600,
		TimeoutSeconds:  120,
	})
	if err != nil {
		t.Fatal(err)
	}

	sources, err := st.ListEnabledJobSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Fatalf("len(sources) = %d, want 0", len(sources))
	}
}
