package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"hedhuntr/internal/profile"
)

func (s *Store) UpsertFullCandidateProfile(ctx context.Context, p profile.Profile) (int64, error) {
	if err := profile.Validate(p); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	profileID, err := upsertCandidateProfileTx(ctx, tx, p)
	if err != nil {
		return 0, err
	}
	if err := replaceProfileSections(ctx, tx, profileID, p); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return profileID, nil
}

func (s *Store) LoadFullCandidateProfile(ctx context.Context, id int64) (profile.Profile, error) {
	var p profile.Profile
	var skillsJSON, titlesJSON, locationsJSON string
	var minSalary sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, COALESCE(headline, ''), skills_json, preferred_titles_json, preferred_locations_json,
	COALESCE(remote_preference, ''), min_salary
FROM candidate_profiles
WHERE id = ?`, id).Scan(
		&p.ID,
		&p.Name,
		&p.Headline,
		&skillsJSON,
		&titlesJSON,
		&locationsJSON,
		&p.RemotePreference,
		&minSalary,
	)
	if err == sql.ErrNoRows {
		return profile.Profile{}, fmt.Errorf("candidate profile %d not found", id)
	}
	if err != nil {
		return profile.Profile{}, fmt.Errorf("load candidate profile: %w", err)
	}
	if err := json.Unmarshal([]byte(skillsJSON), &p.Skills); err != nil {
		return profile.Profile{}, err
	}
	if err := json.Unmarshal([]byte(titlesJSON), &p.PreferredTitles); err != nil {
		return profile.Profile{}, err
	}
	if err := json.Unmarshal([]byte(locationsJSON), &p.PreferredLocations); err != nil {
		return profile.Profile{}, err
	}
	if minSalary.Valid {
		value := int(minSalary.Int64)
		p.MinSalary = &value
	}

	var loadErr error
	if p.WorkHistory, loadErr = s.loadWorkHistory(ctx, id); loadErr != nil {
		return profile.Profile{}, loadErr
	}
	if p.Projects, loadErr = s.loadProjects(ctx, id); loadErr != nil {
		return profile.Profile{}, loadErr
	}
	if p.Education, loadErr = s.loadEducation(ctx, id); loadErr != nil {
		return profile.Profile{}, loadErr
	}
	if p.Certifications, loadErr = s.loadCertifications(ctx, id); loadErr != nil {
		return profile.Profile{}, loadErr
	}
	if p.Links, loadErr = s.loadLinks(ctx, id); loadErr != nil {
		return profile.Profile{}, loadErr
	}
	return p, nil
}

func upsertCandidateProfileTx(ctx context.Context, tx *sql.Tx, p profile.Profile) (int64, error) {
	skillsJSON, err := json.Marshal(p.Skills)
	if err != nil {
		return 0, err
	}
	titlesJSON, err := json.Marshal(p.PreferredTitles)
	if err != nil {
		return 0, err
	}
	locationsJSON, err := json.Marshal(p.PreferredLocations)
	if err != nil {
		return 0, err
	}

	if p.ID > 0 {
		_, err = tx.ExecContext(ctx, `
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
			p.ID,
			p.Name,
			p.Headline,
			string(skillsJSON),
			string(titlesJSON),
			string(locationsJSON),
			nullIfEmpty(p.RemotePreference),
			intPtrOrNil(p.MinSalary),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert candidate profile: %w", err)
		}
		return p.ID, nil
	}

	result, err := tx.ExecContext(ctx, `
INSERT INTO candidate_profiles(name, headline, skills_json, preferred_titles_json, preferred_locations_json, remote_preference, min_salary)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
		p.Name,
		p.Headline,
		string(skillsJSON),
		string(titlesJSON),
		string(locationsJSON),
		nullIfEmpty(p.RemotePreference),
		intPtrOrNil(p.MinSalary),
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

func replaceProfileSections(ctx context.Context, tx *sql.Tx, profileID int64, p profile.Profile) error {
	for _, table := range []string{
		"candidate_work_history",
		"candidate_projects",
		"candidate_education",
		"candidate_certifications",
		"candidate_links",
	} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE candidate_profile_id = ?", profileID); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	for i, item := range p.WorkHistory {
		highlights, _ := json.Marshal(item.Highlights)
		technologies, _ := json.Marshal(item.Technologies)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO candidate_work_history(candidate_profile_id, company, title, location, start_date, end_date, current, summary, highlights_json, technologies_json, sort_order)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, item.Company, item.Title, item.Location, item.StartDate, item.EndDate, boolInt(item.Current),
			item.Summary, string(highlights), string(technologies), i,
		); err != nil {
			return fmt.Errorf("insert work_history[%d]: %w", i, err)
		}
	}
	for i, item := range p.Projects {
		highlights, _ := json.Marshal(item.Highlights)
		technologies, _ := json.Marshal(item.Technologies)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO candidate_projects(candidate_profile_id, name, role, url, summary, highlights_json, technologies_json, sort_order)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, item.Name, item.Role, item.URL, item.Summary, string(highlights), string(technologies), i,
		); err != nil {
			return fmt.Errorf("insert projects[%d]: %w", i, err)
		}
	}
	for i, item := range p.Education {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO candidate_education(candidate_profile_id, institution, degree, field, start_date, end_date, summary, sort_order)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, item.Institution, item.Degree, item.Field, item.StartDate, item.EndDate, item.Summary, i,
		); err != nil {
			return fmt.Errorf("insert education[%d]: %w", i, err)
		}
	}
	for i, item := range p.Certifications {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO candidate_certifications(candidate_profile_id, name, issuer, issued_at, expires_at, url, sort_order)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
			profileID, item.Name, item.Issuer, item.IssuedAt, item.ExpiresAt, item.URL, i,
		); err != nil {
			return fmt.Errorf("insert certifications[%d]: %w", i, err)
		}
	}
	for i, item := range p.Links {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO candidate_links(candidate_profile_id, label, url, sort_order)
VALUES(?, ?, ?, ?)`,
			profileID, item.Label, item.URL, i,
		); err != nil {
			return fmt.Errorf("insert links[%d]: %w", i, err)
		}
	}
	return nil
}

