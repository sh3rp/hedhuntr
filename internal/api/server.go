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
	"hedhuntr/internal/events"
	"hedhuntr/internal/profile"
	"hedhuntr/internal/store"
)

type Server struct {
	cfg       config.API
	store     *store.Store
	hub       *Hub
	publisher EventPublisher
	upgrader  websocket.Upgrader
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
	if closer, ok := s.publisher.(interface{ Close() }); ok {
		closer.Close()
	}
	return s.store.Close()
}

func (s *Server) SetPublisher(publisher EventPublisher) {
	s.publisher = publisher
}

func (s *Server) StartPublisher(ctx context.Context, logger *slog.Logger) error {
	publisher := NewJetStreamPublisher(s.cfg, logger)
	if err := publisher.Start(ctx); err != nil {
		return err
	}
	s.publisher = publisher
	return nil
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
	mux.HandleFunc("GET /api/profile/quality", s.handleProfileQuality)
	mux.HandleFunc("PUT /api/profile", s.handleUpdateProfile)
	mux.HandleFunc("GET /api/resume-sources", s.handleResumeSources)
	mux.HandleFunc("GET /api/review/applications", s.handleReviewApplications)
	mux.HandleFunc("POST /api/review/materials/{id}/status", s.handleReviewMaterialStatus)
	mux.HandleFunc("POST /api/applications/{id}/approve-automation", s.handleApproveAutomation)
	mux.HandleFunc("GET /api/applications/{id}/packet", s.handleAutomationPacket)
	mux.HandleFunc("GET /api/automation/runs", s.handleAutomationRuns)
	mux.HandleFunc("POST /api/automation/runs/{id}/mark-submitted", s.handleAutomationMarkSubmitted)
	mux.HandleFunc("POST /api/automation/runs/{id}/fail", s.handleAutomationFail)
	mux.HandleFunc("POST /api/automation/runs/{id}/retry", s.handleAutomationRetry)
	mux.HandleFunc("GET /api/interviews", s.handleInterviews)
	mux.HandleFunc("POST /api/interviews", s.handleCreateInterview)
	mux.HandleFunc("POST /api/interviews/{id}/status", s.handleUpdateInterview)
	mux.HandleFunc("POST /api/interviews/{id}/tasks", s.handleCreateInterviewTask)
	mux.HandleFunc("POST /api/interview-tasks/{id}/status", s.handleUpdateInterviewTaskStatus)
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

func (s *Server) handleProfileQuality(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.FirstFullCandidateProfile(r.Context())
	if err != nil {
		writeResult(w, nil, err)
		return
	}
	if value == nil {
		writeJSON(w, http.StatusOK, profile.AssessQuality(profile.Profile{}))
		return
	}
	candidate, ok := value.(profile.Profile)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unexpected profile response"})
		return
	}
	writeJSON(w, http.StatusOK, profile.AssessQuality(candidate))
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var request profile.Profile
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	id, err := s.store.UpsertFullCandidateProfile(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updated, err := s.store.LoadFullCandidateProfile(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "profile",
		EventType:  "CandidateProfileUpdated",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"profile_id": updated.ID,
			"name":       updated.Name,
		},
	})
	writeJSON(w, http.StatusOK, updated)
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
	if material.Status == "regeneration_requested" {
		if err := s.publishMaterialRegenerationRequested(r.Context(), material.ID); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
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

func (s *Server) publishMaterialRegenerationRequested(ctx context.Context, materialID int64) error {
	if s.publisher == nil {
		return fmt.Errorf("api event publisher is unavailable")
	}
	context, err := s.store.MaterialRegenerationContext(ctx, materialID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	correlationID := events.StableID(
		"material-regeneration",
		fmt.Sprintf("%d", context.ApplicationID),
		fmt.Sprintf("%d", context.MaterialID),
		now.Format(time.RFC3339Nano),
	)
	idempotencyKey := events.StableID(
		"application-ready-regeneration",
		fmt.Sprintf("%d", context.ApplicationID),
		fmt.Sprintf("%d", context.MaterialID),
		now.Format(time.RFC3339Nano),
	)
	ready := events.Envelope[events.ApplicationReadyPayload]{
		EventID:        events.StableID("event", events.EventApplicationReady, "api-regeneration", idempotencyKey),
		EventType:      events.EventApplicationReady,
		EventVersion:   1,
		OccurredAt:     now,
		Source:         "api-regeneration",
		CorrelationID:  correlationID,
		IdempotencyKey: idempotencyKey,
		Payload: events.ApplicationReadyPayload{
			JobID:              context.JobID,
			CandidateProfileID: context.CandidateProfileID,
			MatchScore:         context.MatchScore,
			ReadyAt:            now,
		},
	}
	return s.publisher.Publish(ctx, events.SubjectApplicationsReady, ready)
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
	if err := s.publishAutomationHandoff(r.Context(), result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
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

func (s *Server) publishAutomationHandoff(ctx context.Context, result store.AutomationHandoffResult) error {
	if s.publisher == nil {
		return fmt.Errorf("api event publisher is unavailable")
	}
	now := time.Now().UTC()
	correlationID := events.StableID("automation-handoff", fmt.Sprintf("%d", result.ApplicationID), fmt.Sprintf("%d", result.AutomationRun.ID))
	approved := events.NewApplicationAutomationApproved("api", correlationID, events.ApplicationAutomationApprovedPayload{
		ApplicationID:         result.ApplicationID,
		JobID:                 result.AutomationRun.JobID,
		CandidateProfileID:    result.AutomationRun.CandidateProfileID,
		AutomationRunID:       result.AutomationRun.ID,
		ResumeMaterialID:      result.AutomationRun.ResumeMaterialID,
		CoverLetterMaterialID: result.AutomationRun.CoverLetterMaterialID,
		ApprovedAt:            now,
	})
	if err := s.publisher.Publish(ctx, events.SubjectApplicationsAutomationApproved, approved); err != nil {
		return err
	}
	return s.publishAutomationRunRequestedWithCorrelation(ctx, result.AutomationRun, correlationID, now)
}

func (s *Server) publishAutomationRunRequested(ctx context.Context, run store.AutomationRun) error {
	correlationID := events.StableID("automation-retry", fmt.Sprintf("%d", run.ApplicationID), fmt.Sprintf("%d", run.ID))
	return s.publishAutomationRunRequestedWithCorrelation(ctx, run, correlationID, time.Now().UTC())
}

func (s *Server) publishAutomationRunRequestedWithCorrelation(ctx context.Context, run store.AutomationRun, correlationID string, requestedAt time.Time) error {
	if s.publisher == nil {
		return fmt.Errorf("api event publisher is unavailable")
	}
	requested := events.NewAutomationRunRequested("api", correlationID, events.AutomationRunRequestedPayload{
		AutomationRunID:       run.ID,
		ApplicationID:         run.ApplicationID,
		JobID:                 run.JobID,
		CandidateProfileID:    run.CandidateProfileID,
		ResumeMaterialID:      run.ResumeMaterialID,
		CoverLetterMaterialID: run.CoverLetterMaterialID,
		RequestedAt:           requestedAt,
	})
	return s.publisher.Publish(ctx, events.SubjectAutomationRunRequested, requested)
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

func (s *Server) handleAutomationRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.APIAutomationRuns(r.Context())
	writeResult(w, runs, err)
}

func (s *Server) handleAutomationMarkSubmitted(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request struct {
		FinalURL string `json:"finalUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	run, err := s.store.MarkAutomationSubmitted(r.Context(), id, request.FinalURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastAutomationRun("AutomationRunSubmitted", run)
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleAutomationFail(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(request.Message) == "" {
		request.Message = "Marked failed by user."
	}
	run, err := s.store.MarkAutomationFailed(r.Context(), id, request.Message)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastAutomationRun("AutomationRunFailed", run)
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleAutomationRetry(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	run, err := s.store.RetryAutomationRun(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.publishAutomationRunRequested(r.Context(), run); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastAutomationRun("AutomationRunRequested", run)
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) broadcastAutomationRun(eventType string, run store.AutomationRun) {
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "automation",
		EventType:  eventType,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"automation_run_id": run.ID,
			"application_id":    run.ApplicationID,
			"status":            run.Status,
		},
	})
}

func (s *Server) handleInterviews(w http.ResponseWriter, r *http.Request) {
	interviews, err := s.store.ListInterviews(r.Context())
	writeResult(w, interviews, err)
}

func (s *Server) handleCreateInterview(w http.ResponseWriter, r *http.Request) {
	var request store.CreateInterviewParams
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	interview, err := s.store.CreateInterview(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastInterview("InterviewCreated", interview)
	writeJSON(w, http.StatusCreated, interview)
}

func (s *Server) handleUpdateInterview(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request store.UpdateInterviewParams
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	interview, err := s.store.UpdateInterview(r.Context(), id, request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.broadcastInterview("InterviewUpdated", interview)
	writeJSON(w, http.StatusOK, interview)
}

func (s *Server) handleCreateInterviewTask(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request store.CreateInterviewTaskParams
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	request.InterviewID = id
	task, err := s.store.CreateInterviewTask(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "interviews",
		EventType:  "InterviewTaskCreated",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"interview_id": task.InterviewID,
			"task_id":      task.ID,
			"title":        task.Title,
		},
	})
	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleUpdateInterviewTaskStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var request store.UpdateInterviewTaskStatusParams
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	task, err := s.store.UpdateInterviewTaskStatus(r.Context(), id, request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "interviews",
		EventType:  "InterviewTaskUpdated",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"interview_id": task.InterviewID,
			"task_id":      task.ID,
			"status":       task.Status,
		},
	})
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) broadcastInterview(eventType string, interview store.Interview) {
	s.hub.Broadcast(WSMessage{
		Type:       "event",
		Topic:      "interviews",
		EventType:  eventType,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: map[string]any{
			"interview_id":   interview.ID,
			"application_id": interview.ApplicationID,
			"job_id":         interview.JobID,
			"stage":          interview.Stage,
			"status":         interview.Status,
		},
	})
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
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

func parsePathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return id, nil
}
