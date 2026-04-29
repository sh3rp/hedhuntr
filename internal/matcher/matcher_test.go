package matcher

import (
	"slices"
	"testing"
)

func TestScoreMatchesSkillsAndPreferences(t *testing.T) {
	minSalary := 120000
	jobMin := 140000
	jobMax := 180000
	result := Score(CandidateProfile{
		Skills:             []string{"Go", "NATS", "SQLite", "React"},
		PreferredTitles:    []string{"backend engineer"},
		PreferredLocations: []string{"remote"},
		RemotePreference:   "remote",
		MinSalary:          &minSalary,
	}, Job{
		Title:        "Senior Backend Engineer",
		Location:     "Remote",
		Skills:       []string{"Go", "NATS", "SQLite", "Docker"},
		SalaryMin:    &jobMin,
		SalaryMax:    &jobMax,
		RemotePolicy: "remote",
	})

	if result.Score < 80 {
		t.Fatalf("Score = %d, want >= 80", result.Score)
	}
	if !slices.Contains(result.MatchedSkills, "Go") {
		t.Fatalf("MatchedSkills = %#v, want Go", result.MatchedSkills)
	}
	if !slices.Contains(result.MissingSkills, "Docker") {
		t.Fatalf("MissingSkills = %#v, want Docker", result.MissingSkills)
	}
}
