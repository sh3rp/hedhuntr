package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type Interview struct {
	ID                 int64           `json:"id"`
	ApplicationID      int64           `json:"applicationId"`
	JobID              int64           `json:"jobId"`
	CandidateProfileID int64           `json:"candidateProfileId"`
	JobTitle           string          `json:"jobTitle"`
	Company            string          `json:"company"`
	Stage              string          `json:"stage"`
	Status             string          `json:"status"`
	ScheduledAt        string          `json:"scheduledAt,omitempty"`
	DurationMinutes    int             `json:"durationMinutes,omitempty"`
	Location           string          `json:"location,omitempty"`
	Contacts           []string        `json:"contacts"`
	Notes              string          `json:"notes,omitempty"`
	Outcome            string          `json:"outcome,omitempty"`
	Tasks              []InterviewTask `json:"tasks"`
	CreatedAt          string          `json:"createdAt"`
	UpdatedAt          string          `json:"updatedAt"`
}

type InterviewTask struct {
	ID          int64  `json:"id"`
	InterviewID int64  `json:"interviewId"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	DueAt       string `json:"dueAt,omitempty"`
	Notes       string `json:"notes,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type CreateInterviewParams struct {
	ApplicationID   int64    `json:"applicationId"`
	Stage           string   `json:"stage"`
	Status          string   `json:"status"`
	ScheduledAt     string   `json:"scheduledAt"`
	DurationMinutes int      `json:"durationMinutes"`
	Location        string   `json:"location"`
	Contacts        []string `json:"contacts"`
	Notes           string   `json:"notes"`
}

type UpdateInterviewParams struct {
	Status  string `json:"status"`
	Outcome string `json:"outcome"`
	Notes   string `json:"notes"`
}

type CreateInterviewTaskParams struct {
	InterviewID int64  `json:"interviewId"`
	Title       string `json:"title"`
	DueAt       string `json:"dueAt"`
	Notes       string `json:"notes"`
}

type UpdateInterviewTaskStatusParams struct {
	Status string `json:"status"`
}

