package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type AutomationRun struct {
	ID                    int64  `json:"id"`
	ApplicationID         int64  `json:"applicationId"`
	JobID                 int64  `json:"jobId"`
	CandidateProfileID    int64  `json:"candidateProfileId"`
	Status                string `json:"status"`
	ResumeMaterialID      int64  `json:"resumeMaterialId"`
	CoverLetterMaterialID *int64 `json:"coverLetterMaterialId,omitempty"`
	FinalURL              string `json:"finalUrl,omitempty"`
	Error                 string `json:"error,omitempty"`
	RequestedAt           string `json:"requestedAt"`
	StartedAt             string `json:"startedAt,omitempty"`
	ReviewRequiredAt      string `json:"reviewRequiredAt,omitempty"`
	FinishedAt            string `json:"finishedAt,omitempty"`
	UpdatedAt             string `json:"updatedAt"`
}

type APIAutomationRun struct {
	AutomationRun
	JobTitle string          `json:"jobTitle"`
	Company  string          `json:"company"`
	Location string          `json:"location"`
	Logs     []AutomationLog `json:"logs"`
}

type AutomationLog struct {
	ID        int64          `json:"id"`
	RunID     int64          `json:"runId"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	CreatedAt string         `json:"createdAt"`
}

type AutomationPacket struct {
	ApplicationID    int64                     `json:"applicationId"`
	AutomationRunID  int64                     `json:"automationRunId"`
	Status           string                    `json:"status"`
	Job              AutomationPacketJob       `json:"job"`
	CandidateProfile int64                     `json:"candidateProfileId"`
	Materials        AutomationPacketMaterials `json:"materials"`
}

type AutomationPacketJob struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Company        string `json:"company"`
	Location       string `json:"location"`
	ApplicationURL string `json:"applicationUrl"`
	SourceURL      string `json:"sourceUrl"`
}

type AutomationPacketMaterials struct {
	Resume      APIReviewMaterial   `json:"resume"`
	CoverLetter *APIReviewMaterial  `json:"coverLetter,omitempty"`
	Answers     []APIReviewMaterial `json:"answers"`
}

type AutomationHandoffResult struct {
	ApplicationID int64            `json:"applicationId"`
	AutomationRun AutomationRun    `json:"automationRun"`
	Packet        AutomationPacket `json:"packet"`
}

type AutomationLogParams struct {
	RunID   int64
	Level   string
	Message string
	Details map[string]any
}

func (s *Store) ApproveApplicationForAutomation(ctx context.Context, applicationID int64) (AutomationHandoffResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AutomationHandoffResult{}, err
	}
	defer tx.Rollback()

	app, err := applicationForAutomationTx(ctx, tx, applicationID)
	if err != nil {
		return AutomationHandoffResult{}, err
	}
	resumeID, coverID, err := approvedMaterialIDsTx(ctx, tx, applicationID)
	if err != nil {
		return AutomationHandoffResult{}, err
	}

	var existingRunID int64
	err = tx.QueryRowContext(ctx, `
SELECT id FROM automation_runs
WHERE application_id = ? AND status IN ('requested', 'started', 'review_required')
ORDER BY id DESC LIMIT 1`, applicationID).Scan(&existingRunID)
	if err != nil && err != sql.ErrNoRows {
		return AutomationHandoffResult{}, err
	}

	runID := existingRunID
	if runID == 0 {
		result, err := tx.ExecContext(ctx, `
INSERT INTO automation_runs(application_id, job_id, candidate_profile_id, status, resume_material_id, cover_letter_material_id)
VALUES(?, ?, ?, 'requested', ?, ?)`,
			applicationID,
			app.JobID,
			app.CandidateProfileID,
			resumeID,
			nullInt64(coverID),
		)
		if err != nil {
			return AutomationHandoffResult{}, fmt.Errorf("create automation run: %w", err)
		}
		runID, err = result.LastInsertId()
		if err != nil {
			return AutomationHandoffResult{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE applications
SET status = 'approved_for_automation',
	selected_resume_material_id = ?,
	selected_cover_letter_material_id = ?,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		resumeID,
		nullInt64(coverID),
		applicationID,
	); err != nil {
		return AutomationHandoffResult{}, fmt.Errorf("update application automation status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return AutomationHandoffResult{}, err
	}
	packet, err := s.AutomationPacket(ctx, applicationID)
	if err != nil {
		return AutomationHandoffResult{}, err
	}
	run, err := s.AutomationRun(ctx, runID)
	if err != nil {
		return AutomationHandoffResult{}, err
	}
	return AutomationHandoffResult{ApplicationID: applicationID, AutomationRun: run, Packet: packet}, nil
}

func (s *Store) AutomationRun(ctx context.Context, runID int64) (AutomationRun, error) {
	var run AutomationRun
	var coverID sql.NullInt64
	var errorText, startedAt, reviewRequiredAt, finishedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
SELECT id, application_id, job_id, candidate_profile_id, status, resume_material_id, cover_letter_material_id,
	COALESCE(final_url, ''), error, requested_at, started_at, review_required_at, finished_at, updated_at
FROM automation_runs
WHERE id = ?`, runID).Scan(
		&run.ID,
		&run.ApplicationID,
		&run.JobID,
		&run.CandidateProfileID,
		&run.Status,
		&run.ResumeMaterialID,
		&coverID,
		&run.FinalURL,
		&errorText,
		&run.RequestedAt,
		&startedAt,
		&reviewRequiredAt,
		&finishedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return AutomationRun{}, err
	}
	if coverID.Valid {
		value := coverID.Int64
		run.CoverLetterMaterialID = &value
	}
	run.Error = errorText.String
	run.StartedAt = startedAt.String
	run.ReviewRequiredAt = reviewRequiredAt.String
	run.FinishedAt = finishedAt.String
	return run, nil
}

