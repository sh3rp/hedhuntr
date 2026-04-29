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
