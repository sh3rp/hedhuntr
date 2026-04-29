package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type JobForParsing struct {
	ID          int64
	Source      string
	Title       string
	Company     string
	Description string
}

type SaveParsedJobParams struct {
	JobID            int64
	Skills           []string
	Requirements     []string
	Responsibilities []string
	SalaryMin        *int
	SalaryMax        *int
	SalaryCurrency   string
	SalaryPeriod     string
	RemotePolicy     string
	Seniority        string
	EmploymentType   string
	ParsedAt         time.Time
}

func (s *Store) GetJobForParsing(ctx context.Context, jobID int64) (JobForParsing, error) {
	var job JobForParsing
	err := s.db.QueryRowContext(ctx, `
SELECT j.id, j.source, j.title, j.company, COALESCE(d.raw_text, '')
FROM jobs j
LEFT JOIN job_descriptions d ON d.job_id = j.id
WHERE j.id = ?`, jobID).Scan(
		&job.ID,
		&job.Source,
		&job.Title,
		&job.Company,
		&job.Description,
	)
	if err == sql.ErrNoRows {
		return JobForParsing{}, fmt.Errorf("job %d not found", jobID)
	}
	if err != nil {
		return JobForParsing{}, fmt.Errorf("load job for parsing: %w", err)
	}
	return job, nil
}

func (s *Store) SaveParsedJob(ctx context.Context, params SaveParsedJobParams) error {
	if params.ParsedAt.IsZero() {
		params.ParsedAt = time.Now().UTC()
	}

	skillsJSON, err := json.Marshal(params.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}
	requirementsJSON, err := json.Marshal(params.Requirements)
	if err != nil {
		return fmt.Errorf("marshal requirements: %w", err)
	}
	responsibilitiesJSON, err := json.Marshal(params.Responsibilities)
	if err != nil {
		return fmt.Errorf("marshal responsibilities: %w", err)
	}
	metadataJSON, err := json.Marshal(map[string]any{
		"skills":           params.Skills,
		"requirements":     params.Requirements,
		"responsibilities": params.Responsibilities,
		"salary_min":       params.SalaryMin,
		"salary_max":       params.SalaryMax,
		"salary_currency":  params.SalaryCurrency,
		"salary_period":    params.SalaryPeriod,
		"remote_policy":    params.RemotePolicy,
		"seniority":        params.Seniority,
		"employment_type":  params.EmploymentType,
	})
	if err != nil {
		return fmt.Errorf("marshal parsed metadata: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO job_requirements(
	job_id,
	skills_json,
	requirements_json,
	responsibilities_json,
	salary_min,
	salary_max,
	salary_currency,
	salary_period,
	remote_policy,
	seniority,
	employment_type
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	skills_json = excluded.skills_json,
	requirements_json = excluded.requirements_json,
	responsibilities_json = excluded.responsibilities_json,
	salary_min = excluded.salary_min,
	salary_max = excluded.salary_max,
	salary_currency = excluded.salary_currency,
	salary_period = excluded.salary_period,
	remote_policy = excluded.remote_policy,
	seniority = excluded.seniority,
	employment_type = excluded.employment_type,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.JobID,
		string(skillsJSON),
		string(requirementsJSON),
		string(responsibilitiesJSON),
		intPtrOrNil(params.SalaryMin),
		intPtrOrNil(params.SalaryMax),
		nullIfEmpty(params.SalaryCurrency),
		nullIfEmpty(params.SalaryPeriod),
		nullIfEmpty(params.RemotePolicy),
		nullIfEmpty(params.Seniority),
		nullIfEmpty(params.EmploymentType),
	); err != nil {
		return fmt.Errorf("upsert job requirements: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE job_descriptions
SET parsed_metadata_json = ?,
	parsed_at = ?,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE job_id = ?`,
		string(metadataJSON),
		params.ParsedAt.UTC().Format(time.RFC3339Nano),
		params.JobID,
	); err != nil {
		return fmt.Errorf("update parsed metadata: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE jobs
SET status = 'parsed',
	remote_policy = COALESCE(?, remote_policy),
	employment_type = COALESCE(?, employment_type),
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		nullIfEmpty(params.RemotePolicy),
		nullIfEmpty(params.EmploymentType),
		params.JobID,
	); err != nil {
		return fmt.Errorf("update job parsed status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

type ParsedJobSnapshot struct {
	Status       string
	SkillsJSON   string
	RemotePolicy string
	SalaryMin    sql.NullInt64
	SalaryMax    sql.NullInt64
}

func (s *Store) GetParsedJobSnapshot(ctx context.Context, jobID int64) (ParsedJobSnapshot, error) {
	var snapshot ParsedJobSnapshot
	err := s.db.QueryRowContext(ctx, `
SELECT j.status, r.skills_json, COALESCE(r.remote_policy, ''), r.salary_min, r.salary_max
FROM jobs j
JOIN job_requirements r ON r.job_id = j.id
WHERE j.id = ?`, jobID).Scan(
		&snapshot.Status,
		&snapshot.SkillsJSON,
		&snapshot.RemotePolicy,
		&snapshot.SalaryMin,
		&snapshot.SalaryMax,
	)
	if err != nil {
		return ParsedJobSnapshot{}, err
	}
	return snapshot, nil
}

func intPtrOrNil(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
