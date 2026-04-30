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
	if !strings.Contains(output.AnswersMarkdown, "## Work authorization") {
		t.Fatalf("answers missing work authorization review prompt:\n%s", output.AnswersMarkdown)
	}
	if strings.Contains(output.AnswersMarkdown, "authorized to work") {
		t.Fatalf("answers introduced unstored work authorization claim:\n%s", output.AnswersMarkdown)
	}
}

func TestTuneRendersAndRanksStructuredProfileSections(t *testing.T) {
	output := Tune(Input{
		Profile: profile.Profile{
			Name:     "Alex Example",
			Headline: "Backend engineer",
			Skills:   []string{"Go", "SQLite", "React"},
			WorkHistory: []profile.WorkHistory{
				{
					Company:      "OpsCo",
					Title:        "Reliability Engineer",
					Summary:      "Maintained deployment tooling.",
					Highlights:   []string{"Improved Linux deployment checks."},
					Technologies: []string{"Linux"},
				},
				{
					Company:      "DataCo",
					Title:        "Backend Engineer",
					Location:     "Remote",
					StartDate:    "2021",
					Current:      true,
					Summary:      "Built Go data services backed by SQLite.",
					Highlights:   []string{"Built Go ingestion services with SQLite persistence."},
					Technologies: []string{"Go", "SQLite", "NATS"},
				},
			},
			Projects: []profile.Project{
				{
					Name:         "Portfolio Site",
					Role:         "Owner",
					Summary:      "React site for writing samples.",
					Technologies: []string{"React"},
				},
				{
					Name:         "Job Pipeline",
					Role:         "Backend",
					URL:          "https://example.com/jobs",
					Summary:      "Go and SQLite service for job workflow automation.",
					Highlights:   []string{"Modeled job events with NATS."},
					Technologies: []string{"Go", "SQLite", "NATS"},
				},
			},
			Education: []profile.Education{
				{Institution: "Example University", Degree: "B.S.", Field: "Computer Science", EndDate: "2018"},
			},
			Certifications: []profile.Certification{
				{Name: "Frontend Specialist", Issuer: "Example Org"},
				{Name: "Go Developer", Issuer: "Example Certs", IssuedAt: "2024", URL: "https://example.com/go-cert"},
			},
			Links: []profile.Link{
				{Label: "GitHub", URL: "https://github.com/example"},
			},
		},
		Application: store.ApplicationReadyContext{
			JobTitle:      "Backend Platform Engineer",
			Company:       "Acme",
			MatchScore:    94,
			Skills:        []string{"Go", "SQLite", "NATS"},
			MatchedSkills: []string{"Go", "SQLite", "NATS"},
		},
		BaseResumeName: "base resume",
		MaxHighlights:  4,
	})

	resume := output.ResumeMarkdown
	if strings.Index(resume, "### Backend Engineer, DataCo") > strings.Index(resume, "### Reliability Engineer, OpsCo") {
		t.Fatalf("resume did not rank relevant work first:\n%s", resume)
	}
	if strings.Index(resume, "### Job Pipeline") > strings.Index(resume, "### Portfolio Site") {
		t.Fatalf("resume did not rank relevant project first:\n%s", resume)
	}
	for _, expected := range []string{
		"_Remote | 2021 - Present_",
		"_Technologies: Go, SQLite, NATS_",
		"_Backend | https://example.com/jobs_",
		"## Education",
		"Example University",
		"## Certifications",
		"[Go Developer (Example Certs | 2024)](https://example.com/go-cert)",
		"[GitHub](https://github.com/example)",
	} {
		if !strings.Contains(resume, expected) {
			t.Fatalf("resume missing %q:\n%s", expected, resume)
		}
	}
}
