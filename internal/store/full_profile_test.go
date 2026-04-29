package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"hedhuntr/internal/profile"
)

func TestUpsertAndLoadFullCandidateProfile(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	minSalary := 140000
	input := profile.Profile{
		ID:                 1,
		Name:               "Example Candidate",
		Headline:           "Backend engineer",
		Skills:             []string{"Go", "NATS", "SQLite"},
		PreferredTitles:    []string{"backend engineer"},
		PreferredLocations: []string{"remote"},
		RemotePreference:   "remote",
		MinSalary:          &minSalary,
		WorkHistory: []profile.WorkHistory{
			{
				Company:      "ExampleCo",
				Title:        "Senior Backend Engineer",
				Current:      true,
				Highlights:   []string{"Built services"},
				Technologies: []string{"Go"},
			},
		},
		Projects: []profile.Project{
			{Name: "Job Pipeline", Technologies: []string{"Go", "SQLite"}},
		},
		Education: []profile.Education{
			{Institution: "Example University", Degree: "B.S."},
		},
		Certifications: []profile.Certification{
			{Name: "AWS Certified Developer", Issuer: "AWS"},
		},
		Links: []profile.Link{
			{Label: "GitHub", URL: "https://github.com/example"},
		},
	}

	id, err := st.UpsertFullCandidateProfile(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id = %d, want 1", id)
	}

	stored, err := st.LoadFullCandidateProfile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != input.Name {
		t.Fatalf("Name = %q, want %q", stored.Name, input.Name)
	}
	if len(stored.WorkHistory) != 1 || stored.WorkHistory[0].Company != "ExampleCo" {
		t.Fatalf("WorkHistory = %#v, want ExampleCo", stored.WorkHistory)
	}
	if len(stored.Projects) != 1 || stored.Projects[0].Technologies[1] != "SQLite" {
		t.Fatalf("Projects = %#v, want SQLite project", stored.Projects)
	}
	if len(stored.Links) != 1 || stored.Links[0].Label != "GitHub" {
		t.Fatalf("Links = %#v, want GitHub", stored.Links)
	}
}

func TestExampleCandidateProfileJSONValidates(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "configs", "candidate-profile.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p profile.Profile
	if err := json.Unmarshal(content, &p); err != nil {
		t.Fatal(err)
	}
	if err := profile.Validate(p); err != nil {
		t.Fatal(err)
	}
}
