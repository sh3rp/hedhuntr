package automationworker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"hedhuntr/internal/config"
	"hedhuntr/internal/store"
)

type BrowserExecution struct {
	Enabled     bool   `json:"enabled"`
	Launched    bool   `json:"launched"`
	Command     string `json:"command,omitempty"`
	HandoffPath string `json:"handoffPath,omitempty"`
	URL         string `json:"url,omitempty"`
	Message     string `json:"message"`
}

type browserHandoff struct {
	CreatedAt time.Time                 `json:"createdAt"`
	RunID     int64                     `json:"runId"`
	Packet    browserHandoffPacket      `json:"packet"`
	Plan      AdapterPlan               `json:"plan"`
	Guardrail string                    `json:"guardrail"`
	Answers   []store.APIReviewMaterial `json:"answers,omitempty"`
}

type browserHandoffPacket struct {
	ApplicationID  int64  `json:"applicationId"`
	JobTitle       string `json:"jobTitle"`
	Company        string `json:"company"`
	SourceURL      string `json:"sourceUrl"`
	ApplicationURL string `json:"applicationUrl"`
}

func ExecuteAssistedBrowser(ctx context.Context, cfg config.BrowserAutomationConfig, run store.AutomationRun, packet store.AutomationPacket, plan AdapterPlan) (BrowserExecution, error) {
	if !cfg.Enabled {
		return BrowserExecution{Enabled: false, Message: "Assisted browser execution disabled."}, nil
	}
	finalURL := plan.FinalURL
	if finalURL == "" {
		return BrowserExecution{Enabled: true, Message: "No application URL available for assisted browser execution."}, nil
	}
	if cfg.HandoffDir == "" {
		cfg.HandoffDir = "data/automation-handoffs"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}

	handoffPath, err := writeBrowserHandoff(cfg.HandoffDir, run, packet, plan)
	if err != nil {
		return BrowserExecution{Enabled: true, URL: finalURL}, err
	}

	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		command = defaultBrowserCommand()
	}
	if command == "" {
		return BrowserExecution{
			Enabled:     true,
			Launched:    false,
			HandoffPath: handoffPath,
			URL:         finalURL,
			Message:     "No browser command configured for this platform.",
		}, nil
	}

	args := browserArgs(cfg.Args, finalURL)
	launchCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	cmd := exec.CommandContext(launchCtx, command, args...)
	if err := cmd.Start(); err != nil {
		return BrowserExecution{
			Enabled:     true,
			Launched:    false,
			Command:     command,
			HandoffPath: handoffPath,
			URL:         finalURL,
		}, fmt.Errorf("launch assisted browser: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return BrowserExecution{
			Enabled:     true,
			Launched:    false,
			Command:     command,
			HandoffPath: handoffPath,
			URL:         finalURL,
		}, fmt.Errorf("assisted browser command failed: %w", err)
	}
	return BrowserExecution{
		Enabled:     true,
		Launched:    true,
		Command:     command,
		HandoffPath: handoffPath,
		URL:         finalURL,
		Message:     "Assisted browser session launched. Final submission remains blocked.",
	}, nil
}

func writeBrowserHandoff(dir string, run store.AutomationRun, packet store.AutomationPacket, plan AdapterPlan) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create browser handoff directory: %w", err)
	}
	handoff := browserHandoff{
		CreatedAt: time.Now().UTC(),
		RunID:     run.ID,
		Packet: browserHandoffPacket{
			ApplicationID:  packet.ApplicationID,
			JobTitle:       packet.Job.Title,
			Company:        packet.Job.Company,
			SourceURL:      packet.Job.SourceURL,
			ApplicationURL: packet.Job.ApplicationURL,
		},
		Plan:      plan,
		Guardrail: "Use the approved packet for assisted filling only. Stop before final submission.",
		Answers:   packet.Materials.Answers,
	}
	content, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal browser handoff: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("automation-run-%d-application-%d-%s.json", run.ID, packet.ApplicationID, plan.Adapter))
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", fmt.Errorf("write browser handoff: %w", err)
	}
	return path, nil
}

func browserArgs(configured []string, finalURL string) []string {
	if len(configured) == 0 {
		return []string{finalURL}
	}
	args := make([]string, len(configured))
	replaced := false
	for i, arg := range configured {
		args[i] = strings.ReplaceAll(arg, "{url}", finalURL)
		if args[i] != arg {
			replaced = true
		}
	}
	if !replaced {
		args = append(args, finalURL)
	}
	return args
}

func defaultBrowserCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	default:
		return "xdg-open"
	}
}
