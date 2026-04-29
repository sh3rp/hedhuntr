package matcher

import (
	"sort"
	"strings"
)

type CandidateProfile struct {
	ID                 int64
	Name               string
	Skills             []string
	PreferredTitles    []string
	PreferredLocations []string
	RemotePreference   string
	MinSalary          *int
}

type Job struct {
	ID             int64
	Title          string
	Location       string
	Skills         []string
	SalaryMin      *int
	SalaryMax      *int
	RemotePolicy   string
	EmploymentType string
}

type Result struct {
	Score         int
	MatchedSkills []string
	MissingSkills []string
	Notes         []string
}

func Score(profile CandidateProfile, job Job) Result {
	var score int
	var notes []string

	matched, missing := matchSkills(profile.Skills, job.Skills)
	if len(job.Skills) == 0 {
		score += 35
		notes = append(notes, "No parsed job skills were available; skill score is neutral.")
	} else {
		score += int(float64(len(matched)) / float64(len(job.Skills)) * 55)
	}

	if titleMatches(profile.PreferredTitles, job.Title) {
		score += 15
		notes = append(notes, "Job title matches candidate preferences.")
	}
	if locationMatches(profile.PreferredLocations, job.Location) {
		score += 10
		notes = append(notes, "Job location matches candidate preferences.")
	}
	if remoteMatches(profile.RemotePreference, job.RemotePolicy) {
		score += 10
		notes = append(notes, "Remote policy matches candidate preference.")
	}
	if salaryMatches(profile.MinSalary, job.SalaryMin, job.SalaryMax) {
		score += 10
		notes = append(notes, "Salary range appears compatible.")
	}

	if score > 100 {
		score = 100
	}
	sort.Strings(matched)
	sort.Strings(missing)
	return Result{Score: score, MatchedSkills: matched, MissingSkills: missing, Notes: notes}
}

func matchSkills(candidateSkills, jobSkills []string) ([]string, []string) {
	candidate := map[string]string{}
	for _, skill := range candidateSkills {
		candidate[normalize(skill)] = skill
	}

	var matched []string
	var missing []string
	seen := map[string]struct{}{}
	for _, skill := range jobSkills {
		key := normalize(skill)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if display, ok := candidate[key]; ok {
			matched = append(matched, display)
		} else {
			missing = append(missing, skill)
		}
	}
	return matched, missing
}

func titleMatches(preferred []string, title string) bool {
	title = normalize(title)
	if title == "" || len(preferred) == 0 {
		return false
	}
	for _, value := range preferred {
		if value = normalize(value); value != "" && strings.Contains(title, value) {
			return true
		}
	}
	return false
}

func locationMatches(preferred []string, location string) bool {
	location = normalize(location)
	if len(preferred) == 0 {
		return false
	}
	for _, value := range preferred {
		value = normalize(value)
		if value == "" {
			continue
		}
		if value == "remote" && strings.Contains(location, "remote") {
			return true
		}
		if strings.Contains(location, value) {
			return true
		}
	}
	return false
}

func remoteMatches(preference, policy string) bool {
	preference = normalize(preference)
	policy = normalize(policy)
	if preference == "" || policy == "" {
		return false
	}
	return preference == policy || preference == "remote" && policy == "hybrid"
}

func salaryMatches(minSalary, jobMin, jobMax *int) bool {
	if minSalary == nil {
		return false
	}
	if jobMax != nil {
		return *jobMax >= *minSalary
	}
	if jobMin != nil {
		return *jobMin >= *minSalary
	}
	return false
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
