package parser

import (
	"slices"
	"testing"
)

func TestParserExtractsSkills(t *testing.T) {
	parsed := New([]string{"Svelte"}).Parse("Senior Backend Engineer", `
Requirements:
- Build services in Go and TypeScript
- Operate NATS, SQLite, and Docker
- Support a Svelte frontend
`)

	for _, skill := range []string{"Go", "TypeScript", "NATS", "SQLite", "Docker", "Svelte"} {
		if !slices.Contains(parsed.Skills, skill) {
			t.Fatalf("Skills = %#v, missing %q", parsed.Skills, skill)
		}
	}
}

func TestParserExtractsRemotePolicy(t *testing.T) {
	parsed := New(nil).Parse("Backend Engineer", "This is a hybrid role based in New York.")
	if parsed.RemotePolicy != "hybrid" {
		t.Fatalf("RemotePolicy = %q, want hybrid", parsed.RemotePolicy)
	}
}

func TestParserExtractsSalary(t *testing.T) {
	parsed := New(nil).Parse("Backend Engineer", "Salary range: $140k - $180k per year.")
	if parsed.SalaryMin == nil || *parsed.SalaryMin != 140000 {
		t.Fatalf("SalaryMin = %#v, want 140000", parsed.SalaryMin)
	}
	if parsed.SalaryMax == nil || *parsed.SalaryMax != 180000 {
		t.Fatalf("SalaryMax = %#v, want 180000", parsed.SalaryMax)
	}
	if parsed.SalaryCurrency != "USD" {
		t.Fatalf("SalaryCurrency = %q, want USD", parsed.SalaryCurrency)
	}
}

func TestParserExtractsSections(t *testing.T) {
	parsed := New(nil).Parse("Senior Backend Engineer", `
Responsibilities:
- Build APIs
- Improve observability

Requirements:
- 5 years experience
- Strong SQL skills
`)

	if len(parsed.Responsibilities) != 2 {
		t.Fatalf("Responsibilities = %#v, want 2", parsed.Responsibilities)
	}
	if len(parsed.Requirements) != 2 {
		t.Fatalf("Requirements = %#v, want 2", parsed.Requirements)
	}
}
