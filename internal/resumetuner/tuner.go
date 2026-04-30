package resumetuner

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"hedhuntr/internal/profile"
	"hedhuntr/internal/store"
)

type Input struct {
	Profile           profile.Profile
	Application       store.ApplicationReadyContext
	BaseResumeName    string
	BaseResumeContent []byte
	MaxHighlights     int
}

type Output struct {
	ResumeMarkdown      string
	CoverLetterMarkdown string
	AnswersMarkdown     string
	Notes               []string
}

func Tune(input Input) Output {
	if input.MaxHighlights <= 0 {
		input.MaxHighlights = 6
	}
	matched := set(input.Application.MatchedSkills)
	jobSkills := orderedUnique(input.Application.Skills)
	prioritySkills := prioritizeSkills(jobSkills, matched)
	highlights := selectHighlights(input.Profile, prioritySkills, input.MaxHighlights)
	workHistory := rankedWorkHistory(input.Profile.WorkHistory, prioritySkills)
	projects := rankedProjects(input.Profile.Projects, prioritySkills)

	var resume bytes.Buffer
	writeLine(&resume, "# %s", input.Profile.Name)
	if input.Profile.Headline != "" {
		writeLine(&resume, "")
		writeLine(&resume, "%s", input.Profile.Headline)
	}
	if len(input.Profile.Links) > 0 {
		links := make([]string, 0, len(input.Profile.Links))
		for _, link := range input.Profile.Links {
			links = append(links, fmt.Sprintf("[%s](%s)", link.Label, link.URL))
		}
		writeLine(&resume, "")
		writeLine(&resume, "%s", strings.Join(links, " | "))
	}

	writeLine(&resume, "")
	writeLine(&resume, "## Target Role")
	writeLine(&resume, "")
	writeLine(&resume, "- %s at %s", input.Application.JobTitle, input.Application.Company)
	if input.Application.Location != "" {
		writeLine(&resume, "- Location: %s", input.Application.Location)
	}
	writeLine(&resume, "- Match score: %d%%", input.Application.MatchScore)

	writeLine(&resume, "")
	writeLine(&resume, "## Relevant Skills")
	writeLine(&resume, "")
	for _, skill := range prioritySkills {
		writeLine(&resume, "- %s", skill)
	}
	if len(prioritySkills) == 0 {
		for _, skill := range input.Profile.Skills {
			writeLine(&resume, "- %s", skill)
		}
	}

	writeLine(&resume, "")
	writeLine(&resume, "## Selected Highlights")
	writeLine(&resume, "")
	for _, item := range highlights {
		writeLine(&resume, "- %s", item)
	}
	if len(highlights) == 0 {
		writeLine(&resume, "- Review the base resume and add role-specific truthful highlights before applying.")
	}

	writeLine(&resume, "")
	writeLine(&resume, "## Experience")
	for _, item := range workHistory {
		writeLine(&resume, "")
		writeLine(&resume, "### %s, %s", item.Title, item.Company)
		endDate := item.EndDate
		if item.Current && endDate == "" {
			endDate = "Present"
		}
		dates := strings.TrimSpace(strings.Join(nonEmpty(item.StartDate, endDate), " - "))
		meta := strings.TrimSpace(strings.Join(nonEmpty(item.Location, dates), " | "))
		if meta != "" {
			writeLine(&resume, "_%s_", meta)
		}
		if len(item.Technologies) > 0 {
			writeLine(&resume, "")
			writeLine(&resume, "_Technologies: %s_", strings.Join(orderedUnique(item.Technologies), ", "))
		}
		if item.Summary != "" {
			writeLine(&resume, "")
			writeLine(&resume, "%s", item.Summary)
		}
		for _, highlight := range item.Highlights {
			writeLine(&resume, "- %s", highlight)
		}
	}

	if len(projects) > 0 {
		writeLine(&resume, "")
		writeLine(&resume, "## Projects")
		for _, item := range projects {
			writeLine(&resume, "")
			writeLine(&resume, "### %s", item.Name)
			meta := strings.Join(nonEmpty(item.Role, item.URL), " | ")
			if meta != "" {
				writeLine(&resume, "_%s_", meta)
			}
			if len(item.Technologies) > 0 {
				writeLine(&resume, "")
				writeLine(&resume, "_Technologies: %s_", strings.Join(orderedUnique(item.Technologies), ", "))
			}
			if item.Summary != "" {
				writeLine(&resume, "")
				writeLine(&resume, "%s", item.Summary)
			}
			for _, highlight := range item.Highlights {
				writeLine(&resume, "- %s", highlight)
			}
		}
	}

	if len(input.Profile.Education) > 0 {
		writeLine(&resume, "")
		writeLine(&resume, "## Education")
		for _, item := range input.Profile.Education {
			writeLine(&resume, "")
			writeLine(&resume, "### %s", item.Institution)
			detail := strings.TrimSpace(strings.Join(nonEmpty(item.Degree, item.Field), ", "))
			dates := strings.TrimSpace(strings.Join(nonEmpty(item.StartDate, item.EndDate), " - "))
			meta := strings.TrimSpace(strings.Join(nonEmpty(detail, dates), " | "))
			if meta != "" {
				writeLine(&resume, "_%s_", meta)
			}
			if item.Summary != "" {
				writeLine(&resume, "")
				writeLine(&resume, "%s", item.Summary)
			}
		}
	}

	if len(input.Profile.Certifications) > 0 {
		writeLine(&resume, "")
		writeLine(&resume, "## Certifications")
		for _, item := range rankedCertifications(input.Profile.Certifications, prioritySkills) {
			dates := strings.TrimSpace(strings.Join(nonEmpty(item.IssuedAt, item.ExpiresAt), " - "))
			meta := strings.TrimSpace(strings.Join(nonEmpty(item.Issuer, dates), " | "))
			line := item.Name
			if meta != "" {
				line = fmt.Sprintf("%s (%s)", line, meta)
			}
			if item.URL != "" {
				line = fmt.Sprintf("[%s](%s)", line, item.URL)
			}
			writeLine(&resume, "- %s", line)
		}
	}

	writeLine(&resume, "")
	writeLine(&resume, "## Review Notes")
	writeLine(&resume, "")
	writeLine(&resume, "- Draft generated from stored candidate profile and base resume `%s`.", input.BaseResumeName)
	writeLine(&resume, "- Verify all ordering and emphasis before sending. Do not add unverified claims.")

	var letter bytes.Buffer
	writeLine(&letter, "# Cover Letter Draft")
	writeLine(&letter, "")
	writeLine(&letter, "Dear %s hiring team,", input.Application.Company)
	writeLine(&letter, "")
	writeLine(&letter, "I am interested in the %s role at %s. My background aligns with the role through %s.", input.Application.JobTitle, input.Application.Company, sentenceList(prioritySkills, "the stored skills in my candidate profile"))
	if len(highlights) > 0 {
		writeLine(&letter, "")
		writeLine(&letter, "Relevant examples from my stored profile include:")
		for _, item := range highlights[:min(len(highlights), 3)] {
			writeLine(&letter, "- %s", item)
		}
	}
	writeLine(&letter, "")
	writeLine(&letter, "I would welcome the chance to discuss how this experience maps to your needs for the role.")
	writeLine(&letter, "")
	writeLine(&letter, "Sincerely,")
	writeLine(&letter, "%s", input.Profile.Name)
	writeLine(&letter, "")
	writeLine(&letter, "## Review Notes")
	writeLine(&letter, "")
	writeLine(&letter, "- This is a draft for human review. Confirm company details, tone, and any role-specific claims before use.")

	var answers bytes.Buffer
	writeLine(&answers, "# Application Answers Draft")
	writeLine(&answers, "")
	writeLine(&answers, "## Why are you interested in this role?")
	writeLine(&answers, "")
	writeLine(&answers, "I am interested in the %s role at %s because it aligns with my stored background in %s.", input.Application.JobTitle, input.Application.Company, sentenceList(prioritySkills, "the areas represented in my candidate profile"))
	writeLine(&answers, "")
	writeLine(&answers, "## What relevant experience do you bring?")
	writeLine(&answers, "")
	if len(highlights) > 0 {
		for _, item := range highlights[:min(len(highlights), 3)] {
			writeLine(&answers, "- %s", item)
		}
	} else {
		writeLine(&answers, "- Review the candidate profile and add a truthful, role-specific example before submitting.")
	}
	writeLine(&answers, "")
	writeLine(&answers, "## What skills match this job?")
	writeLine(&answers, "")
	for _, skill := range prioritySkills {
		writeLine(&answers, "- %s", skill)
	}
	if len(prioritySkills) == 0 {
		writeLine(&answers, "- Review stored skills before answering.")
	}
	writeLine(&answers, "")
	writeLine(&answers, "## Work authorization")
	writeLine(&answers, "")
	writeLine(&answers, "Review and fill this answer manually. The stored candidate profile does not currently include work authorization facts.")
	writeLine(&answers, "")
	writeLine(&answers, "## Salary expectations")
	writeLine(&answers, "")
	if input.Profile.MinSalary != nil {
		writeLine(&answers, "Minimum salary from stored profile: %d.", *input.Profile.MinSalary)
	} else {
		writeLine(&answers, "Review and fill this answer manually. The stored candidate profile does not currently include a salary floor.")
	}
	writeLine(&answers, "")
	writeLine(&answers, "## Review Notes")
	writeLine(&answers, "")
	writeLine(&answers, "- These answers are drafts for human review.")
	writeLine(&answers, "- Do not submit work authorization, sponsorship, clearance, demographic, or legal answers without direct confirmation.")
	writeLine(&answers, "- Do not add claims that are not present in the stored candidate profile or approved resume source.")

	return Output{
		ResumeMarkdown:      strings.TrimSpace(resume.String()) + "\n",
		CoverLetterMarkdown: strings.TrimSpace(letter.String()) + "\n",
		AnswersMarkdown:     strings.TrimSpace(answers.String()) + "\n",
		Notes: []string{
			"Generated deterministic drafts from stored candidate data.",
			"Human approval is required before application submission.",
		},
	}
}

