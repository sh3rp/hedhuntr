package resumetuningworker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"hedhuntr/internal/broker"
	"hedhuntr/internal/config"
	"hedhuntr/internal/document"
	"hedhuntr/internal/events"
	"hedhuntr/internal/resumetuner"
	"hedhuntr/internal/store"
)

type Service struct {
	cfg    config.ResumeTuningWorker
	logger *slog.Logger
	nc     *nats.Conn
	js     nats.JetStreamContext
	sub    *nats.Subscription
	store  *store.Store
}

type RunResult struct {
	MessagesProcessed int
	DraftsCreated     int
}

func NewService(ctx context.Context, cfg config.ResumeTuningWorker, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.Default()
	}

	st, err := store.Open(ctx, cfg.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("initialize jetstream: %w", err)
	}
	if err := broker.EnsureJobsStream(js, cfg.NATS.Stream); err != nil {
		st.Close()
		nc.Close()
		return nil, err
	}

	sub, err := js.PullSubscribe(
		events.SubjectApplicationsReady,
		cfg.Consumer.Durable,
		nats.BindStream(cfg.NATS.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		st.Close()
		nc.Close()
		return nil, fmt.Errorf("subscribe to %s: %w", events.SubjectApplicationsReady, err)
	}

	return &Service{cfg: cfg, logger: logger, nc: nc, js: js, sub: sub, store: st}, nil
}

func (s *Service) Close() {
	if s.store != nil {
		s.store.Close()
	}
	if s.nc != nil {
		s.nc.Drain()
		s.nc.Close()
	}
}

func (s *Service) Run(ctx context.Context, maxMessages int) (RunResult, error) {
	var result RunResult

	for maxMessages <= 0 || result.MessagesProcessed < maxMessages {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		remaining := s.cfg.Consumer.BatchSize
		if maxMessages > 0 {
			left := maxMessages - result.MessagesProcessed
			if left < remaining {
				remaining = left
			}
		}

		messages, err := s.sub.Fetch(remaining, nats.Context(ctx), nats.MaxWait(s.cfg.Consumer.FetchWait))
		if err != nil {
			if err == nats.ErrTimeout && maxMessages > 0 {
				return result, nil
			}
			if err == nats.ErrTimeout {
				continue
			}
			return result, fmt.Errorf("fetch messages: %w", err)
		}

		for _, msg := range messages {
			if err := s.processMessage(ctx, msg); err != nil {
				s.logger.Error("process application ready event", "error", err, "subject", msg.Subject)
				msg.Nak()
				continue
			}
			if err := msg.Ack(); err != nil {
				return result, fmt.Errorf("ack message: %w", err)
			}
			result.MessagesProcessed++
			result.DraftsCreated++
		}
	}

	return result, nil
}

