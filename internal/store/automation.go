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
	RequestedAt           string `json:"requestedAt"`
	UpdatedAt             string `json:"updatedAt"`
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
	Resume      APIReviewMaterial  `json:"resume"`
	CoverLetter *APIReviewMaterial `json:"coverLetter,omitempty"`
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
	err := s.db.QueryRowContext(ctx, `
SELECT id, application_id, job_id, candidate_profile_id, status, resume_material_id, cover_letter_material_id,
	requested_at, updated_at
FROM automation_runs
WHERE id = ?`, runID).Scan(
		&run.ID,
		&run.ApplicationID,
		&run.JobID,
		&run.CandidateProfileID,
		&run.Status,
		&run.ResumeMaterialID,
		&coverID,
		&run.RequestedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return AutomationRun{}, err
	}
	if coverID.Valid {
		value := coverID.Int64
		run.CoverLetterMaterialID = &value
	}
	return run, nil
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
	if coverID.Valid {
		cover, err := s.APIReviewMaterial(ctx, coverID.Int64)
		if err != nil {
			return AutomationPacket{}, err
		}
		packet.Materials.CoverLetter = &cover
	}
	return packet, nil
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