func (s *Store) APIAutomationRuns(ctx context.Context) ([]APIAutomationRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT ar.id, ar.application_id, ar.job_id, ar.candidate_profile_id, ar.status,
	ar.resume_material_id, ar.cover_letter_material_id, COALESCE(ar.final_url, ''), ar.error,
	ar.requested_at, ar.started_at, ar.review_required_at, ar.finished_at, ar.updated_at,
	j.title, j.company, COALESCE(j.location, '')
FROM automation_runs ar
JOIN jobs j ON j.id = ar.job_id
ORDER BY ar.updated_at DESC, ar.id DESC
LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("query automation runs: %w", err)
	}
	defer rows.Close()

	runs := []APIAutomationRun{}
	for rows.Next() {
		var run APIAutomationRun
		var coverID sql.NullInt64
		var errorText, startedAt, reviewRequiredAt, finishedAt sql.NullString
		if err := rows.Scan(
			&run.ID,
			&run.ApplicationID,
			&run.JobID,
			&run.CandidateProfileID,
			&run.Status,
			&run.ResumeMaterialID,
			&coverID,
			&run.FinalURL,
			&errorText,
			&run.RequestedAt,
			&startedAt,
			&reviewRequiredAt,
			&finishedAt,
			&run.UpdatedAt,
			&run.JobTitle,
			&run.Company,
			&run.Location,
		); err != nil {
			return nil, err
		}
		if coverID.Valid {
			value := coverID.Int64
			run.CoverLetterMaterialID = &value
		}
		run.Error = errorText.String
		run.StartedAt = startedAt.String
		run.ReviewRequiredAt = reviewRequiredAt.String
		run.FinishedAt = finishedAt.String
		logs, err := s.AutomationLogs(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		run.Logs = logs
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) AutomationLogs(ctx context.Context, runID int64) ([]AutomationLog, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, automation_run_id, level, message, details_json, created_at
FROM automation_logs
WHERE automation_run_id = ?
ORDER BY id DESC
LIMIT 100`, runID)
	if err != nil {
		return nil, fmt.Errorf("query automation logs: %w", err)
	}
	defer rows.Close()

	logs := []AutomationLog{}
	for rows.Next() {
		var log AutomationLog
		var detailsJSON string
		if err := rows.Scan(&log.ID, &log.RunID, &log.Level, &log.Message, &detailsJSON, &log.CreatedAt); err != nil {
			return nil, err
		}
		log.Details = map[string]any{}
		json.Unmarshal([]byte(detailsJSON), &log.Details)
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) StartAutomationRun(ctx context.Context, runID int64) (AutomationRun, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE automation_runs
SET status = 'started',
	started_at = COALESCE(started_at, strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status IN ('requested', 'started')`, runID)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("start automation run: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AutomationRun{}, err
	}
	if affected == 0 {
		return AutomationRun{}, fmt.Errorf("automation run %d is not startable", runID)
	}
	return s.AutomationRun(ctx, runID)
}