func (s *Store) CreateInterview(ctx context.Context, params CreateInterviewParams) (Interview, error) {
	if params.ApplicationID <= 0 {
		return Interview{}, fmt.Errorf("application_id is required")
	}
	if strings.TrimSpace(params.Stage) == "" {
		return Interview{}, fmt.Errorf("stage is required")
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = "scheduled"
	}
	if !validInterviewStatus(status) {
		return Interview{}, fmt.Errorf("invalid interview status %q", status)
	}

	app, err := s.loadApplicationForInterview(ctx, params.ApplicationID)
	if err != nil {
		return Interview{}, err
	}
	contactsJSON, err := json.Marshal(nonBlankStrings(params.Contacts))
	if err != nil {
		return Interview{}, err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO interviews(application_id, job_id, candidate_profile_id, stage, status, scheduled_at, duration_minutes, location, contacts_json, notes)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		params.ApplicationID,
		app.JobID,
		app.CandidateProfileID,
		strings.TrimSpace(params.Stage),
		status,
		nullIfEmpty(params.ScheduledAt),
		zeroIntOrNil(params.DurationMinutes),
		params.Location,
		string(contactsJSON),
		params.Notes,
	)
	if err != nil {
		return Interview{}, fmt.Errorf("create interview: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Interview{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
UPDATE applications SET status = 'interview', updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, params.ApplicationID); err != nil {
		return Interview{}, fmt.Errorf("mark application interview: %w", err)
	}
	return s.GetInterview(ctx, id)
}

func (s *Store) UpdateInterview(ctx context.Context, id int64, params UpdateInterviewParams) (Interview, error) {
	if id <= 0 {
		return Interview{}, fmt.Errorf("interview id is required")
	}
	if strings.TrimSpace(params.Status) != "" && !validInterviewStatus(params.Status) {
		return Interview{}, fmt.Errorf("invalid interview status %q", params.Status)
	}
	current, err := s.GetInterview(ctx, id)
	if err != nil {
		return Interview{}, err
	}
	status := current.Status
	if strings.TrimSpace(params.Status) != "" {
		status = strings.TrimSpace(params.Status)
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE interviews
SET status = ?, outcome = ?, notes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`,
		status,
		params.Outcome,
		params.Notes,
		id,
	)
	if err != nil {
		return Interview{}, fmt.Errorf("update interview: %w", err)
	}
	return s.GetInterview(ctx, id)
}

func (s *Store) CreateInterviewTask(ctx context.Context, params CreateInterviewTaskParams) (InterviewTask, error) {
	if params.InterviewID <= 0 {
		return InterviewTask{}, fmt.Errorf("interview_id is required")
	}
	if strings.TrimSpace(params.Title) == "" {
		return InterviewTask{}, fmt.Errorf("title is required")
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO interview_tasks(interview_id, title, due_at, notes)
VALUES(?, ?, ?, ?)`,
		params.InterviewID,
		strings.TrimSpace(params.Title),
		nullIfEmpty(params.DueAt),
		params.Notes,
	)
	if err != nil {
		return InterviewTask{}, fmt.Errorf("create interview task: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return InterviewTask{}, err
	}
	return s.getInterviewTask(ctx, id)
}

func (s *Store) UpdateInterviewTaskStatus(ctx context.Context, id int64, params UpdateInterviewTaskStatusParams) (InterviewTask, error) {
	if id <= 0 {
		return InterviewTask{}, fmt.Errorf("interview task id is required")
	}
	status := strings.TrimSpace(params.Status)
	if !validInterviewTaskStatus(status) {
		return InterviewTask{}, fmt.Errorf("invalid interview task status %q", params.Status)
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE interview_tasks
SET status = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, status, id)
	if err != nil {
		return InterviewTask{}, fmt.Errorf("update interview task: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return InterviewTask{}, err
	}
	if affected == 0 {
		return InterviewTask{}, fmt.Errorf("interview task %d not found", id)
	}
	return s.getInterviewTask(ctx, id)
}

func (s *Store) ListInterviews(ctx context.Context) ([]Interview, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT i.id, i.application_id, i.job_id, i.candidate_profile_id, j.title, j.company, i.stage, i.status,
	COALESCE(i.scheduled_at, ''), COALESCE(i.duration_minutes, 0), COALESCE(i.location, ''),
	i.contacts_json, COALESCE(i.notes, ''), COALESCE(i.outcome, ''), i.created_at, i.updated_at
FROM interviews i
JOIN jobs j ON j.id = i.job_id
ORDER BY COALESCE(i.scheduled_at, i.created_at) DESC, i.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query interviews: %w", err)
	}
	defer rows.Close()

	interviews := []Interview{}
	for rows.Next() {
		interview, err := scanInterview(rows)
		if err != nil {
			return nil, err
		}
		interviews = append(interviews, interview)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachInterviewTasks(ctx, interviews)
}

func (s *Store) GetInterview(ctx context.Context, id int64) (Interview, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT i.id, i.application_id, i.job_id, i.candidate_profile_id, j.title, j.company, i.stage, i.status,
	COALESCE(i.scheduled_at, ''), COALESCE(i.duration_minutes, 0), COALESCE(i.location, ''),
	i.contacts_json, COALESCE(i.notes, ''), COALESCE(i.outcome, ''), i.created_at, i.updated_at
FROM interviews i
JOIN jobs j ON j.id = i.job_id
WHERE i.id = ?`, id)
	interview, err := scanInterview(row)
	if err == sql.ErrNoRows {
		return Interview{}, fmt.Errorf("interview %d not found", id)
	}
	if err != nil {
		return Interview{}, err
	}
	withTasks, err := s.attachInterviewTasks(ctx, []Interview{interview})
	if err != nil {
		return Interview{}, err
	}
	return withTasks[0], nil
}

type applicationForInterview struct {
	JobID              int64
	CandidateProfileID int64
}

func (s *Store) loadApplicationForInterview(ctx context.Context, applicationID int64) (applicationForInterview, error) {
	var app applicationForInterview
	err := s.db.QueryRowContext(ctx, `
SELECT job_id, candidate_profile_id
FROM applications
WHERE id = ?`, applicationID).Scan(&app.JobID, &app.CandidateProfileID)
	if err == sql.ErrNoRows {
		return applicationForInterview{}, fmt.Errorf("application %d not found", applicationID)
	}
	return app, err
}

type interviewScanner interface {
	Scan(dest ...any) error
}

func scanInterview(scanner interviewScanner) (Interview, error) {
	var interview Interview
	var contactsJSON string
	err := scanner.Scan(
		&interview.ID,
		&interview.ApplicationID,
		&interview.JobID,
		&interview.CandidateProfileID,
		&interview.JobTitle,
		&interview.Company,
		&interview.Stage,
		&interview.Status,
		&interview.ScheduledAt,
		&interview.DurationMinutes,
		&interview.Location,
		&contactsJSON,
		&interview.Notes,
		&interview.Outcome,
		&interview.CreatedAt,
		&interview.UpdatedAt,
	)
	if err != nil {
		return Interview{}, err
	}
	if err := json.Unmarshal([]byte(contactsJSON), &interview.Contacts); err != nil {
		return Interview{}, fmt.Errorf("decode interview contacts: %w", err)
	}
	if interview.Contacts == nil {
		interview.Contacts = []string{}
	}
	interview.Tasks = []InterviewTask{}
	return interview, nil
}

func (s *Store) attachInterviewTasks(ctx context.Context, interviews []Interview) ([]Interview, error) {
	for i := range interviews {
		tasks, err := s.listInterviewTasks(ctx, interviews[i].ID)
		if err != nil {
			return nil, err
		}
		interviews[i].Tasks = tasks
	}
	return interviews, nil
}

func (s *Store) listInterviewTasks(ctx context.Context, interviewID int64) ([]InterviewTask, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, interview_id, title, status, COALESCE(due_at, ''), COALESCE(notes, ''), created_at, updated_at
FROM interview_tasks
WHERE interview_id = ?
ORDER BY COALESCE(due_at, created_at), id`, interviewID)
	if err != nil {
		return nil, fmt.Errorf("query interview tasks: %w", err)
	}
	defer rows.Close()
	tasks := []InterviewTask{}
	for rows.Next() {
		var task InterviewTask
		if err := rows.Scan(&task.ID, &task.InterviewID, &task.Title, &task.Status, &task.DueAt, &task.Notes, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) getInterviewTask(ctx context.Context, id int64) (InterviewTask, error) {
	var task InterviewTask
	err := s.db.QueryRowContext(ctx, `
SELECT id, interview_id, title, status, COALESCE(due_at, ''), COALESCE(notes, ''), created_at, updated_at
FROM interview_tasks
WHERE id = ?`, id).Scan(&task.ID, &task.InterviewID, &task.Title, &task.Status, &task.DueAt, &task.Notes, &task.CreatedAt, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		return InterviewTask{}, fmt.Errorf("interview task %d not found", id)
	}
	return task, err
}

func validInterviewStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "scheduled", "completed", "cancelled", "no_show", "offer", "rejected":
		return true
	default:
		return false
	}
}

func validInterviewTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "open", "done":
		return true
	default:
		return false
	}
}

func nonBlankStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func zeroIntOrNil(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
