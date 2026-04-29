package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"hedhuntr/internal/config"
	"hedhuntr/internal/store"
)

type Server struct {
	cfg      config.API
	store    *store.Store
	hub      *Hub
	upgrader websocket.Upgrader
}

func NewServer(ctx context.Context, cfg config.API) (*Server, error) {
	st, err := store.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, err
	}
	server := &Server{
		cfg:   cfg,
		store: st,
		hub:   NewHub(),
	}
	server.upgrader = websocket.Upgrader{
		CheckOrigin: server.checkOrigin,
	}
	return server, nil
}

func (s *Server) Close() error {
	return s.store.Close()
}

func (s *Server) StartRealtime(ctx context.Context, logger *slog.Logger) error {
	return NewEventBridge(s.cfg, s.hub, logger).Start(ctx)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/jobs", s.handleJobs)
	mux.HandleFunc("GET /api/pipeline", s.handlePipeline)
	mux.HandleFunc("GET /api/profile", s.handleProfile)
	mux.HandleFunc("GET /api/resume-sources", s.handleResumeSources)
	mux.HandleFunc("GET /api/review/applications", s.handleReviewApplications)
	mux.HandleFunc("POST /api/review/materials/{id}/status", s.handleReviewMaterialStatus)
	mux.HandleFunc("POST /api/applications/{id}/approve-automation", s.handleApproveAutomation)
	mux.HandleFunc("GET /api/applications/{id}/packet", s.handleAutomationPacket)
	mux.HandleFunc("GET /api/notifications", s.handleNotifications)
	mux.HandleFunc("GET /api/workers", s.handleWorkers)
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	return s.cors(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.APIJobs(r.Context())
	writeResult(w, jobs, err)
}

func (s *Server) handlePipeline(w http.ResponseWriter, r *http.Request) {
	pipeline, err := s.store.APIPipeline(r.Context())
	writeResult(w, pipeline, err)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.FirstFullCandidateProfile(r.Context())
	writeResult(w, profile, err)
}

func (s *Server) handleResumeSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListResumeSources(r.Context())
	writeResult(w, sources, err)
}

func (s *Server) handleReviewApplications(w http.ResponseWriter, r *http.Request) {
	queue, err := s.store.APIReviewQueue(r.Context())
	writeResult(w, queue, err)
}

func (s *Server) handleReviewMaterialStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid material id"})
		return
	}
	var request struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	material, err := s.store.UpdateApplicationMaterialStatus(r.Context(), store.UpdateApplicationMaterialStatusParams{
		ID:     id,
		Status: request.Status,
		Notes:  request.Notes,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "applications",
		EventType:  "ApplicationMaterialStatusChanged",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"material_id": material.ID,
			"kind":        material.Kind,
			"status":      material.Status,
		},
	})
	writeJSON(w, http.StatusOK, material)
}

func (s *Server) handleApproveAutomation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid application id"})
		return
	}
	result, err := s.store.ApproveApplicationForAutomation(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "applications",
		EventType:  "AutomationRunRequested",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"application_id":    result.ApplicationID,
			"automation_run_id": result.AutomationRun.ID,
			"status":            result.AutomationRun.Status,
		},
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAutomationPacket(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid application id"})
		return
	}
	packet, err := s.store.AutomationPacket(r.Context(), id)
	writeResult(w, packet, err)
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	deliveries, err := s.store.APINotifications(r.Context())
	writeResult(w, deliveries, err)
}

func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := s.store.APIWorkers(r.Context())
	writeResult(w, workers, err)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := s.hub.Add(conn)
	client.send <- WSMessage{
		Type:       "ack",
		Topic:      "system",
		EventType:  "Connected",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"protocol_version": 1,
			"server_time":      time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	go client.writeLoop(s.cfg.WebSocket.PingInterval)
	client.readLoop()
}

func (s *Server) checkOrigin(r *http.Request) bool {
	if len(s.cfg.HTTP.AllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, allowed := range s.cfg.HTTP.AllowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	if len(s.cfg.HTTP.AllowedOrigins) == 0 {
		return true
	}
	for _, allowed := range s.cfg.HTTP.AllowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}

func writeResult(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintf(w, `{"error":%q}`, err.Error())
	}
}