func (s *Store) MarkAutomationReviewRequired(ctx context.Context, runID int64, finalURL string) (AutomationRun, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE automation_runs
SET status = 'review_required',
	final_url = ?,
	review_required_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status IN ('requested', 'started', 'review_required')`, finalURL, runID)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("mark automation review required: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AutomationRun{}, err
	}
	if affected == 0 {
		return AutomationRun{}, fmt.Errorf("automation run %d cannot be marked review_required", runID)
	}
	return s.AutomationRun(ctx, runID)
}

func (s *Store) MarkAutomationFailed(ctx context.Context, runID int64, message string) (AutomationRun, error) {
	_, err := s.db.ExecContext(ctx, `
UPDATE automation_runs
SET status = 'failed',
	error = ?,
	finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, message, runID)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("mark automation failed: %w", err)
	}
	return s.AutomationRun(ctx, runID)
}

func (s *Store) MarkAutomationSubmitted(ctx context.Context, runID int64, finalURL string) (AutomationRun, error) {
	run, err := s.AutomationRun(ctx, runID)
	if err != nil {
		return AutomationRun{}, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE automation_runs
SET status = 'submitted',
	final_url = COALESCE(NULLIF(?, ''), final_url),
	finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ? AND status IN ('review_required', 'started')`, finalURL, runID)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("mark automation submitted: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AutomationRun{}, err
	}
	if affected == 0 {
		return AutomationRun{}, fmt.Errorf("automation run %d cannot be marked submitted", runID)
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE applications
SET status = 'submitted',
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, run.ApplicationID)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("update application submitted status: %w", err)
	}
	return s.AutomationRun(ctx, runID)
}

func (s *Store) RetryAutomationRun(ctx context.Context, runID int64) (AutomationRun, error) {
	run, err := s.AutomationRun(ctx, runID)
	if err != nil {
		return AutomationRun{}, err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO automation_runs(application_id, job_id, candidate_profile_id, status, resume_material_id, cover_letter_material_id)
VALUES(?, ?, ?, 'requested', ?, ?)`,
		run.ApplicationID,
		run.JobID,
		run.CandidateProfileID,
		run.ResumeMaterialID,
		nullInt64Ptr(run.CoverLetterMaterialID),
	)
	if err != nil {
		return AutomationRun{}, fmt.Errorf("retry automation run: %w", err)
	}
	newID, err := result.LastInsertId()
	if err != nil {
		return AutomationRun{}, err
	}
	return s.AutomationRun(ctx, newID)
}

