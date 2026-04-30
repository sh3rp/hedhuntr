package automationworker

import (
	"testing"

	"hedhuntr/internal/store"
)

func TestBuildAdapterPlanDetectsSupportedATS(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "greenhouse", url: "https://boards.greenhouse.io/acme/jobs/123", want: "greenhouse"},
		{name: "lever", url: "https://jobs.lever.co/acme/123", want: "lever"},
		{name: "ashby", url: "https://jobs.ashbyhq.com/acme/123", want: "ashby"},
		{name: "workday", url: "https://acme.wd1.myworkdayjobs.com/jobs/job/123", want: "workday"},
		{name: "generic", url: "https://example.com/apply", want: "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := BuildAdapterPlan(testPacket(tt.url), nil)
			if plan.Adapter != tt.want {
				t.Fatalf("Adapter = %q, want %q", plan.Adapter, tt.want)
			}
			if !plan.ReviewOnly {
				t.Fatal("ReviewOnly = false, want true")
			}
			if len(plan.Steps) == 0 {
				t.Fatal("Steps empty")
			}
		})
	}
}

func TestBuildAdapterPlanRespectsAllowedAdapters(t *testing.T) {
	plan := BuildAdapterPlan(testPacket("https://jobs.lever.co/acme/123"), []string{"greenhouse"})
	if plan.Adapter != "generic" {
		t.Fatalf("Adapter = %q, want generic fallback when lever is not allowed", plan.Adapter)
	}
}

func testPacket(applicationURL string) store.AutomationPacket {
	return store.AutomationPacket{
		ApplicationID: 1,
		Job: store.AutomationPacketJob{
			ApplicationURL: applicationURL,
			SourceURL:      "https://example.com/job",
		},
		Materials: store.AutomationPacketMaterials{
			Resume: store.APIReviewMaterial{ID: 7, Path: "/tmp/resume.md"},
		},
	}
}
