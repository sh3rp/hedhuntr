package profile

import "testing"

func TestValidateRequiresName(t *testing.T) {
	err := Validate(Profile{Skills: []string{"Go"}})
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestValidateRequiresSkills(t *testing.T) {
	err := Validate(Profile{Name: "Example Candidate"})
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestValidateAcceptsCompleteProfile(t *testing.T) {
	err := Validate(Profile{
		Name:             "Example Candidate",
		Skills:           []string{"Go"},
		RemotePreference: "remote",
		WorkHistory: []WorkHistory{
			{Company: "ExampleCo", Title: "Engineer"},
		},
		Projects: []Project{
			{Name: "Project"},
		},
		Education: []Education{
			{Institution: "Example University"},
		},
		Certifications: []Certification{
			{Name: "Certification"},
		},
		Links: []Link{
			{Label: "GitHub", URL: "https://example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestAssessQualityScoresCompleteProfileReady(t *testing.T) {
	minSalary := 150000
	report := AssessQuality(Profile{
		Name:               "Example Candidate",
		Headline:           "Backend engineer",
		Skills:             []string{"Go", "SQLite", "NATS", "React", "TypeScript"},
		PreferredTitles:    []string{"Backend Engineer"},
		PreferredLocations: []string{"Remote"},
		RemotePreference:   "remote",
		MinSalary:          &minSalary,
		WorkHistory: []WorkHistory{
			{
				Company:    "ExampleCo",
				Title:      "Senior Engineer",
				Summary:    "Built backend systems.",
				Highlights: []string{"Built Go APIs.", "Improved SQLite performance.", "Shipped NATS workflows."},
			},
		},
		Projects: []Project{
			{Name: "Job Pipeline", Technologies: []string{"Go", "SQLite"}},
		},
		Education: []Education{
			{Institution: "Example University"},
		},
		Links: []Link{
			{Label: "GitHub", URL: "https://example.com"},
		},
	})

	if report.Status != "ready" || report.Score != 100 {
		t.Fatalf("quality report = %#v, want ready 100", report)
	}
}

func TestAssessQualityFindsMissingProfileInputs(t *testing.T) {
	report := AssessQuality(Profile{Name: "Example Candidate", Skills: []string{"Go"}})
	if report.Status != "incomplete" {
		t.Fatalf("Status = %q, want incomplete", report.Status)
	}
	if report.Score >= 65 {
		t.Fatalf("Score = %d, want less than 65", report.Score)
	}
	var foundWorkHistory bool
	for _, item := range report.Checks {
		if item.ID == "work_history" && item.Status == "missing" {
			foundWorkHistory = true
		}
	}
	if !foundWorkHistory {
		t.Fatalf("Checks = %#v, want missing work_history", report.Checks)
	}
}
