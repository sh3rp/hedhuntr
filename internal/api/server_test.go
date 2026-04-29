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
	httpServer := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		httpServer.Close()
		server.Close()
	})
	return server, httpServer
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

func strconvFormat(value int64) string {
	return strconv.FormatInt(value, 10)
}
