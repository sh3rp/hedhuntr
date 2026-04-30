package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

type APIReviewApplication struct {
	ApplicationID      int64               `json:"applicationId"`
	JobID              int64               `json:"jobId"`
	CandidateProfileID int64               `json:"candidateProfileId"`
	JobTitle           string              `json:"jobTitle"`
	Company            string              `json:"company"`
	Location           string              `json:"location"`
	MatchScore         int                 `json:"matchScore"`
	ApplicationStatus  string              `json:"applicationStatus"`
	UpdatedAt          string              `json:"updatedAt"`
	Materials          []APIReviewMaterial `json:"materials"`
}

type APIReviewMaterial struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Notes      string `json:"notes"`
	DocumentID int64  `json:"documentId"`
	Path       string `json:"path"`
	Content    string `json:"content"`
	UpdatedAt  string `json:"updatedAt"`
}

type UpdateApplicationMaterialStatusParams struct {
	ID     int64
	Status string
	Notes  string
}

type MaterialRegenerationContext struct {
	ApplicationID      int64
	JobID              int64
	CandidateProfileID int64
	MatchScore         int
	MaterialID         int64
	Kind               string
}

func (s *Store) APIReviewQueue(ctx context.Context) ([]APIReviewApplication, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT a.id, a.job_id, a.candidate_profile_id, j.title, j.company, COALESCE(j.location, ''),
	a.match_score, a.status, a.updated_at,
	m.id, m.kind, m.status, COALESCE(m.notes, ''), m.document_id, d.path, m.updated_at
FROM applications a
JOIN jobs j ON j.id = a.job_id
LEFT JOIN application_materials m ON m.application_id = a.id
LEFT JOIN documents d ON d.id = m.document_id
WHERE a.status IN ('ready_to_apply', 'materials_drafted', 'reviewing')
	OR m.status IN ('draft', 'needs_changes', 'regeneration_requested')
ORDER BY a.updated_at DESC, a.id DESC, m.kind`)
	if err != nil {
		return nil, fmt.Errorf("query review queue: %w", err)
	}
	defer rows.Close()

	apps := []APIReviewApplication{}
	index := map[int64]int{}
	for rows.Next() {
		var app APIReviewApplication
		var material APIReviewMaterial
		var materialID sql.NullInt64
		var kind, materialStatus, notes, path, materialUpdated sql.NullString
		var documentID sql.NullInt64
		if err := rows.Scan(
			&app.ApplicationID,
			&app.JobID,
			&app.CandidateProfileID,
			&app.JobTitle,
			&app.Company,
			&app.Location,
			&app.MatchScore,
			&app.ApplicationStatus,
			&app.UpdatedAt,
			&materialID,
			&kind,
			&materialStatus,
			&notes,
			&documentID,
			&path,
			&materialUpdated,
		); err != nil {
			return nil, err
		}

		pos, ok := index[app.ApplicationID]
		if !ok {
			index[app.ApplicationID] = len(apps)
			app.Materials = []APIReviewMaterial{}
			apps = append(apps, app)
			pos = len(apps) - 1
		}

		if materialID.Valid {
			material.ID = materialID.Int64
			material.Kind = kind.String
			material.Status = materialStatus.String
			material.Notes = notes.String
			material.DocumentID = documentID.Int64
			material.Path = path.String
			material.UpdatedAt = materialUpdated.String
			content, err := os.ReadFile(material.Path)
			if err == nil {
				material.Content = string(content)
			}
			apps[pos].Materials = append(apps[pos].Materials, material)
		}
	}
	return apps, rows.Err()
}

func (s *Store) UpdateApplicationMaterialStatus(ctx context.Context, params UpdateApplicationMaterialStatusParams) (APIReviewMaterial, error) {
	switch params.Status {
	case "draft", "approved", "rejected", "needs_changes", "regeneration_requested":
	default:
		return APIReviewMaterial{}, fmt.Errorf("unsupported material status %q", params.Status)
	}

	result, err := s.db.ExecContext(ctx, `
UPDATE application_materials
SET status = ?, notes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		params.Status,
		params.Notes,
		params.ID,
	)
	if err != nil {
		return APIReviewMaterial{}, fmt.Errorf("update application material: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return APIReviewMaterial{}, err
	}
	if affected == 0 {
		return APIReviewMaterial{}, sql.ErrNoRows
	}
	return s.APIReviewMaterial(ctx, params.ID)
}

func (s *Store) APIReviewMaterial(ctx context.Context, id int64) (APIReviewMaterial, error) {
	var material APIReviewMaterial
	err := s.db.QueryRowContext(ctx, `
SELECT m.id, m.kind, m.status, COALESCE(m.notes, ''), m.document_id, d.path, m.updated_at
FROM application_materials m
JOIN documents d ON d.id = m.document_id
WHERE m.id = ?`, id).Scan(
		&material.ID,
		&material.Kind,
		&material.Status,
		&material.Notes,
		&material.DocumentID,
		&material.Path,
		&material.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return APIReviewMaterial{}, fmt.Errorf("application material %d not found", id)
	}
	if err != nil {
		return APIReviewMaterial{}, fmt.Errorf("load application material: %w", err)
	}
	content, err := os.ReadFile(material.Path)
	if err == nil {
		material.Content = string(content)
	}
	return material, nil
}

func (s *Store) MaterialRegenerationContext(ctx context.Context, materialID int64) (MaterialRegenerationContext, error) {
	var out MaterialRegenerationContext
	err := s.db.QueryRowContext(ctx, `
SELECT a.id, a.job_id, a.candidate_profile_id, a.match_score, m.id, m.kind
FROM application_materials m
JOIN applications a ON a.id = m.application_id
WHERE m.id = ?`, materialID).Scan(
		&out.ApplicationID,
		&out.JobID,
		&out.CandidateProfileID,
		&out.MatchScore,
		&out.MaterialID,
		&out.Kind,
	)
	if err == sql.ErrNoRows {
		return MaterialRegenerationContext{}, fmt.Errorf("application material %d not found", materialID)
	}
	if err != nil {
		return MaterialRegenerationContext{}, fmt.Errorf("load regeneration context: %w", err)
	}
	return out, nil
}
