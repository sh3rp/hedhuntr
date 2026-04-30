package automationworker

import (
	"net/url"
	"strings"

	"hedhuntr/internal/store"
)

type ATSAdapter interface {
	Name() string
	Matches(store.AutomationPacket) bool
	Plan(store.AutomationPacket) AdapterPlan
}

type AdapterPlan struct {
	Adapter        string         `json:"adapter"`
	ApplicationURL string         `json:"applicationUrl"`
	FinalURL       string         `json:"finalUrl"`
	Steps          []AdapterStep  `json:"steps"`
	Materials      map[string]any `json:"materials"`
	ReviewOnly     bool           `json:"reviewOnly"`
}

type AdapterStep struct {
	Action      string `json:"action"`
	Description string `json:"description"`
}

func BuildAdapterPlan(packet store.AutomationPacket, allowed []string) AdapterPlan {
	adapters := []ATSAdapter{
		hostAdapter{name: "greenhouse", hosts: []string{"greenhouse.io", "greenhouse.com", "boards.greenhouse.io"}},
		hostAdapter{name: "lever", hosts: []string{"lever.co", "jobs.lever.co"}},
		hostAdapter{name: "ashby", hosts: []string{"ashbyhq.com", "jobs.ashbyhq.com"}},
		hostAdapter{name: "workday", hosts: []string{"myworkdayjobs.com", "workdayjobs.com"}},
		genericAdapter{},
	}
	allowedSet := map[string]struct{}{}
	for _, name := range allowed {
		name = strings.TrimSpace(strings.ToLower(name))
		if name != "" {
			allowedSet[name] = struct{}{}
		}
	}
	for _, adapter := range adapters {
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[adapter.Name()]; !ok {
				continue
			}
		}
		if adapter.Matches(packet) {
			return adapter.Plan(packet)
		}
	}
	return genericAdapter{}.Plan(packet)
}

type hostAdapter struct {
	name  string
	hosts []string
}

func (a hostAdapter) Name() string {
	return a.name
}

func (a hostAdapter) Matches(packet store.AutomationPacket) bool {
	host := applicationHost(packet)
	for _, candidate := range a.hosts {
		if host == candidate || strings.HasSuffix(host, "."+candidate) {
			return true
		}
	}
	return false
}

func (a hostAdapter) Plan(packet store.AutomationPacket) AdapterPlan {
	return basePlan(a.name, packet, []AdapterStep{
		{Action: "open_application", Description: "Open the ATS application URL in an assisted browser session."},
		{Action: "map_identity_fields", Description: "Prepare candidate identity, contact, and location fields for user review."},
		{Action: "attach_resume", Description: "Prepare the approved resume material for upload."},
		{Action: "apply_answers", Description: "Prepare approved application answers for matching ATS questions."},
		{Action: "stop_before_submit", Description: "Stop before final submission and require explicit user confirmation."},
	})
}

type genericAdapter struct{}

func (genericAdapter) Name() string {
	return "generic"
}

func (genericAdapter) Matches(store.AutomationPacket) bool {
	return true
}

func (genericAdapter) Plan(packet store.AutomationPacket) AdapterPlan {
	return basePlan("generic", packet, []AdapterStep{
		{Action: "open_application", Description: "Open the application URL for manual assisted filling."},
		{Action: "present_packet", Description: "Present approved resume, cover letter, and application answers for copy/paste."},
		{Action: "stop_before_submit", Description: "Stop before final submission and require explicit user confirmation."},
	})
}

func basePlan(adapter string, packet store.AutomationPacket, steps []AdapterStep) AdapterPlan {
	finalURL := packet.Job.ApplicationURL
	if finalURL == "" {
		finalURL = packet.Job.SourceURL
	}
	materials := map[string]any{
		"resume": map[string]any{
			"id":   packet.Materials.Resume.ID,
			"path": packet.Materials.Resume.Path,
		},
		"answers_count": len(packet.Materials.Answers),
	}
	if packet.Materials.CoverLetter != nil {
		materials["cover_letter"] = map[string]any{
			"id":   packet.Materials.CoverLetter.ID,
			"path": packet.Materials.CoverLetter.Path,
		}
	}
	return AdapterPlan{
		Adapter:        adapter,
		ApplicationURL: packet.Job.ApplicationURL,
		FinalURL:       finalURL,
		Steps:          steps,
		Materials:      materials,
		ReviewOnly:     true,
	}
}

func applicationHost(packet store.AutomationPacket) string {
	rawURL := packet.Job.ApplicationURL
	if rawURL == "" {
		rawURL = packet.Job.SourceURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
