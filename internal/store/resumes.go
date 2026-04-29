package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

type CreateDocumentParams struct {
	Kind      string
	Format    string
	Path      string
	SHA256    string
	SizeBytes int64
}

type CreateResumeSourceParams struct {
	CandidateProfileID *int64
	Name               string
	Format             string
	DocumentID         int64
}

type CreateResumeVersionParams struct {
	ResumeSourceID int64
	JobID          int64
	DocumentID     int64
	Status         string
	Notes          string
}

type CreateApplicationMaterialParams struct {
	ApplicationID      int64
	JobID              int64
	CandidateProfileID int64
	Kind               string
	DocumentID         int64
	Status             string
	Notes              string
	SourceEventID      string
}

type ResumeSource struct {
	ID                 int64         `json:"id"`
	CandidateProfileID sql.NullInt64 `json:"candidateProfileId"`
	Name               string        `json:"name"`
	Format             string        `json:"format"`
	DocumentID         int64         `json:"documentId"`
	DocumentPath       string        `json:"documentPath"`
	DocumentSHA256     string        `json:"documentSHA256"`
	SizeBytes          int64         `json:"sizeBytes"`
	CreatedAt          string        `json:"createdAt"`
}

func (s *Store) CreateDocument(ctx context.Context, params CreateDocumentParams) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO documents(kind, format, path, sha256, size_bytes)
VALUES(?, ?, ?, ?, ?)`,
		params.Kind,
		params.Format,
		params.Path,
		params.SHA256,
		params.SizeBytes,
	)
	if err != nil {
		return 0, fmt.Errorf("create document: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) CreateResumeSource(ctx context.Context, params CreateResumeSourceParams) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO resume_sources(candidate_profile_id, name, format, document_id)
VALUES(?, ?, ?, ?)`,
		int64PtrOrNil(params.CandidateProfileID),
		params.Name,
		params.Format,
		params.DocumentID,
	)
	if err != nil {
		return 0, fmt.Errorf("create resume source: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) CreateResumeVersion(ctx context.Context, params CreateResumeVersionParams) (int64, error) {
	if params.Status == "" {
		params.Status = "draft"
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO resume_versions(resume_source_id, job_id, document_id, status, notes)
VALUES(?, ?, ?, ?, ?)`,
		params.ResumeSourceID,
		nullInt64(params.JobID),
		params.DocumentID,
		params.Status,
		params.Notes,
	)
	if err != nil {
		return 0, fmt.Errorf("create resume version: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) CreateApplicationMaterial(ctx context.Context, params CreateApplicationMaterialParams) (int64, error) {
	if params.Status == "" {
		params.Status = "draft"
	}
	if params.SourceEventID != "" {
		var existingID int64
		err := s.db.QueryRowContext(ctx, `
SELECT id FROM application_materials
WHERE application_id = ? AND kind = ? AND source_event_id = ?`,
			params.ApplicationID,
			params.Kind,
			params.SourceEventID,
		).Scan(&existingID)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("load application material: %w", err)
		}
		if existingID > 0 {
			_, err = s.db.ExecContext(ctx, `
UPDATE application_materials
SET document_id = ?, status = ?, notes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
				params.DocumentID,
				params.Status,
				params.Notes,
				existingID,
			)
			if err != nil {
				return 0, fmt.Errorf("update application material: %w", err)
			}
			return existingID, nil
		}
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO application_materials(application_id, job_id, candidate_profile_id, kind, document_id, status, notes, source_event_id)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		params.ApplicationID,
		params.JobID,
		params.CandidateProfileID,
		params.Kind,
		params.DocumentID,
		params.Status,
		params.Notes,
		nullIfEmpty(params.SourceEventID),
	)
	if err != nil {
		return 0, fmt.Errorf("create application material: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) ListResumeSources(ctx context.Context) ([]ResumeSource, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT rs.id, rs.candidate_profile_id, rs.name, rs.format, rs.document_id, d.path, d.sha256, d.size_bytes, rs.created_at
FROM resume_sources rs
JOIN documents d ON d.id = rs.document_id
ORDER BY rs.created_at DESC, rs.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list resume sources: %w", err)
	}
	defer rows.Close()

	sources := []ResumeSource{}
	for rows.Next() {
		var source ResumeSource
		if err := rows.Scan(
			&source.ID,
			&source.CandidateProfileID,
			&source.Name,
			&source.Format,
			&source.DocumentID,
			&source.DocumentPath,
			&source.DocumentSHA256,
			&source.SizeBytes,
			&source.CreatedAt,
		); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) LoadResumeSourceContent(ctx context.Context, id int64) (ResumeSource, []byte, error) {
	var source ResumeSource
	err := s.db.QueryRowContext(ctx, `
SELECT rs.id, rs.candidate_profile_id, rs.name, rs.format, rs.document_id, d.path, d.sha256, d.size_bytes, rs.created_at
FROM resume_sources rs
JOIN documents d ON d.id = rs.document_id
WHERE rs.id = ?`, id).Scan(
		&source.ID,
		&source.CandidateProfileID,
		&source.Name,
		&source.Format,
		&source.DocumentID,
		&source.DocumentPath,
		&source.DocumentSHA256,
		&source.SizeBytes,
		&source.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return ResumeSource{}, nil, fmt.Errorf("resume source %d not found", id)
	}
	if err != nil {
		return ResumeSource{}, nil, fmt.Errorf("load resume source: %w", err)
	}
	content, err := os.ReadFile(source.DocumentPath)
	if err != nil {
		return ResumeSource{}, nil, err
	}
	return source, content, nil
}

func int64PtrOrNil(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
