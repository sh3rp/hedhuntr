package profile

import (
	"fmt"
	"strings"
)

type Profile struct {
	ID                 int64           `json:"id,omitempty"`
	Name               string          `json:"name"`
	Headline           string          `json:"headline,omitempty"`
	Skills             []string        `json:"skills"`
	PreferredTitles    []string        `json:"preferred_titles"`
	PreferredLocations []string        `json:"preferred_locations"`
	RemotePreference   string          `json:"remote_preference,omitempty"`
	MinSalary          *int            `json:"min_salary,omitempty"`
	WorkHistory        []WorkHistory   `json:"work_history"`
	Projects           []Project       `json:"projects"`
	Education          []Education     `json:"education"`
	Certifications     []Certification `json:"certifications"`
	Links              []Link          `json:"links"`
}

type WorkHistory struct {
	Company      string   `json:"company"`
	Title        string   `json:"title"`
	Location     string   `json:"location,omitempty"`
	StartDate    string   `json:"start_date,omitempty"`
	EndDate      string   `json:"end_date,omitempty"`
	Current      bool     `json:"current,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Highlights   []string `json:"highlights,omitempty"`
	Technologies []string `json:"technologies,omitempty"`
}

type Project struct {
	Name         string   `json:"name"`
	Role         string   `json:"role,omitempty"`
	URL          string   `json:"url,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Highlights   []string `json:"highlights,omitempty"`
	Technologies []string `json:"technologies,omitempty"`
}

type Education struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree,omitempty"`
	Field       string `json:"field,omitempty"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type Certification struct {
	Name      string `json:"name"`
	Issuer    string `json:"issuer,omitempty"`
	IssuedAt  string `json:"issued_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	URL       string `json:"url,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func Validate(p Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(p.Skills) == 0 {
		return fmt.Errorf("at least one skill is required")
	}
	if p.MinSalary != nil && *p.MinSalary < 0 {
		return fmt.Errorf("min_salary cannot be negative")
	}
	if p.RemotePreference != "" {
		switch p.RemotePreference {
		case "remote", "hybrid", "onsite":
		default:
			return fmt.Errorf("remote_preference must be one of remote, hybrid, onsite")
		}
	}
	for i, item := range p.WorkHistory {
		if strings.TrimSpace(item.Company) == "" {
			return fmt.Errorf("work_history[%d].company is required", i)
		}
		if strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("work_history[%d].title is required", i)
		}
	}
	for i, item := range p.Projects {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("projects[%d].name is required", i)
		}
	}
	for i, item := range p.Education {
		if strings.TrimSpace(item.Institution) == "" {
			return fmt.Errorf("education[%d].institution is required", i)
		}
	}
	for i, item := range p.Certifications {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("certifications[%d].name is required", i)
		}
	}
	for i, item := range p.Links {
		if strings.TrimSpace(item.Label) == "" {
			return fmt.Errorf("links[%d].label is required", i)
		}
		if strings.TrimSpace(item.URL) == "" {
			return fmt.Errorf("links[%d].url is required", i)
		}
	}
	return nil
}
