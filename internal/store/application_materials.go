package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type ApplicationReadyContext struct {
	ApplicationID      int64
	JobID              int64
	CandidateProfileID int64
	MatchScore         int
	JobTitle           string
	Company            string
	Location           string
	ApplicationURL     string
	SourceURL          string
	Description        string
	Skills             []string
	Requirements       []string
	Responsibilities   []string
	MatchedSkills      []string
	MissingSkills      []string
}

func (s *Store) GetApplicationReadyContext(ctx context.Context, jobID, candidateProfileID int64) (ApplicationReadyContext, error) {
	var out ApplicationReadyContext
	var skillsJSON, requirementsJSON, responsibilitiesJSON string
	var matchedSkillsJSON, missingSkillsJSON string
	err := s.db.QueryRowContext(ctx, `
SELECT a.id, a.job_id, a.candidate_profile_id, a.match_score,
	j.title, j.company, COALESCE(j.location, ''), COALESCE(j.application_url, ''), j.source_url,
	COALESCE(d.raw_text, ''),
	COALESCE(r.skills_json, '[]'), COALESCE(r.requirements_json, '[]'), COALESCE(r.responsibilities_json, '[]'),
	COALESCE(m.matched_skills_json, '[]'), COALESCE(m.missing_skills_json, '[]')
FROM applications a
JOIN jobs j ON j.id = a.job_id
LEFT JOIN job_descriptions d ON d.job_id = j.id
LEFT JOIN job_requirements r ON r.job_id = j.id
LEFT JOIN job_matches m ON m.job_id = a.job_id AND m.candidate_profile_id = a.candidate_profile_id
WHERE a.job_id = ? AND a.candidate_profile_id = ? AND a.status = 'ready_to_apply'`,
		jobID,
		candidateProfileID,
	).Scan(
		&out.ApplicationID,
		&out.JobID,
		&out.CandidateProfileID,
		&out.MatchScore,
		&out.JobTitle,
		&out.Company,
		&out.Location,
		&out.ApplicationURL,
		&out.SourceURL,
		&out.Description,
		&skillsJSON,
		&requirementsJSON,
		&responsibilitiesJSON,
		&matchedSkillsJSON,
		&missingSkillsJSON,
	)
	if err == sql.ErrNoRows {
		return ApplicationReadyContext{}, fmt.Errorf("ready application for job %d and profile %d not found", jobID, candidateProfileID)
	}
	if err != nil {
		return ApplicationReadyContext{}, fmt.Errorf("load application context: %w", err)
	}
	json.Unmarshal([]byte(skillsJSON), &out.Skills)
	json.Unmarshal([]byte(requirementsJSON), &out.Requirements)
	json.Unmarshal([]byte(responsibilitiesJSON), &out.Responsibilities)
	json.Unmarshal([]byte(matchedSkillsJSON), &out.MatchedSkills)
	json.Unmarshal([]byte(missingSkillsJSON), &out.MissingSkills)
	return out, nil
}

func (s *Store) DefaultResumeSource(ctx context.Context, candidateProfileID int64) (ResumeSource, []byte, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
SELECT id
FROM resume_sources
WHERE (? <= 0 OR candidate_profile_id = ? OR candidate_profile_id IS NULL)
ORDER BY CASE WHEN candidate_profile_id = ? THEN 0 ELSE 1 END, created_at DESC, id DESC
LIMIT 1`,
		candidateProfileID,
		candidateProfileID,
		candidateProfileID,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return ResumeSource{}, nil, fmt.Errorf("no resume source available for profile %d", candidateProfileID)
	}
	if err != nil {
		return ResumeSource{}, nil, fmt.Errorf("find default resume source: %w", err)
	}
	return s.LoadResumeSourceContent(ctx, id)
}
