package sources

import (
	"context"
	"encoding/json"
	"testing"

	"hedhuntr/internal/config"
)

func TestStaticSourceFetchNormalizesJobs(t *testing.T) {
	settings := staticSettings{
		Jobs: []staticJob{
			{
				ExternalID:     "job-1",
				Title:          " Backend Engineer ",
				Company:        " ExampleCo ",
				Location:       " Remote ",
				SourceURL:      " https://example.com/jobs/1 ",
				ApplicationURL: " https://example.com/apply/1 ",
				DetectedSkills: []string{"Go"},
			},
		},
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}

	source, err := newStatic(config.SourceConfig{
		Name:     "static-test",
		Type:     "static",
		Enabled:  true,
		Settings: raw,
	})
	if err != nil {
		t.Fatal(err)
	}

	jobs, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].Title != "Backend Engineer" {
		t.Fatalf("Title = %q, want normalized title", jobs[0].Title)
	}
	if jobs[0].Source != "static-test" {
		t.Fatalf("Source = %q, want static-test", jobs[0].Source)
	}
	if len(jobs[0].DetectedSkills) != 1 || jobs[0].DetectedSkills[0] != "Go" {
		t.Fatalf("DetectedSkills = %#v, want [Go]", jobs[0].DetectedSkills)
	}
}
