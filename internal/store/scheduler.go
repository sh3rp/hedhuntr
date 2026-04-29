package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type JobSource struct {
	ID              int64
	Name            string
	Type            string
	Enabled         bool
	Schedule        string
	IntervalSeconds int
	TimeoutSeconds  int
	LastRunAt       *time.Time
	LastSuccessAt   *time.Time
}

type UpsertJobSourceParams struct {
	Name            string
	Type            string
	Enabled         bool
	Schedule        string
	IntervalSeconds int
	TimeoutSeconds  int
}

type CompleteSourceRunParams struct {
	RunID           int64
	Status          string
	JobsSeen        int
	EventsPublished int
	Error           string
	FinishedAt      time.Time
}

func (s *Store) UpsertJobSource(ctx context.Context, params UpsertJobSourceParams) (int64, error) {
	enabled := 0
	if params.Enabled {
		enabled = 1
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO job_sources(name, type, enabled, schedule, interval_seconds, timeout_seconds)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
	type = excluded.type,
	enabled = excluded.enabled,
	schedule = excluded.schedule,
	interval_seconds = excluded.interval_seconds,
	timeout_seconds = excluded.timeout_seconds,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.Name,
		params.Type,
		enabled,
		params.Schedule,
		params.IntervalSeconds,
		params.TimeoutSeconds,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert job source: %w", err)
	}

	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM job_sources WHERE name = ?", params.Name).Scan(&id); err != nil {
		return 0, fmt.Errorf("load upserted job source: %w", err)
	}
	return id, nil
}

func (s *Store) ListEnabledJobSources(ctx context.Context) ([]JobSource, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, type, enabled, schedule, interval_seconds, timeout_seconds, last_run_at, last_success_at
FROM job_sources
WHERE enabled = 1
ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list enabled job sources: %w", err)
	}
	defer rows.Close()

	var sources []JobSource
	for rows.Next() {
		source, err := scanJobSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled job sources: %w", err)
	}
	return sources, nil
}

func (s *Store) BeginSourceRun(ctx context.Context, source JobSource, startedAt time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
INSERT INTO job_source_runs(source_id, source_name, status, started_at)
VALUES(?, ?, 'running', ?)`,
		source.ID,
		source.Name,
		startedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("insert source run: %w", err)
	}
	runID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get source run id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE job_sources
SET last_run_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		startedAt.UTC().Format(time.RFC3339Nano),
		source.ID,
	); err != nil {
		return 0, fmt.Errorf("update source last_run_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return runID, nil
}

func (s *Store) CompleteSourceRun(ctx context.Context, params CompleteSourceRunParams) error {
	if params.FinishedAt.IsZero() {
		params.FinishedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var sourceID int64
	var startedAtRaw string
	if err := tx.QueryRowContext(ctx, "SELECT source_id, started_at FROM job_source_runs WHERE id = ?", params.RunID).Scan(&sourceID, &startedAtRaw); err != nil {
		return fmt.Errorf("load source run: %w", err)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, startedAtRaw)
	if err != nil {
		return fmt.Errorf("parse run started_at: %w", err)
	}
	durationMS := params.FinishedAt.Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE job_source_runs
SET status = ?,
	finished_at = ?,
	duration_ms = ?,
	jobs_seen = ?,
	events_published = ?,
	error = ?
WHERE id = ?`,
		params.Status,
		params.FinishedAt.UTC().Format(time.RFC3339Nano),
		durationMS,
		params.JobsSeen,
		params.EventsPublished,
		nullIfEmpty(params.Error),
		params.RunID,
	); err != nil {
		return fmt.Errorf("complete source run: %w", err)
	}

	if params.Status == "succeeded" {
		if _, err := tx.ExecContext(ctx, `
UPDATE job_sources
SET last_success_at = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
			params.FinishedAt.UTC().Format(time.RFC3339Nano),
			sourceID,
		); err != nil {
			return fmt.Errorf("update source last_success_at: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) CountSourceRuns(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM job_source_runs").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

type jobSourceScanner interface {
	Scan(dest ...any) error
}

func scanJobSource(scanner jobSourceScanner) (JobSource, error) {
	var source JobSource
	var enabled int
	var lastRunAt sql.NullString
	var lastSuccessAt sql.NullString
	if err := scanner.Scan(
		&source.ID,
		&source.Name,
		&source.Type,
		&enabled,
		&source.Schedule,
		&source.IntervalSeconds,
		&source.TimeoutSeconds,
		&lastRunAt,
		&lastSuccessAt,
	); err != nil {
		return JobSource{}, fmt.Errorf("scan job source: %w", err)
	}
	source.Enabled = enabled == 1
	source.LastRunAt = parseOptionalTime(lastRunAt)
	source.LastSuccessAt = parseOptionalTime(lastSuccessAt)
	return source, nil
}

func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil
	}
	return &parsed
}