func selectHighlights(p profile.Profile, prioritySkills []string, max int) []string {
	var candidates []string
	for _, item := range p.WorkHistory {
		candidates = append(candidates, item.Highlights...)
	}
	for _, item := range p.Projects {
		candidates = append(candidates, item.Highlights...)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return scoreText(candidates[i], prioritySkills) > scoreText(candidates[j], prioritySkills)
	})
	unique := orderedUnique(candidates)
	return unique[:min(len(unique), max)]
}

func rankedWorkHistory(items []profile.WorkHistory, prioritySkills []string) []profile.WorkHistory {
	out := append([]profile.WorkHistory(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		left := scoreText(workHistoryText(out[i]), prioritySkills)
		right := scoreText(workHistoryText(out[j]), prioritySkills)
		if left == right {
			return false
		}
		return left > right
	})
	return out
}

func rankedProjects(items []profile.Project, prioritySkills []string) []profile.Project {
	out := append([]profile.Project(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		left := scoreText(projectText(out[i]), prioritySkills)
		right := scoreText(projectText(out[j]), prioritySkills)
		if left == right {
			return false
		}
		return left > right
	})
	return out
}

func rankedCertifications(items []profile.Certification, prioritySkills []string) []profile.Certification {
	out := append([]profile.Certification(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		left := scoreText(certificationText(out[i]), prioritySkills)
		right := scoreText(certificationText(out[j]), prioritySkills)
		if left == right {
			return false
		}
		return left > right
	})
	return out
}