func (s *Service) processMessage(ctx context.Context, msg *nats.Msg) error {
	var envelope events.Envelope[events.ApplicationReadyPayload]
	if err := json.Unmarshal(msg.Data, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if envelope.EventType != events.EventApplicationReady {
		return fmt.Errorf("unexpected event type %q", envelope.EventType)
	}

	app, err := s.store.GetApplicationReadyContext(ctx, envelope.Payload.JobID, envelope.Payload.CandidateProfileID)
	if err != nil {
		return err
	}
	profile, err := s.store.LoadFullCandidateProfile(ctx, envelope.Payload.CandidateProfileID)
	if err != nil {
		return err
	}

	source, baseResume, err := s.resumeSource(ctx, envelope.Payload.CandidateProfileID)
	if err != nil {
		return err
	}

	output := resumetuner.Tune(resumetuner.Input{
		Profile:           profile,
		Application:       app,
		BaseResumeName:    source.Name,
		BaseResumeContent: baseResume,
		MaxHighlights:     s.cfg.Tuning.MaxHighlights,
	})

	resumeDoc, err := document.StoreContent(
		s.cfg.Documents.RootDir,
		"tailored-resumes",
		fmt.Sprintf("%s-%s-resume.md", app.Company, app.JobTitle),
		[]byte(output.ResumeMarkdown),
	)
	if err != nil {
		return fmt.Errorf("store tailored resume: %w", err)
	}
	resumeDocID, err := s.store.CreateDocument(ctx, store.CreateDocumentParams{
		Kind:      "tailored_resume",
		Format:    "markdown",
		Path:      resumeDoc.Path,
		SHA256:    resumeDoc.SHA256,
		SizeBytes: resumeDoc.SizeBytes,
	})
	if err != nil {
		return err
	}
	resumeVersionID, err := s.store.CreateResumeVersion(ctx, store.CreateResumeVersionParams{
		ResumeSourceID: source.ID,
		JobID:          app.JobID,
		DocumentID:     resumeDocID,
		Status:         "draft",
		Notes:          strings.Join(output.Notes, "\n"),
	})
	if err != nil {
		return err
	}
	if _, err := s.store.CreateApplicationMaterial(ctx, store.CreateApplicationMaterialParams{
		ApplicationID:      app.ApplicationID,
		JobID:              app.JobID,
		CandidateProfileID: app.CandidateProfileID,
		Kind:               "resume",
		DocumentID:         resumeDocID,
		Status:             "draft",
		Notes:              fmt.Sprintf("Resume version %d. Human approval required.", resumeVersionID),
		SourceEventID:      envelope.EventID,
	}); err != nil {
		return err
	}

	coverLetterDoc, err := document.StoreContent(
		s.cfg.Documents.RootDir,
		"cover-letters",
		fmt.Sprintf("%s-%s-cover-letter.md", app.Company, app.JobTitle),
		[]byte(output.CoverLetterMarkdown),
	)
	if err != nil {
		return fmt.Errorf("store cover letter: %w", err)
	}
	coverLetterDocID, err := s.store.CreateDocument(ctx, store.CreateDocumentParams{
		Kind:      "cover_letter",
		Format:    "markdown",
		Path:      coverLetterDoc.Path,
		SHA256:    coverLetterDoc.SHA256,
		SizeBytes: coverLetterDoc.SizeBytes,
	})
	if err != nil {
		return err
	}
	coverLetterID, err := s.store.CreateApplicationMaterial(ctx, store.CreateApplicationMaterialParams{
		ApplicationID:      app.ApplicationID,
		JobID:              app.JobID,
		CandidateProfileID: app.CandidateProfileID,
		Kind:               "cover_letter",
		DocumentID:         coverLetterDocID,
		Status:             "draft",
		Notes:              "Human approval required.",
		SourceEventID:      envelope.EventID,
	})
	if err != nil {
		return err
	}

	draftedAt := time.Now().UTC()
	drafted := events.NewApplicationMaterialsDrafted(envelope.Source, envelope.CorrelationID, events.ApplicationMaterialsDraftedPayload{
		JobID:              app.JobID,
		ApplicationID:      app.ApplicationID,
		CandidateProfileID: app.CandidateProfileID,
		ResumeSourceID:     source.ID,
		ResumeVersionID:    resumeVersionID,
		ResumeDocumentID:   resumeDocID,
		CoverLetterID:      coverLetterID,
		CoverLetterDocID:   coverLetterDocID,
		Status:             "draft",
		DraftedAt:          draftedAt,
	})
	if err := s.publish(ctx, events.SubjectApplicationsMaterialsDrafted, drafted); err != nil {
		return fmt.Errorf("publish applications.materials.drafted: %w", err)
	}

	s.logger.Info(
		"drafted application materials",
		"job_id", app.JobID,
		"profile_id", app.CandidateProfileID,
		"resume_version_id", resumeVersionID,
		"resume_path", filepath.Clean(resumeDoc.Path),
		"cover_letter_path", filepath.Clean(coverLetterDoc.Path),
	)
	return nil
}

func (s *Service) resumeSource(ctx context.Context, candidateProfileID int64) (store.ResumeSource, []byte, error) {
	if s.cfg.Tuning.ResumeSourceID > 0 {
		return s.store.LoadResumeSourceContent(ctx, s.cfg.Tuning.ResumeSourceID)
	}
	return s.store.DefaultResumeSource(ctx, candidateProfileID)
}

func (s *Service) publish(ctx context.Context, subject string, envelope any) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal %s envelope: %w", subject, err)
	}

	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.NATS.PublishTimeout)
	defer cancel()

	_, err = s.js.Publish(subject, data, nats.Context(publishCtx), nats.MsgId(idempotencyKeyOf(envelope)))
	return err
}

func idempotencyKeyOf(envelope any) string {
	data, err := json.Marshal(envelope)
	if err != nil {
		return events.StableID("unknown-envelope")
	}
	var decoded struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.IdempotencyKey == "" {
		return events.StableID(string(data))
	}
	return decoded.IdempotencyKey
}
