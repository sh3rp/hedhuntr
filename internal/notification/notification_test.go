package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

func TestFormatJobMatched(t *testing.T) {
	message := FormatJobMatched(events.JobMatchedPayload{
		JobID:              42,
		CandidateProfileID: 7,
		Score:              83,
		MatchedSkills:      []string{"Go", "NATS"},
		MissingSkills:      []string{"Docker"},
	})

	for _, want := range []string{"job #42", "scored 83", "Go, NATS", "Docker"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message = %q, missing %q", message, want)
		}
	}
}

func TestShouldNotify(t *testing.T) {
	cfg := config.NotificationConfig{
		MinScore:                70,
		NotifyJobsMatched:       true,
		NotifyApplicationsReady: true,
	}
	if ShouldNotify(events.SubjectJobsMatched, 69, cfg) {
		t.Fatal("ShouldNotify jobs.matched score 69 = true, want false")
	}
	if !ShouldNotify(events.SubjectJobsMatched, 70, cfg) {
		t.Fatal("ShouldNotify jobs.matched score 70 = false, want true")
	}
	if !ShouldNotify(events.SubjectApplicationsReady, 0, cfg) {
		t.Fatal("ShouldNotify applications.ready = false, want true")
	}
}

func TestSenderSendsDiscordWebhook(t *testing.T) {
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := NewSender(0).Send(context.Background(), Channel{
		Name:       "discord",
		Type:       "discord",
		Enabled:    true,
		WebhookURL: server.URL,
	}, "hello")

	if result.Error != "" {
		t.Fatalf("Error = %q, want empty", result.Error)
	}
	if body["content"] != "hello" {
		t.Fatalf("content = %q, want hello", body["content"])
	}
}

func TestSenderSendsSlackWebhook(t *testing.T) {
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := NewSender(0).Send(context.Background(), Channel{
		Name:       "slack",
		Type:       "slack",
		Enabled:    true,
		WebhookURL: server.URL,
	}, "hello")

	if result.Error != "" {
		t.Fatalf("Error = %q, want empty", result.Error)
	}
	if body["text"] != "hello" {
		t.Fatalf("text = %q, want hello", body["text"])
	}
}
