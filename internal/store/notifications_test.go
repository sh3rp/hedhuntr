package store

import (
	"context"
	"path/filepath"
	"testing"

	"hedhuntr/internal/events"
)

func TestNotificationDeliveryPersistence(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.UpsertNotificationChannel(ctx, UpsertNotificationChannelParams{
		Name:       "discord",
		Type:       "discord",
		Enabled:    true,
		WebhookURL: "https://example.com/webhook",
	}); err != nil {
		t.Fatal(err)
	}
	minScore := 70
	if err := st.UpsertNotificationRule(ctx, UpsertNotificationRuleParams{
		Name:         "jobs-matched",
		EventSubject: events.SubjectJobsMatched,
		Enabled:      true,
		MinScore:     &minScore,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordNotificationDelivery(ctx, RecordNotificationDeliveryParams{
		ChannelName:  "discord",
		ChannelType:  "discord",
		EventID:      "event-1",
		EventSubject: events.SubjectJobsMatched,
		Status:       "sent",
		StatusCode:   204,
	}); err != nil {
		t.Fatal(err)
	}

	count, err := st.CountNotificationDeliveries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("CountNotificationDeliveries() = %d, want 1", count)
	}

	channels, err := st.ListNotificationChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || !channels[0].Enabled || channels[0].Type != "discord" {
		t.Fatalf("channels = %#v, want enabled discord channel", channels)
	}
	rules, err := st.ListNotificationRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || !rules[0].Enabled || rules[0].MinScore == nil || *rules[0].MinScore != 70 {
		t.Fatalf("rules = %#v, want enabled jobs-matched rule min score 70", rules)
	}
}
