package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
)

type Channel struct {
	Name       string
	Type       string
	Enabled    bool
	WebhookURL string
}

type DeliveryResult struct {
	StatusCode   int
	ResponseBody string
	Error        string
}

type Sender struct {
	client *http.Client
}

func NewSender(timeout time.Duration) Sender {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return Sender{client: &http.Client{Timeout: timeout}}
}

func ChannelsFromConfig(channels []config.NotificationChannelConfig) []Channel {
	result := make([]Channel, 0, len(channels))
	for _, channel := range channels {
		result = append(result, Channel{
			Name:       channel.Name,
			Type:       strings.ToLower(strings.TrimSpace(channel.Type)),
			Enabled:    channel.Enabled,
			WebhookURL: strings.TrimSpace(channel.WebhookURL),
		})
	}
	return result
}

func ShouldNotify(subject string, score int, cfg config.NotificationConfig) bool {
	switch subject {
	case events.SubjectJobsMatched:
		return cfg.NotifyJobsMatched && score >= cfg.MinScore
	case events.SubjectApplicationsReady:
		return cfg.NotifyApplicationsReady
	default:
		return false
	}
}

func FormatJobMatched(payload events.JobMatchedPayload) string {
	return fmt.Sprintf(
		"Job match: job #%d scored %d for candidate #%d\nMatched skills: %s\nMissing skills: %s",
		payload.JobID,
		payload.Score,
		payload.CandidateProfileID,
		joinOrNone(payload.MatchedSkills),
		joinOrNone(payload.MissingSkills),
	)
}

func FormatApplicationReady(payload events.ApplicationReadyPayload) string {
	return fmt.Sprintf(
		"Application ready: job #%d is ready to apply for candidate #%d with match score %d.",
		payload.JobID,
		payload.CandidateProfileID,
		payload.MatchScore,
	)
}

func (s Sender) Send(ctx context.Context, channel Channel, message string) DeliveryResult {
	if !channel.Enabled {
		return DeliveryResult{Error: "channel disabled"}
	}
	if channel.WebhookURL == "" {
		return DeliveryResult{Error: "missing webhook_url"}
	}

	var body []byte
	var err error
	switch channel.Type {
	case "discord":
		body, err = json.Marshal(map[string]string{"content": message})
	case "slack":
		body, err = json.Marshal(map[string]string{"text": message})
	default:
		return DeliveryResult{Error: fmt.Sprintf("unsupported channel type %q", channel.Type)}
	}
	if err != nil {
		return DeliveryResult{Error: err.Error()}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return DeliveryResult{Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hedhuntr-notification-worker/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return DeliveryResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	result := DeliveryResult{
		StatusCode:   resp.StatusCode,
		ResponseBody: strings.TrimSpace(string(respBody)),
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		result.Error = fmt.Sprintf("webhook returned status %d", resp.StatusCode)
	}
	return result
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