func (s *Store) AddAutomationLog(ctx context.Context, params AutomationLogParams) error {
	if params.Level == "" {
		params.Level = "info"
	}
	details := "{}"
	if params.Details != nil {
		encoded, err := json.Marshal(params.Details)
		if err != nil {
			return fmt.Errorf("marshal automation log details: %w", err)
		}
		details = string(encoded)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO automation_logs(automation_run_id, level, message, details_json)
VALUES(?, ?, ?, ?)`, params.RunID, params.Level, params.Message, details)
	if err != nil {
		return fmt.Errorf("insert automation log: %w", err)
	}
	return nil
}

func (s *Store) AutomationPacket(ctx context.Context, applicationID int64) (AutomationPacket, error) {
	var packet AutomationPacket
	var resumeID sql.NullInt64
	var coverID sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT a.id, COALESCE(ar.id, 0), a.status, a.candidate_profile_id,
	j.id, j.title, j.company, COALESCE(j.location, ''), COALESCE(j.application_url, ''), j.source_url,
	a.selected_resume_material_id, a.selected_cover_letter_material_id
FROM applications a
JOIN jobs j ON j.id = a.job_id
LEFT JOIN automation_runs ar ON ar.application_id = a.id AND ar.status IN ('requested', 'started', 'review_required')
WHERE a.id = ?
ORDER BY ar.id DESC
LIMIT 1`, applicationID).Scan(
		&packet.ApplicationID,
		&packet.AutomationRunID,
		&packet.Status,
		&packet.CandidateProfile,
		&packet.Job.ID,
		&packet.Job.Title,
		&packet.Job.Company,
		&packet.Job.Location,
		&packet.Job.ApplicationURL,
		&packet.Job.SourceURL,
		&resumeID,
		&coverID,
	)
	if err == sql.ErrNoRows {
		return AutomationPacket{}, fmt.Errorf("application %d not found", applicationID)
	}
	if err != nil {
		return AutomationPacket{}, fmt.Errorf("load automation packet: %w", err)
	}
	if !resumeID.Valid || resumeID.Int64 == 0 {
		return AutomationPacket{}, fmt.Errorf("application %d has no selected approved resume", applicationID)
	}
	resume, err := s.APIReviewMaterial(ctx, resumeID.Int64)
	if err != nil {
		return AutomationPacket{}, err
	}
	packet.Materials.Resume = resume
	packet.Materials.Answers = []APIReviewMaterial{}
	if coverID.Valid {
		cover, err := s.APIReviewMaterial(ctx, coverID.Int64)
		if err != nil {
			return AutomationPacket{}, err
		}
		packet.Materials.CoverLetter = &cover
	}
	answers, err := s.approvedApplicationAnswerMaterials(ctx, applicationID)
	if err != nil {
		return AutomationPacket{}, err
	}
	packet.Materials.Answers = answers
	return packet, nil
}

func (s *Store) approvedApplicationAnswerMaterials(ctx context.Context, applicationID int64) ([]APIReviewMaterial, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id
FROM application_materials
WHERE application_id = ? AND kind = 'application_answers' AND status = 'approved'
ORDER BY id`, applicationID)
	if err != nil {
		return nil, fmt.Errorf("query approved application answers: %w", err)
	}
	defer rows.Close()
	answers := []APIReviewMaterial{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		material, err := s.APIReviewMaterial(ctx, id)
		if err != nil {
			return nil, err
		}
		answers = append(answers, material)
	}
	return answers, rows.Err()
}

type applicationAutomationRow struct {
	JobID              int64
	CandidateProfileID int64
	Status             string
}

func applicationForAutomationTx(ctx context.Context, tx *sql.Tx, applicationID int64) (applicationAutomationRow, error) {
	var app applicationAutomationRow
	err := tx.QueryRowContext(ctx, `
SELECT job_id, candidate_profile_id, status
FROM applications
WHERE id = ?`, applicationID).Scan(&app.JobID, &app.CandidateProfileID, &app.Status)
	if err == sql.ErrNoRows {
		return applicationAutomationRow{}, fmt.Errorf("application %d not found", applicationID)
	}
	if err != nil {
		return applicationAutomationRow{}, err
	}
	if app.Status == "submitted" {
		return applicationAutomationRow{}, fmt.Errorf("application %d is already submitted", applicationID)
	}
	return app, nil
}

func approvedMaterialIDsTx(ctx context.Context, tx *sql.Tx, applicationID int64) (int64, int64, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, kind, status
FROM application_materials
WHERE application_id = ?`, applicationID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var resumeID int64
	var coverID int64
	for rows.Next() {
		var id int64
		var kind, status string
		if err := rows.Scan(&id, &kind, &status); err != nil {
			return 0, 0, err
		}
		if status != "approved" {
			return 0, 0, fmt.Errorf("%s material %d is %s, not approved", kind, id, status)
		}
		switch kind {
		case "resume":
			resumeID = id
		case "cover_letter":
			coverID = id
		}
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	if resumeID == 0 {
		return 0, 0, fmt.Errorf("application %d has no approved resume material", applicationID)
	}
	return resumeID, coverID, nil
}

func nullInt64Ptr(value *int64) any {
	if value == nil || *value <= 0 {
		return nil
	}
	return *value
}