func (s *Store) loadWorkHistory(ctx context.Context, profileID int64) ([]profile.WorkHistory, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT company, title, COALESCE(location, ''), COALESCE(start_date, ''), COALESCE(end_date, ''), current,
	COALESCE(summary, ''), highlights_json, technologies_json
FROM candidate_work_history
WHERE candidate_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []profile.WorkHistory
	for rows.Next() {
		var item profile.WorkHistory
		var current int
		var highlightsJSON, technologiesJSON string
		if err := rows.Scan(&item.Company, &item.Title, &item.Location, &item.StartDate, &item.EndDate, &current, &item.Summary, &highlightsJSON, &technologiesJSON); err != nil {
			return nil, err
		}
		item.Current = current == 1
		json.Unmarshal([]byte(highlightsJSON), &item.Highlights)
		json.Unmarshal([]byte(technologiesJSON), &item.Technologies)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) loadProjects(ctx context.Context, profileID int64) ([]profile.Project, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT name, COALESCE(role, ''), COALESCE(url, ''), COALESCE(summary, ''), highlights_json, technologies_json
FROM candidate_projects
WHERE candidate_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []profile.Project
	for rows.Next() {
		var item profile.Project
		var highlightsJSON, technologiesJSON string
		if err := rows.Scan(&item.Name, &item.Role, &item.URL, &item.Summary, &highlightsJSON, &technologiesJSON); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(highlightsJSON), &item.Highlights)
		json.Unmarshal([]byte(technologiesJSON), &item.Technologies)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) loadEducation(ctx context.Context, profileID int64) ([]profile.Education, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT institution, COALESCE(degree, ''), COALESCE(field, ''), COALESCE(start_date, ''), COALESCE(end_date, ''), COALESCE(summary, '')
FROM candidate_education
WHERE candidate_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []profile.Education
	for rows.Next() {
		var item profile.Education
		if err := rows.Scan(&item.Institution, &item.Degree, &item.Field, &item.StartDate, &item.EndDate, &item.Summary); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) loadCertifications(ctx context.Context, profileID int64) ([]profile.Certification, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT name, COALESCE(issuer, ''), COALESCE(issued_at, ''), COALESCE(expires_at, ''), COALESCE(url, '')
FROM candidate_certifications
WHERE candidate_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []profile.Certification
	for rows.Next() {
		var item profile.Certification
		if err := rows.Scan(&item.Name, &item.Issuer, &item.IssuedAt, &item.ExpiresAt, &item.URL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) loadLinks(ctx context.Context, profileID int64) ([]profile.Link, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT label, url
FROM candidate_links
WHERE candidate_profile_id = ?
ORDER BY sort_order, id`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []profile.Link
	for rows.Next() {
		var item profile.Link
		if err := rows.Scan(&item.Label, &item.URL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
