package scheduler

import (
	"testing"
	"time"

	"hedhuntr/internal/store"
)

func TestDueSourcesIncludesNeverRunSources(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	due := DueSources([]store.JobSource{
		{Name: "new-source", Enabled: true, IntervalSeconds: 3600},
	}, now)

	if len(due) != 1 || due[0].Name != "new-source" {
		t.Fatalf("DueSources() = %#v, want new-source due", due)
	}
}

func TestDueSourcesSkipsDisabledSources(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	due := DueSources([]store.JobSource{
		{Name: "disabled-source", Enabled: false, IntervalSeconds: 3600},
	}, now)

	if len(due) != 0 {
		t.Fatalf("len(due) = %d, want 0", len(due))
	}
}

func TestDueSourcesUsesInterval(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-30 * time.Minute)
	old := now.Add(-2 * time.Hour)

	due := DueSources([]store.JobSource{
		{Name: "recent", Enabled: true, IntervalSeconds: 3600, LastRunAt: &recent},
		{Name: "old", Enabled: true, IntervalSeconds: 3600, LastRunAt: &old},
	}, now)

	if len(due) != 1 || due[0].Name != "old" {
		t.Fatalf("DueSources() = %#v, want only old source due", due)
	}
}
