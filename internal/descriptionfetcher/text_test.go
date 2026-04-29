package descriptionfetcher

import (
	"strings"
	"testing"
)

func TestTextFromHTML(t *testing.T) {
	input := `
<html>
  <head><style>.hidden { display: none; }</style></head>
  <body>
    <h1>Senior Backend Engineer</h1>
    <p>Build Go services &amp; event pipelines.</p>
    <script>alert("ignored")</script>
    <ul><li>NATS</li><li>SQLite</li></ul>
  </body>
</html>`

	got := TextFromHTML(input)
	for _, want := range []string{
		"Senior Backend Engineer",
		"Build Go services & event pipelines.",
		"NATS",
		"SQLite",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("TextFromHTML() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "alert") || strings.Contains(got, "display") {
		t.Fatalf("TextFromHTML() included script/style content: %q", got)
	}
}
