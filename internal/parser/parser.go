package parser

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type ParsedJob struct {
	Skills           []string
	Requirements     []string
	Responsibilities []string
	SalaryMin        *int
	SalaryMax        *int
	SalaryCurrency   string
	SalaryPeriod     string
	RemotePolicy     string
	Seniority        string
	EmploymentType   string
}

type Parser struct {
	skills []string
}

var defaultSkills = []string{
	"aws", "azure", "gcp", "go", "golang", "python", "typescript", "javascript", "react", "node.js", "node",
	"sql", "sqlite", "postgresql", "mysql", "redis", "nats", "kafka", "docker", "kubernetes", "terraform",
	"linux", "graphql", "rest", "grpc", "ci/cd", "git", "github", "playwright",
}

var salaryPattern = regexp.MustCompile(`(?i)\$?\b([1-9][0-9]{1,2})(?:,?000|k)?\s*(?:-|to|–|—)\s*\$?\b([1-9][0-9]{1,2})(?:,?000|k)?\b`)

func New(extraSkills []string) Parser {
	seen := map[string]struct{}{}
	var skills []string
	for _, skill := range append(defaultSkills, extraSkills...) {
		normalized := normalizeSkill(skill)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		skills = append(skills, skill)
	}
	return Parser{skills: skills}
}

func (p Parser) Parse(title, description string) ParsedJob {
	text := strings.TrimSpace(description)
	return ParsedJob{
		Skills:           p.extractSkills(text),
		Requirements:     extractSectionItems(text, []string{"requirements", "required qualifications", "what you bring", "qualifications"}),
		Responsibilities: extractSectionItems(text, []string{"responsibilities", "what you will do", "what you'll do", "about the role"}),
		SalaryMin:        extractSalary(text).min,
		SalaryMax:        extractSalary(text).max,
		SalaryCurrency:   extractSalary(text).currency,
		SalaryPeriod:     extractSalary(text).period,
		RemotePolicy:     ExtractRemotePolicy(title + "\n" + text),
		Seniority:        ExtractSeniority(title + "\n" + text),
		EmploymentType:   ExtractEmploymentType(title + "\n" + text),
	}
}

func (p Parser) extractSkills(text string) []string {
	lower := strings.ToLower(text)
	found := map[string]string{}
	for _, skill := range p.skills {
		normalized := normalizeSkill(skill)
		if normalized == "" {
			continue
		}
		pattern := `(?i)(^|[^a-z0-9+#.])` + regexp.QuoteMeta(strings.ToLower(skill)) + `([^a-z0-9+#.]|$)`
		if regexp.MustCompile(pattern).FindStringIndex(lower) != nil {
			display := skill
			if strings.EqualFold(skill, "golang") {
				display = "Go"
				normalized = "go"
			}
			if strings.EqualFold(skill, "node") {
				display = "Node.js"
				normalized = "node.js"
			}
			found[normalized] = canonicalSkill(display)
		}
	}

	var skills []string
	for _, skill := range found {
		skills = append(skills, skill)
	}
	sort.Strings(skills)
	return skills
}

type salary struct {
	min      *int
	max      *int
	currency string
	period   string
}

func extractSalary(text string) salary {
	match := salaryPattern.FindStringSubmatch(text)
	if len(match) != 3 {
		return salary{}
	}
	min := salaryValue(match[1])
	max := salaryValue(match[2])
	period := "year"
	if strings.Contains(strings.ToLower(text), "hour") {
		period = "hour"
	}
	return salary{min: &min, max: &max, currency: "USD", period: period}
}

func salaryValue(value string) int {
	parsed, _ := strconv.Atoi(strings.ReplaceAll(value, ",", ""))
	if parsed < 1000 {
		return parsed * 1000
	}
	return parsed
}

func ExtractRemotePolicy(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "hybrid"):
		return "hybrid"
	case strings.Contains(lower, "remote"):
		return "remote"
	case strings.Contains(lower, "on-site") || strings.Contains(lower, "onsite") || strings.Contains(lower, "in office"):
		return "onsite"
	default:
		return ""
	}
}

func ExtractSeniority(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "principal") || strings.Contains(lower, "staff"):
		return "staff"
	case strings.Contains(lower, "senior") || strings.Contains(lower, "sr."):
		return "senior"
	case strings.Contains(lower, "lead"):
		return "lead"
	case strings.Contains(lower, "junior") || strings.Contains(lower, "entry level"):
		return "junior"
	default:
		return ""
	}
}

func ExtractEmploymentType(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "contract"):
		return "contract"
	case strings.Contains(lower, "part-time") || strings.Contains(lower, "part time"):
		return "part_time"
	case strings.Contains(lower, "internship") || strings.Contains(lower, "intern "):
		return "internship"
	case strings.Contains(lower, "full-time") || strings.Contains(lower, "full time"):
		return "full_time"
	default:
		return ""
	}
}

func extractSectionItems(text string, headings []string) []string {
	lines := strings.Split(text, "\n")
	inSection := false
	var items []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		heading := normalizeHeading(trimmed)
		if containsHeading(headings, heading) {
			inSection = true
			continue
		}
		if inSection && looksLikeHeading(trimmed) {
			break
		}
		if inSection {
			item := strings.TrimSpace(strings.TrimLeft(trimmed, "-*•0123456789. )"))
			if item != "" {
				items = append(items, item)
			}
		}
	}
	if len(items) > 8 {
		return items[:8]
	}
	return items
}

func containsHeading(headings []string, heading string) bool {
	for _, candidate := range headings {
		if heading == candidate {
			return true
		}
	}
	return false
}

func looksLikeHeading(value string) bool {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), ":")
	if len(trimmed) > 48 {
		return false
	}
	return !strings.Contains(trimmed, ".") && strings.Title(strings.ToLower(trimmed)) == trimmed
}

func normalizeHeading(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), ":"))
}

func normalizeSkill(skill string) string {
	return strings.ToLower(strings.TrimSpace(skill))
}

func canonicalSkill(skill string) string {
	switch strings.ToLower(strings.TrimSpace(skill)) {
	case "aws":
		return "AWS"
	case "gcp":
		return "GCP"
	case "golang":
		return "Go"
	case "go":
		return "Go"
	case "typescript":
		return "TypeScript"
	case "javascript":
		return "JavaScript"
	case "sql":
		return "SQL"
	case "sqlite":
		return "SQLite"
	case "postgresql":
		return "PostgreSQL"
	case "mysql":
		return "MySQL"
	case "redis":
		return "Redis"
	case "nats":
		return "NATS"
	case "kafka":
		return "Kafka"
	case "docker":
		return "Docker"
	case "kubernetes":
		return "Kubernetes"
	case "terraform":
		return "Terraform"
	case "linux":
		return "Linux"
	case "graphql":
		return "GraphQL"
	case "rest":
		return "REST"
	case "grpc":
		return "gRPC"
	case "github":
		return "GitHub"
	case "node":
		return "Node.js"
	default:
		return strings.TrimSpace(skill)
	}
}
