package automationworker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/store"
)

func TestExecuteAssistedBrowserWritesHandoffAndLaunchesCommand(t *testing.T) {
	packet := testPacket("https://jobs.lever.co/acme/123")
	plan := BuildAdapterPlan(packet, nil)
	result, err := ExecuteAssistedBrowser(context.Background(), config.BrowserAutomationConfig{
		Enabled:    true,
		Command:    "true",
		HandoffDir: t.TempDir(),
		Timeout:    time.Second,
	}, store.AutomationRun{ID: 42}, packet, plan)
	if err != nil {
		t.Fatalf("ExecuteAssistedBrowser() error = %v", err)
	}
	if !result.Launched {
		t.Fatal("Launched = false, want true")
	}
	if result.HandoffPath == "" {
		t.Fatal("HandoffPath empty")
	}
	content, err := os.ReadFile(result.HandoffPath)
	if err != nil {
		t.Fatalf("read handoff: %v", err)
	}
	if !strings.Contains(string(content), `"adapter": "lever"`) {
		t.Fatalf("handoff missing adapter: %s", content)
	}
	if !strings.Contains(string(content), "Stop before final submission") {
		t.Fatalf("handoff missing guardrail: %s", content)
	}
}

func TestExecuteAssistedBrowserDisabled(t *testing.T) {
	result, err := ExecuteAssistedBrowser(context.Background(), config.BrowserAutomationConfig{}, store.AutomationRun{ID: 1}, testPacket("https://example.com/apply"), AdapterPlan{})
	if err != nil {
		t.Fatalf("ExecuteAssistedBrowser() error = %v", err)
	}
	if result.Enabled {
		t.Fatal("Enabled = true, want false")
	}
}

func TestBrowserArgsReplacesURLPlaceholder(t *testing.T) {
	args := browserArgs([]string{"--new-window", "{url}"}, "https://example.com")
	if len(args) != 2 || args[1] != "https://example.com" {
		t.Fatalf("args = %#v", args)
	}
}
