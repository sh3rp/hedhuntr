package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"hedhuntr/internal/events"
)

type Store struct {
	db *sql.DB
}

type SaveJobResult struct {
	JobID   int64
	Created bool
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveDiscoveredJob(ctx context.Context, subject string, envelope events.Envelope[events.JobDiscoveredPayload], rawPayload []byte) (SaveJobResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SaveJobResult{}, err
	}
	defer tx.Rollback()

	var existingEventID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM job_events WHERE event_id = ?", envelope.EventID).Scan(&existingEventID)
	if err == nil {
		jobID, findErr := findJobIDByIdempotencyKey(ctx, tx, envelope.IdempotencyKey)
		return SaveJobResult{JobID: jobID, Created: false}, findErr
	}
	if err != sql.ErrNoRows {
		return SaveJobResult{}, fmt.Errorf("check job event: %w", err)
	}

	payloadJSON := string(rawPayload)
	if payloadJSON == "" {
		encoded, err := json.Marshal(envelope.Payload)
		if err != nil {
			return SaveJobResult{}, fmt.Errorf("marshal payload: %w", err)
		}
		payloadJSON = string(encoded)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO job_events(
	event_id,
	event_type,
	event_version,
	subject,
	source,
	correlation_id,
	idempotency_key,
	payload_json
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		envelope.EventID,
		envelope.EventType,
		envelope.EventVersion,
		subject,
		envelope.Source,
		envelope.CorrelationID,
		envelope.IdempotencyKey,
		payloadJSON,
	); err != nil {
		return SaveJobResult{}, fmt.Errorf("insert job event: %w", err)
	}

	jobID, created, err := upsertJob(ctx, tx, envelope)
	if err != nil {
		return SaveJobResult{}, err
	}
	if err := upsertDescription(ctx, tx, jobID, envelope.Payload); err != nil {
		return SaveJobResult{}, err
	}
	if err := replaceSearchIndex(ctx, tx, jobID, envelope.Payload); err != nil {
		return SaveJobResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return SaveJobResult{}, err
	}

	return SaveJobResult{JobID: jobID, Created: created}, nil
}

func upsertJob(ctx context.Context, tx *sql.Tx, envelope events.Envelope[events.JobDiscoveredPayload]) (int64, bool, error) {
	job := envelope.Payload
	sourceExternalKey := ""
	if strings.TrimSpace(job.ExternalID) != "" {
		sourceExternalKey = strings.ToLower(strings.TrimSpace(job.Source)) + ":" + strings.ToLower(strings.TrimSpace(job.ExternalID))
	}
	canonicalURL := canonicalJobURL(job.ApplicationURL)
	if canonicalURL == "" {
		canonicalURL = canonicalJobURL(job.SourceURL)
	}

	var existingID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE idempotency_key = ?", envelope.IdempotencyKey).Scan(&existingID)
	if err == sql.ErrNoRows && sourceExternalKey != "" {
		err = tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE source_external_key = ?", sourceExternalKey).Scan(&existingID)
	}
	if err == sql.ErrNoRows && canonicalURL != "" {
		err = tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE canonical_url = ? ORDER BY id LIMIT 1", canonicalURL).Scan(&existingID)
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("find existing job: %w", err)
	}

	publishedAt := timePtrToString(job.PublishedAt)
	discoveredAt := job.DiscoveredAt
	if discoveredAt.IsZero() {
		discoveredAt = envelope.OccurredAt
	}

	if existingID != 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE jobs SET
	source = ?,
	external_id = ?,
	title = ?,
	company = ?,
	location = ?,
	remote_policy = ?,
	employment_type = ?,
	source_url = ?,
	application_url = ?,
	idempotency_key = ?,
	source_external_key = ?,
	canonical_url = ?,
	published_at = ?,
	discovered_at = ?,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
			job.Source,
			job.ExternalID,
			job.Title,
			job.Company,
			job.Location,
			job.RemotePolicy,
			job.EmploymentType,
			job.SourceURL,
			job.ApplicationURL,
			envelope.IdempotencyKey,
			nullIfEmpty(sourceExternalKey),
			nullIfEmpty(canonicalURL),
			publishedAt,
			discoveredAt.UTC().Format(time.RFC3339Nano),
			existingID,
		); err != nil {
			return 0, false, fmt.Errorf("update job: %w", err)
		}
		return existingID, false, nil
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO jobs(
	source,
	external_id,
	title,
	company,
	location,
	remote_policy,
	employment_type,
	source_url,
	application_url,
	idempotency_key,
	source_external_key,
	canonical_url,
	published_at,
	discovered_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.Source,
		job.ExternalID,
		job.Title,
		job.Company,
		job.Location,
		job.RemotePolicy,
		job.EmploymentType,
		job.SourceURL,
		job.ApplicationURL,
		envelope.IdempotencyKey,
		nullIfEmpty(sourceExternalKey),
		nullIfEmpty(canonicalURL),
		publishedAt,
		discoveredAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, false, fmt.Errorf("insert job: %w", err)
	}

	jobID, err := result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("get inserted job id: %w", err)
	}
	return jobID, true, nil
}

func upsertDescription(ctx context.Context, tx *sql.Tx, jobID int64, job events.JobDiscoveredPayload) error {
	skills, err := json.Marshal(job.DetectedSkills)
	if err != nil {
		return fmt.Errorf("marshal detected skills: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO job_descriptions(job_id, raw_text, detected_skills_json)
VALUES(?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	raw_text = excluded.raw_text,
	detected_skills_json = excluded.detected_skills_json,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		jobID,
		job.Description,
		string(skills),
	)
	if err != nil {
		return fmt.Errorf("upsert description: %w", err)
	}
	return nil
}

func replaceSearchIndex(ctx context.Context, tx *sql.Tx, jobID int64, job events.JobDiscoveredPayload) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM jobs_fts WHERE rowid = ?", jobID); err != nil {
		return fmt.Errorf("delete fts row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO jobs_fts(rowid, title, company, location, description, skills)
VALUES(?, ?, ?, ?, ?, ?)`,
		jobID,
		job.Title,
		job.Company,
		job.Location,
		job.Description,
		strings.Join(job.DetectedSkills, " "),
	); err != nil {
		return fmt.Errorf("insert fts row: %w", err)
	}
	return nil
}

func findJobIDByIdempotencyKey(ctx context.Context, tx *sql.Tx, idempotencyKey string) (int64, error) {
	var jobID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM jobs WHERE idempotency_key = ?", idempotencyKey).Scan(&jobID)
	if err != nil {
		return 0, fmt.Errorf("find job by idempotency key: %w", err)
	}
	return jobID, nil
}

func canonicalJobURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return strings.ToLower(value)
	}
	parsed.Fragment = ""
	query := parsed.Query()
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String()
}

func timePtrToString(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func nullIfEmpty(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