func workHistoryText(item profile.WorkHistory) string {
	parts := []string{item.Company, item.Title, item.Location, item.Summary}
	parts = append(parts, item.Highlights...)
	parts = append(parts, item.Technologies...)
	return strings.Join(parts, " ")
}

func projectText(item profile.Project) string {
	parts := []string{item.Name, item.Role, item.URL, item.Summary}
	parts = append(parts, item.Highlights...)
	parts = append(parts, item.Technologies...)
	return strings.Join(parts, " ")
}

func certificationText(item profile.Certification) string {
	return strings.Join([]string{item.Name, item.Issuer, item.URL}, " ")
}

func prioritizeSkills(jobSkills []string, matched map[string]struct{}) []string {
	var skills []string
	for _, skill := range jobSkills {
		if _, ok := matched[strings.ToLower(skill)]; ok {
			skills = append(skills, skill)
		}
	}
	for _, skill := range jobSkills {
		if _, ok := matched[strings.ToLower(skill)]; !ok {
			skills = append(skills, skill)
		}
	}
	return orderedUnique(skills)
}

func scoreText(value string, skills []string) int {
	lower := strings.ToLower(value)
	score := 0
	for _, skill := range skills {
		if strings.Contains(lower, strings.ToLower(skill)) {
			score++
		}
	}
	return score
}

func orderedUnique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func set(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		out[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	return out
}

func writeLine(buf *bytes.Buffer, format string, args ...any) {
	fmt.Fprintf(buf, format+"\n", args...)
}

func nonEmpty(values ...string) []string {
	out := []string{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func sentenceList(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	if len(values) == 1 {
		return values[0]
	}
	if len(values) == 2 {
		return values[0] + " and " + values[1]
	}
	return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
