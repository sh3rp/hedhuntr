package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"hedhuntr/internal/matcher"
)

type UpsertCandidateProfileParams struct {
	ID                 int64
	Name               string
	Headline           string
	Skills             []string
	PreferredTitles    []string
	PreferredLocations []string
	RemotePreference   string
	MinSalary          *int
}

type JobForMatching struct {
	ID             int64
	Source         string
	Title          string
	Location       string
	Skills         []string
	SalaryMin      *int
	SalaryMax      *int
	RemotePolicy   string
	EmploymentType string
}

type SaveJobMatchParams struct {
	JobID              int64
	CandidateProfileID int64
	Score              int
	MatchedSkills      []string
	MissingSkills      []string
	Notes              []string
}

func (s *Store) UpsertCandidateProfile(ctx context.Context, params UpsertCandidateProfileParams) (int64, error) {
	skillsJSON, err := json.Marshal(params.Skills)
	if err != nil {
		return 0, fmt.Errorf("marshal skills: %w", err)
	}
	titlesJSON, err := json.Marshal(params.PreferredTitles)
	if err != nil {
		return 0, fmt.Errorf("marshal preferred titles: %w", err)
	}
	locationsJSON, err := json.Marshal(params.PreferredLocations)
	if err != nil {
		return 0, fmt.Errorf("marshal preferred locations: %w", err)
	}

	if params.ID > 0 {
		_, err = s.db.ExecContext(ctx, `
INSERT INTO candidate_profiles(
	id, name, headline, skills_json, preferred_titles_json, preferred_locations_json, remote_preference, min_salary
) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	headline = excluded.headline,
	skills_json = excluded.skills_json,
	preferred_titles_json = excluded.preferred_titles_json,
	preferred_locations_json = excluded.preferred_locations_json,
	remote_preference = excluded.remote_preference,
	min_salary = excluded.min_salary,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
			params.ID,
			params.Name,
			params.Headline,
			string(skillsJSON),
			string(titlesJSON),
			string(locationsJSON),
			nullIfEmpty(params.RemotePreference),
			intPtrOrNil(params.MinSalary),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert candidate profile: %w", err)
		}
		return params.ID, nil
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO candidate_profiles(name, headline, skills_json, preferred_titles_json, preferred_locations_json, remote_preference, min_salary)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
		params.Name,
		params.Headline,
		string(skillsJSON),
		string(titlesJSON),
		string(locationsJSON),
		nullIfEmpty(params.RemotePreference),
		intPtrOrNil(params.MinSalary),
	)
	if err != nil {
		return 0, fmt.Errorf("insert candidate profile: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) GetCandidateProfile(ctx context.Context, id int64) (matcher.CandidateProfile, error) {
	if id <= 0 {
		var err error
		id, err = s.defaultCandidateProfileID(ctx)
		if err != nil {
			return matcher.CandidateProfile{}, err
		}
	}

	var profile matcher.CandidateProfile
	var skillsJSON, titlesJSON, locationsJSON string
	var minSalary sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, skills_json, preferred_titles_json, preferred_locations_json, COALESCE(remote_preference, ''), min_salary
FROM candidate_profiles
WHERE id = ?`, id).Scan(
		&profile.ID,
		&profile.Name,
		&skillsJSON,
		&titlesJSON,
		&locationsJSON,
		&profile.RemotePreference,
		&minSalary,
	)
	if err == sql.ErrNoRows {
		return matcher.CandidateProfile{}, fmt.Errorf("candidate profile %d not found", id)
	}
	if err != nil {
		return matcher.CandidateProfile{}, fmt.Errorf("load candidate profile: %w", err)
	}
	if err := json.Unmarshal([]byte(skillsJSON), &profile.Skills); err != nil {
		return matcher.CandidateProfile{}, fmt.Errorf("decode profile skills: %w", err)
	}
	if err := json.Unmarshal([]byte(titlesJSON), &profile.PreferredTitles); err != nil {
		return matcher.CandidateProfile{}, fmt.Errorf("decode preferred titles: %w", err)
	}
	if err := json.Unmarshal([]byte(locationsJSON), &profile.PreferredLocations); err != nil {
		return matcher.CandidateProfile{}, fmt.Errorf("decode preferred locations: %w", err)
	}
	if minSalary.Valid {
		value := int(minSalary.Int64)
		profile.MinSalary = &value
	}
	return profile, nil
}

func (s *Store) GetJobForMatching(ctx context.Context, jobID int64) (JobForMatching, error) {
	var job JobForMatching
	var skillsJSON string
	var salaryMin, salaryMax sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT j.id, j.source, j.title, COALESCE(j.location, ''), r.skills_json, r.salary_min, r.salary_max,
	COALESCE(r.remote_policy, ''), COALESCE(r.employment_type, '')
FROM jobs j
JOIN job_requirements r ON r.job_id = j.id
WHERE j.id = ?`, jobID).Scan(
		&job.ID,
		&job.Source,
		&job.Title,
		&job.Location,
		&skillsJSON,
		&salaryMin,
		&salaryMax,
		&job.RemotePolicy,
		&job.EmploymentType,
	)
	if err == sql.ErrNoRows {
		return JobForMatching{}, fmt.Errorf("parsed job %d not found", jobID)
	}
	if err != nil {
		return JobForMatching{}, fmt.Errorf("load job for matching: %w", err)
	}
	if err := json.Unmarshal([]byte(skillsJSON), &job.Skills); err != nil {
		return JobForMatching{}, fmt.Errorf("decode job skills: %w", err)
	}
	if salaryMin.Valid {
		value := int(salaryMin.Int64)
		job.SalaryMin = &value
	}
	if salaryMax.Valid {
		value := int(salaryMax.Int64)
		job.SalaryMax = &value
	}
	return job, nil
}

func (s *Store) SaveJobMatch(ctx context.Context, params SaveJobMatchParams) error {
	matchedJSON, err := json.Marshal(params.MatchedSkills)
	if err != nil {
		return fmt.Errorf("marshal matched skills: %w", err)
	}
	missingJSON, err := json.Marshal(params.MissingSkills)
	if err != nil {
		return fmt.Errorf("marshal missing skills: %w", err)
	}
	notesJSON, err := json.Marshal(params.Notes)
	if err != nil {
		return fmt.Errorf("marshal notes: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO job_matches(job_id, candidate_profile_id, score, matched_skills_json, missing_skills_json, notes_json)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id, candidate_profile_id) DO UPDATE SET
	score = excluded.score,
	matched_skills_json = excluded.matched_skills_json,
	missing_skills_json = excluded.missing_skills_json,
	notes_json = excluded.notes_json,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		params.JobID,
		params.CandidateProfileID,
		params.Score,
		string(matchedJSON),
		string(missingJSON),
		string(notesJSON),
	)
	if err != nil {
		return fmt.Errorf("save job match: %w", err)
	}
	return nil
}

func (s *Store) MarkApplicationReady(ctx context.Context, jobID, candidateProfileID int64, score int) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO applications(job_id, candidate_profile_id, status, match_score)
VALUES(?, ?, 'ready_to_apply', ?)
ON CONFLICT(job_id) DO UPDATE SET
	candidate_profile_id = excluded.candidate_profile_id,
	status = excluded.status,
	match_score = excluded.match_score,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`,
		jobID,
		candidateProfileID,
		score,
	)
	if err != nil {
		return fmt.Errorf("mark application ready: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'ready_to_apply',
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, jobID)
	if err != nil {
		return fmt.Errorf("update job ready status: %w", err)
	}
	return nil
}

type JobMatchSnapshot struct {
	Score             int
	MatchedSkillsJSON string
	MissingSkillsJSON string
	ApplicationStatus string
}

func (s *Store) GetJobMatchSnapshot(ctx context.Context, jobID, candidateProfileID int64) (JobMatchSnapshot, error) {
	var snapshot JobMatchSnapshot
	err := s.db.QueryRowContext(ctx, `
SELECT m.score, m.matched_skills_json, m.missing_skills_json, COALESCE(a.status, '')
FROM job_matches m
LEFT JOIN applications a ON a.job_id = m.job_id AND a.candidate_profile_id = m.candidate_profile_id
WHERE m.job_id = ? AND m.candidate_profile_id = ?`, jobID, candidateProfileID).Scan(
		&snapshot.Score,
		&snapshot.MatchedSkillsJSON,
		&snapshot.MissingSkillsJSON,
		&snapshot.ApplicationStatus,
	)
	if err != nil {
		return JobMatchSnapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) defaultCandidateProfileID(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM candidate_profiles ORDER BY id LIMIT 1").Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	return s.UpsertCandidateProfile(ctx, UpsertCandidateProfileParams{
		Name:               "Default Candidate",
		Skills:             []string{"Go", "TypeScript", "React", "SQLite", "NATS"},
		PreferredTitles:    []string{"engineer", "developer"},
		PreferredLocations: []string{"remote"},
		RemotePreference:   "remote",
		MinSalary:          intPtr(100000),
	})
}

func intPtr(value int) *int {
	return &value
}

func (job JobForMatching) MatcherJob() matcher.Job {
	return matcher.Job{
		ID:             job.ID,
		Title:          job.Title,
		Location:       job.Location,
		Skills:         job.Skills,
		SalaryMin:      job.SalaryMin,
		SalaryMax:      job.SalaryMax,
		RemotePolicy:   job.RemotePolicy,
		EmploymentType: job.EmploymentType,
	}
}
