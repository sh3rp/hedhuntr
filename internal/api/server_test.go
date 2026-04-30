package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"hedhuntr/internal/config"
	"hedhuntr/internal/events"
	"hedhuntr/internal/profile"
	"hedhuntr/internal/store"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	server, err := NewServer(context.Background(), config.API{
		SQLite: config.SQLiteConfig{Path: filepath.Join(t.TempDir(), "api.db")},
		WebSocket: config.WebSocketConfig{
			PingInterval: 5 * time.Second,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server.SetPublisher(&recordingPublisher{})
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		httpServer.Close()
		server.Close()
	})
	return server, httpServer
}

type publishedEvent struct {
	subject  string
	envelope any
}

type recordingPublisher struct {
	events []publishedEvent
}

func (p *recordingPublisher) Publish(_ context.Context, subject string, envelope any) error {
	p.events = append(p.events, publishedEvent{subject: subject, envelope: envelope})
	return nil
}

func TestHealthEndpoint(t *testing.T) {
	_, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %#v, want ok", body)
	}
}

func TestJobsEndpointShape(t *testing.T) {
	_, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/api/jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body == nil {
		t.Fatal("jobs response decoded to nil slice")
	}
}

func TestWebSocketAckAndSubscribe(t *testing.T) {
	_, httpServer := newTestServer(t)
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var ack WSMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}
	if ack.Type != "ack" {
		t.Fatalf("ack.Type = %q, want ack", ack.Type)
	}

	if err := conn.WriteJSON(map[string]any{"type": "subscribe", "topics": []string{"jobs"}}); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketBroadcast(t *testing.T) {
	server, httpServer := newTestServer(t)
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var ack WSMessage
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}

	server.hub.Broadcast(WSMessage{
		Type:      "event",
		Topic:     "jobs",
		EventID:   "event-1",
		EventType: "JobMatched",
		Payload: map[string]any{
			"job_id": float64(10),
			"score":  float64(93),
		},
	})

	var event WSMessage
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	if event.EventType != "JobMatched" {
		t.Fatalf("event.EventType = %q, want JobMatched", event.EventType)
	}
}

func TestWSMessageFromNATS(t *testing.T) {
	raw := []byte(`{
		"event_id":"evt-1",
		"event_type":"JobSaved",
		"occurred_at":"2026-04-29T14:00:00Z",
		"payload":{"job_id":7,"title":"Backend Engineer"}
	}`)
	message, err := WSMessageFromNATS("jobs.saved", raw)
	if err != nil {
		t.Fatal(err)
	}
	if message.Type != "event" || message.Topic != "jobs" || message.EventType != "JobSaved" {
		t.Fatalf("message = %#v, want jobs JobSaved event", message)
	}
	if message.Payload["title"] != "Backend Engineer" {
		t.Fatalf("payload = %#v, missing title", message.Payload)
	}
}

