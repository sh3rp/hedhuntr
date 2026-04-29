package profile

import "strings"

type QualityReport struct {
	Score  int            `json:"score"`
	Status string         `json:"status"`
	Checks []QualityCheck `json:"checks"`
}

type QualityCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Weight  int    `json:"weight"`
}

func AssessQuality(p Profile) QualityReport {
	checks := []QualityCheck{
		check("name", "Name", 5, strings.TrimSpace(p.Name) != "", "Profile has a candidate name.", "Add the candidate name."),
		check("headline", "Headline", 10, strings.TrimSpace(p.Headline) != "", "Headline is ready for resume summaries.", "Add a concise candidate headline."),
		check("skills", "Skills", 15, len(nonBlank(p.Skills)) >= 5, "Skills list has enough signal for matching.", "Add at least five skills."),
		check("preferred_titles", "Preferred Titles", 8, len(nonBlank(p.PreferredTitles)) > 0, "Preferred titles are set.", "Add at least one preferred job title."),
		check("preferred_locations", "Preferred Locations", 7, len(nonBlank(p.PreferredLocations)) > 0 || strings.TrimSpace(p.RemotePreference) != "", "Location or remote preference is set.", "Add preferred locations or a remote preference."),
		check("salary", "Salary Floor", 5, p.MinSalary != nil && *p.MinSalary > 0, "Salary floor is set.", "Add a minimum salary."),
		check("work_history", "Work History", 20, hasUsableWorkHistory(p.WorkHistory), "Work history has role, company, and detail.", "Add at least one role with company, title, and summary or highlights."),
		check("work_highlights", "Work Highlights", 10, countWorkHighlights(p.WorkHistory) >= 3, "Work highlights can feed resume tuning.", "Add at least three work highlights."),
		check("projects", "Projects", 10, hasUsableProject(p.Projects), "Projects can support tailored resumes.", "Add at least one project with summary, highlights, or technologies."),
		check("links", "Links", 5, len(p.Links) > 0, "Profile has at least one supporting link.", "Add a portfolio, GitHub, LinkedIn, or similar link."),
		check("education_or_certs", "Education or Certifications", 5, len(p.Education) > 0 || len(p.Certifications) > 0, "Education or certification history is present.", "Add education or certifications when relevant."),
	}

	score := 0
	for _, item := range checks {
		if item.Status == "complete" {
			score += item.Weight
		}
	}

	status := "incomplete"
	switch {
	case score >= 85:
		status = "ready"
	case score >= 65:
		status = "usable"
	}
	return QualityReport{Score: score, Status: status, Checks: checks}
}

func check(id, label string, weight int, passed bool, completeMessage, missingMessage string) QualityCheck {
	status := "missing"
	message := missingMessage
	if passed {
		status = "complete"
		message = completeMessage
	}
	return QualityCheck{
		ID:      id,
		Label:   label,
		Status:  status,
		Message: message,
		Weight:  weight,
	}
}

func hasUsableWorkHistory(items []WorkHistory) bool {
	for _, item := range items {
		if strings.TrimSpace(item.Company) == "" || strings.TrimSpace(item.Title) == "" {
			continue
		}
		if strings.TrimSpace(item.Summary) != "" || len(nonBlank(item.Highlights)) > 0 {
			return true
		}
	}
	return false
}

func countWorkHighlights(items []WorkHistory) int {
	count := 0
	for _, item := range items {
		count += len(nonBlank(item.Highlights))
	}
	return count
}

func hasUsableProject(items []Project) bool {
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		if strings.TrimSpace(item.Summary) != "" || len(nonBlank(item.Highlights)) > 0 || len(nonBlank(item.Technologies)) > 0 {
			return true
		}
	}
	return false
}

func nonBlank(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}
