package resumetuner

import (
	"strings"
	"testing"

	"hedhuntr/internal/profile"
	"hedhuntr/internal/store"
)

func TestTuneUsesOnlyStoredProfileData(t *testing.T) {
	output := Tune(Input{
		Profile: profile.Profile{
			Name:     "Alex Example",
			Headline: "Backend engineer",
			Skills:   []string{"Go", "SQLite", "React"},
			WorkHistory: []profile.WorkHistory{
				{
					Company:    "ExampleCo",
					Title:      "Senior Engineer",
					Highlights: []string{"Built Go services backed by SQLite.", "Led incident response improvements."},
				},
			},
		},
		Application: store.ApplicationReadyContext{
			JobTitle:      "Platform Engineer",
			Company:       "Acme",
			MatchScore:    91,
			Skills:        []string{"Go", "NATS", "SQLite"},
			MatchedSkills: []string{"Go", "SQLite"},
		},
		BaseResumeName: "base resume",
		MaxHighlights:  3,
	})

	if !strings.Contains(output.ResumeMarkdown, "Platform Engineer at Acme") {
		t.Fatalf("resume markdown missing target role:\n%s", output.ResumeMarkdown)
	}
	if !strings.Contains(output.ResumeMarkdown, "Built Go services backed by SQLite.") {
		t.Fatalf("resume markdown missing stored highlight:\n%s", output.ResumeMarkdown)
	}
	if strings.Contains(output.ResumeMarkdown, "Kubernetes") {
		t.Fatalf("resume markdown introduced unstored claim:\n%s", output.ResumeMarkdown)
	}
	if !strings.Contains(output.CoverLetterMarkdown, "Dear Acme hiring team") {
		t.Fatalf("cover letter missing company:\n%s", output.CoverLetterMarkdown)
	}
}