func TestReviewEndpoints(t *testing.T) {
	server, httpServer := newTestServer(t)
	publisher := &recordingPublisher{}
	server.SetPublisher(publisher)
	materialID := seedReviewMaterial(t, server)

	resp, err := http.Get(httpServer.URL + "/api/review/applications")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var queue []store.APIReviewApplication
	if err := json.NewDecoder(resp.Body).Decode(&queue); err != nil {
		t.Fatal(err)
	}
	if len(queue) != 1 || len(queue[0].Materials) != 1 {
		t.Fatalf("queue = %#v, want one application with one material", queue)
	}

	body := bytes.NewBufferString(`{"status":"approved","notes":"Approved for use."}`)
	resp, err = http.Post(httpServer.URL+"/api/review/materials/"+strconvFormat(materialID)+"/status", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var material store.APIReviewMaterial
	if err := json.NewDecoder(resp.Body).Decode(&material); err != nil {
		t.Fatal(err)
	}
	if material.Status != "approved" {
		t.Fatalf("material.Status = %q, want approved", material.Status)
	}

	resp, err = http.Post(httpServer.URL+"/api/applications/"+strconvFormat(queue[0].ApplicationID)+"/approve-automation", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve automation status = %d, want 200", resp.StatusCode)
	}
	var handoff store.AutomationHandoffResult
	if err := json.NewDecoder(resp.Body).Decode(&handoff); err != nil {
		t.Fatal(err)
	}
	if handoff.AutomationRun.ID == 0 || handoff.Packet.Materials.Resume.ID != materialID {
		t.Fatalf("handoff = %#v", handoff)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("published events = %d, want 2", len(publisher.events))
	}
	if publisher.events[0].subject != events.SubjectApplicationsAutomationApproved {
		t.Fatalf("first subject = %q, want %q", publisher.events[0].subject, events.SubjectApplicationsAutomationApproved)
	}
	if publisher.events[1].subject != events.SubjectAutomationRunRequested {
		t.Fatalf("second subject = %q, want %q", publisher.events[1].subject, events.SubjectAutomationRunRequested)
	}

	resp, err = http.Get(httpServer.URL + "/api/automation/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("automation runs status = %d, want 200", resp.StatusCode)
	}
	var runs []store.APIAutomationRun
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %#v, want one run", runs)
	}

	resp, err = http.Post(httpServer.URL+"/api/automation/runs/"+strconvFormat(handoff.AutomationRun.ID)+"/fail", "application/json", bytes.NewBufferString(`{"message":"manual fail"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fail status = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Post(httpServer.URL+"/api/automation/runs/"+strconvFormat(handoff.AutomationRun.ID)+"/retry", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("retry status = %d, want 200", resp.StatusCode)
	}
	if len(publisher.events) != 3 {
		t.Fatalf("published events after retry = %d, want 3", len(publisher.events))
	}
	if publisher.events[2].subject != events.SubjectAutomationRunRequested {
		t.Fatalf("retry subject = %q, want %q", publisher.events[2].subject, events.SubjectAutomationRunRequested)
	}
}

func TestReviewMaterialRegenerationPublishesApplicationReady(t *testing.T) {
	server, httpServer := newTestServer(t)
	publisher := &recordingPublisher{}
	server.SetPublisher(publisher)
	materialID := seedReviewMaterial(t, server)

	body := bytes.NewBufferString(`{"status":"regeneration_requested","notes":"Try a stronger platform angle."}`)
	resp, err := http.Post(httpServer.URL+"/api/review/materials/"+strconvFormat(materialID)+"/status", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].subject != events.SubjectApplicationsReady {
		t.Fatalf("subject = %q, want %q", publisher.events[0].subject, events.SubjectApplicationsReady)
	}
	envelope, ok := publisher.events[0].envelope.(events.Envelope[events.ApplicationReadyPayload])
	if !ok {
		t.Fatalf("envelope type = %T, want ApplicationReady envelope", publisher.events[0].envelope)
	}
	if envelope.EventType != events.EventApplicationReady || envelope.Source != "api-regeneration" {
		t.Fatalf("envelope = %#v, want api-regeneration ApplicationReady", envelope)
	}
	if envelope.Payload.JobID == 0 || envelope.Payload.CandidateProfileID == 0 || envelope.Payload.MatchScore != 92 {
		t.Fatalf("payload = %#v, want seeded application context", envelope.Payload)
	}
}

func TestUpdateProfileEndpoint(t *testing.T) {
	_, httpServer := newTestServer(t)
	body := bytes.NewBufferString(`{
		"name":"Alex Example",
		"headline":"Backend engineer",
		"skills":["Go","SQLite"],
		"preferred_titles":["Backend Engineer"],
		"preferred_locations":["Remote"],
		"remote_preference":"remote",
		"min_salary":150000,
		"work_history":[],
		"projects":[],
		"education":[],
		"certifications":[],
		"links":[]
	}`)
	req, err := http.NewRequest(http.MethodPut, httpServer.URL+"/api/profile", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var saved profile.Profile
	if err := json.NewDecoder(resp.Body).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == 0 || saved.Name != "Alex Example" || len(saved.Skills) != 2 {
		t.Fatalf("saved profile = %#v", saved)
	}
}

func TestUpdateProfileEndpointValidation(t *testing.T) {
	_, httpServer := newTestServer(t)
	req, err := http.NewRequest(http.MethodPut, httpServer.URL+"/api/profile", bytes.NewBufferString(`{"name":"","skills":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestProfileQualityEndpoint(t *testing.T) {
	server, httpServer := newTestServer(t)
	minSalary := 150000
	if _, err := server.store.UpsertFullCandidateProfile(context.Background(), profile.Profile{
		Name:               "Alex Example",
		Headline:           "Backend engineer",
		Skills:             []string{"Go", "SQLite", "NATS", "React", "TypeScript"},
		PreferredTitles:    []string{"Backend Engineer"},
		PreferredLocations: []string{"Remote"},
		RemotePreference:   "remote",
		MinSalary:          &minSalary,
		WorkHistory: []profile.WorkHistory{
			{
				Company:    "ExampleCo",
				Title:      "Senior Engineer",
				Summary:    "Built backend systems.",
				Highlights: []string{"Built Go APIs.", "Improved SQLite performance.", "Shipped NATS workflows."},
			},
		},
		Projects: []profile.Project{
			{Name: "Job Pipeline", Technologies: []string{"Go", "SQLite"}},
		},
		Certifications: []profile.Certification{
			{Name: "Go Developer"},
		},
		Links: []profile.Link{
			{Label: "GitHub", URL: "https://example.com"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(httpServer.URL + "/api/profile/quality")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var report profile.QualityReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "ready" || report.Score != 100 || len(report.Checks) == 0 {
		t.Fatalf("quality report = %#v, want ready 100 with checks", report)
	}
}

func TestInterviewEndpoints(t *testing.T) {
	server, httpServer := newTestServer(t)
	appID := seedInterviewApplication(t, server)

	createBody := bytes.NewBufferString(`{
		"applicationId":` + strconvFormat(appID) + `,
		"stage":"technical",
		"scheduledAt":"2026-05-05T15:00:00Z",
		"durationMinutes":60,
		"location":"Zoom",
		"contacts":["Hiring Manager"],
		"notes":"Prepare systems examples."
	}`)
	resp, err := http.Post(httpServer.URL+"/api/interviews", "application/json", createBody)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var interview store.Interview
	if err := json.NewDecoder(resp.Body).Decode(&interview); err != nil {
		t.Fatal(err)
	}
	if interview.ID == 0 || interview.Stage != "technical" || interview.Status != "scheduled" {
		t.Fatalf("interview = %#v", interview)
	}

	taskBody := bytes.NewBufferString(`{"title":"Send thank-you note","dueAt":"2026-05-06T15:00:00Z"}`)
	resp, err = http.Post(httpServer.URL+"/api/interviews/"+strconvFormat(interview.ID)+"/tasks", "application/json", taskBody)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("task status = %d, want 201", resp.StatusCode)
	}

	statusBody := bytes.NewBufferString(`{"status":"completed","outcome":"advance","notes":"Went well."}`)
	resp, err = http.Post(httpServer.URL+"/api/interviews/"+strconvFormat(interview.ID)+"/status", "application/json", statusBody)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(httpServer.URL + "/api/interviews")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", resp.StatusCode)
	}
	var interviews []store.Interview
	if err := json.NewDecoder(resp.Body).Decode(&interviews); err != nil {
		t.Fatal(err)
	}
	if len(interviews) != 1 || len(interviews[0].Tasks) != 1 || interviews[0].Status != "completed" {
		t.Fatalf("interviews = %#v", interviews)
	}
}

func seedReviewMaterial(t *testing.T, server *Server) int64 {
	t.Helper()
	ctx := context.Background()
	profileID, err := server.store.UpsertFullCandidateProfile(ctx, profile.Profile{
		Name:            "Alex Example",
		Skills:          []string{"Go", "SQLite"},
		PreferredTitles: []string{"Backend Engineer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.store.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, events.Envelope[events.JobDiscoveredPayload]{
		EventID:        "review-event-1",
		EventType:      events.EventJobDiscovered,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		Source:         "test",
		CorrelationID:  "corr",
		IdempotencyKey: "test:review-job",
		Payload: events.JobDiscoveredPayload{
			Source:       "test",
			ExternalID:   "review-job",
			Title:        "Backend Engineer",
			Company:      "Acme",
			SourceURL:    "https://example.test/jobs/1",
			DiscoveredAt: time.Now().UTC(),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.store.MarkApplicationReady(ctx, result.JobID, profileID, 92); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "resume.md")
	if err := os.WriteFile(path, []byte("# Resume\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	docID, err := server.store.CreateDocument(ctx, store.CreateDocumentParams{
		Kind:      "tailored_resume",
		Format:    "markdown",
		Path:      path,
		SHA256:    "abc",
		SizeBytes: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	app, err := server.store.GetApplicationReadyContext(ctx, result.JobID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	materialID, err := server.store.CreateApplicationMaterial(ctx, store.CreateApplicationMaterialParams{
		ApplicationID:      app.ApplicationID,
		JobID:              result.JobID,
		CandidateProfileID: profileID,
		Kind:               "resume",
		DocumentID:         docID,
		Status:             "draft",
		SourceEventID:      "ready-event",
	})
	if err != nil {
		t.Fatal(err)
	}
	return materialID
}

func seedInterviewApplication(t *testing.T, server *Server) int64 {
	t.Helper()
	ctx := context.Background()
	profileID, err := server.store.UpsertFullCandidateProfile(ctx, profile.Profile{
		Name:            "Alex Example",
		Skills:          []string{"Go", "SQLite"},
		PreferredTitles: []string{"Backend Engineer"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := server.store.SaveDiscoveredJob(ctx, events.SubjectJobsDiscovered, events.Envelope[events.JobDiscoveredPayload]{
		EventID:        "interview-api-event-1",
		EventType:      events.EventJobDiscovered,
		EventVersion:   1,
		OccurredAt:     time.Now().UTC(),
		Source:         "test",
		CorrelationID:  "interview-api-corr",
		IdempotencyKey: "test:interview-api-job",
		Payload: events.JobDiscoveredPayload{
			Source:       "test",
			ExternalID:   "interview-api-job",
			Title:        "Backend Engineer",
			Company:      "Acme",
			SourceURL:    "https://example.test/jobs/interview-api",
			DiscoveredAt: time.Now().UTC(),
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.store.MarkApplicationReady(ctx, result.JobID, profileID, 92); err != nil {
		t.Fatal(err)
	}
	app, err := server.store.GetApplicationReadyContext(ctx, result.JobID, profileID)
	if err != nil {
		t.Fatal(err)
	}
	return app.ApplicationID
}

func strconvFormat(value int64) string {
	return strconv.FormatInt(value, 10)
}
