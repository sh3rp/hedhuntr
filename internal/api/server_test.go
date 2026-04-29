package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"hedhuntr/internal/config"
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
