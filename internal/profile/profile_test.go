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
