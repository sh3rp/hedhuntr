package descriptionfetcher

import (
	"html"
	"regexp"
	"strings"
)

var (
	scriptPattern  = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</(script|style|noscript)>`)
	breakPattern   = regexp.MustCompile(`(?i)<\s*(br|/p|/li|/div|/section|/h[1-6])[^>]*>`)
	tagPattern     = regexp.MustCompile(`(?s)<[^>]*>`)
	spacePattern   = regexp.MustCompile(`[ \t\r\f\v]+`)
	newlinePattern = regexp.MustCompile(`\n{3,}`)
)

func TextFromHTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}

	text := scriptPattern.ReplaceAllString(input, " ")
	text = breakPattern.ReplaceAllString(text, "\n")
	text = tagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = spacePattern.ReplaceAllString(text, " ")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")
	text = newlinePattern.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
