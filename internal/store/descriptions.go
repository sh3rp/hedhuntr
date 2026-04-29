package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type JobForDescriptionFetch struct {
	ID             int64
	Source         string
	Title          string
	Company        string
	Location       string
	SourceURL      string
	ApplicationURL string
}

type UpdateFetchedDescriptionParams struct {
	JobID     int64
	RawText   string
	RawHTML   string
	FetchedAt time.Time
}

func (s *Store) GetJobForDescriptionFetch(ctx context.Context, jobID int64) (JobForDescriptionFetch, error) {
	var job JobForDescriptionFetch
	err := s.db.QueryRowContext(ctx, `
SELECT id, source, title, company, location, source_url, COALESCE(application_url, '')
FROM jobs
WHERE id = ?`, jobID).Scan(
		&job.ID,
		&job.Source,
		&job.Title,
		&job.Company,
		&job.Location,
		&job.SourceURL,
		&job.ApplicationURL,
	)
	if err == sql.ErrNoRows {
		return JobForDescriptionFetch{}, fmt.Errorf("job %d not found", jobID)
	}
	if err != nil {
		return JobForDescriptionFetch{}, fmt.Errorf("load job for description fetch: %w", err)
	}
	return job, nil
}

func (s *Store) UpdateFetchedDescription(ctx context.Context, params UpdateFetchedDescriptionParams) error {
	if params.FetchedAt.IsZero() {
		params.FetchedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO job_descriptions(job_id, raw_text, raw_html, fetched_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	raw_text = excluded.raw_text,
	raw_html = excluded.raw_html,
	fetched_at = excluded.fetched_at,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.JobID,
		params.RawText,
		nullIfEmpty(params.RawHTML),
		params.FetchedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("update fetched description: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET status = 'description_fetched',
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, params.JobID); err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	var title, company, location, skills string
	if err := tx.QueryRowContext(ctx, `
SELECT j.title, j.company, j.location, COALESCE(d.detected_skills_json, '[]')
FROM jobs j
LEFT JOIN job_descriptions d ON d.job_id = j.id
WHERE j.id = ?`, params.JobID).Scan(&title, &company, &location, &skills); err != nil {
		return fmt.Errorf("load job search fields: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs_fts WHERE rowid = ?", params.JobID); err != nil {
		return fmt.Errorf("delete fts row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO jobs_fts(rowid, title, company, location, description, skills)
VALUES(?, ?, ?, ?, ?, ?)`,
		params.JobID,
		title,
		company,
		location,
		params.RawText,
		strings.Trim(skills, "[]"),
	); err != nil {
		return fmt.Errorf("insert fts row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetJobDescriptionText(ctx context.Context, jobID int64) (string, string, error) {
	var rawText string
	var status string
	err := s.db.QueryRowContext(ctx, `
SELECT COALESCE(d.raw_text, ''), j.status
FROM jobs j
LEFT JOIN job_descriptions d ON d.job_id = j.id
WHERE j.id = ?`, jobID).Scan(&rawText, &status)
	if err != nil {
		return "", "", err
	}
	return rawText, status, nil
}
