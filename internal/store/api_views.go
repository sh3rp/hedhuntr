package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type APIJob struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Company    string   `json:"company"`
	Location   string   `json:"location"`
	Status     string   `json:"status"`
	MatchScore int      `json:"matchScore"`
	Salary     string   `json:"salary"`
	Skills     []string `json:"skills"`
	Source     string   `json:"source"`
	UpdatedAt  string   `json:"updatedAt"`
}

type APIPipelineStage struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
}

type APIWorkerState struct {
	Name      string `json:"name"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Processed int    `json:"processed"`
	Failed    int    `json:"failed"`
}

type APINotificationDelivery struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Subject string `json:"subject"`
	Time    string `json:"time"`
}

func (s *Store) APIJobs(ctx context.Context) ([]APIJob, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT j.id, j.title, j.company, COALESCE(j.location, ''), j.status, COALESCE(jm.score, 0),
	COALESCE(jr.salary_min, 0), COALESCE(jr.salary_max, 0), COALESCE(jr.salary_currency, ''),
	COALESCE(jr.skills_json, '[]'), j.source, j.updated_at
FROM jobs j
LEFT JOIN job_requirements jr ON jr.job_id = j.id
LEFT JOIN job_matches jm ON jm.job_id = j.id
ORDER BY j.updated_at DESC, j.id DESC
LIMIT 100`)
	if err != nil {
		return nil, fmt.Errorf("query api jobs: %w", err)
	}
	defer rows.Close()

	jobs := []APIJob{}
	for rows.Next() {
		var job APIJob
		var skillsJSON string
		var salaryMin, salaryMax int
		var currency string
		if err := rows.Scan(&job.ID, &job.Title, &job.Company, &job.Location, &job.Status, &job.MatchScore, &salaryMin, &salaryMax, &currency, &skillsJSON, &job.Source, &job.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(skillsJSON), &job.Skills)
		job.Salary = formatSalary(salaryMin, salaryMax, currency)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *Store) APIPipeline(ctx context.Context) ([]APIPipelineStage, error) {
	counts := map[string]int{}
	rows, err := s.db.QueryContext(ctx, "SELECT status, COUNT(*) FROM jobs GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("query api pipeline: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	order := []struct {
		status string
		label  string
	}{
		{"discovered", "Discovered"},
		{"description_fetched", "Fetched"},
		{"parsed", "Parsed"},
		{"matched", "Matched"},
		{"ready_to_apply", "Ready"},
		{"applied", "Applied"},
		{"interview", "Interview"},
	}
	result := make([]APIPipelineStage, 0, len(order))
	for _, item := range order {
		result = append(result, APIPipelineStage{Status: item.status, Label: item.label, Count: counts[item.status]})
	}
	return result, nil
}

func (s *Store) APINotifications(ctx context.Context) ([]APINotificationDelivery, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT channel_name, channel_type, status, event_subject, delivered_at
FROM notification_deliveries
ORDER BY delivered_at DESC, id DESC
LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("query api notifications: %w", err)
	}
	defer rows.Close()

	deliveries := []APINotificationDelivery{}
	for rows.Next() {
		var delivery APINotificationDelivery
		if err := rows.Scan(&delivery.Channel, &delivery.Type, &delivery.Status, &delivery.Subject, &delivery.Time); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (s *Store) APIWorkers(ctx context.Context) ([]APIWorkerState, error) {
	sourceRuns, _ := s.countRows(ctx, "job_source_runs")
	jobEvents, _ := s.countRows(ctx, "job_events")
	notifications, _ := s.countRows(ctx, "notification_deliveries")
	return []APIWorkerState{
		{Name: "Scheduler", Subject: "source runs", Status: statusFromCount(sourceRuns), Processed: sourceRuns},
		{Name: "Dispatcher", Subject: "jobs.discovered", Status: statusFromCount(jobEvents), Processed: jobEvents},
		{Name: "Notifications", Subject: "jobs.matched/applications.ready", Status: statusFromCount(notifications), Processed: notifications},
	}, nil
}

func (s *Store) countRows(ctx context.Context, table string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
	return count, err
}

func (s *Store) FirstFullCandidateProfile(ctx context.Context) (any, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM candidate_profiles ORDER BY id LIMIT 1").Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.LoadFullCandidateProfile(ctx, id)
}

func formatSalary(minValue, maxValue int, currency string) string {
	if minValue == 0 && maxValue == 0 {
		return "Not listed"
	}
	prefix := "$"
	if currency != "" && currency != "USD" {
		prefix = currency + " "
	}
	if minValue > 0 && maxValue > 0 {
		return fmt.Sprintf("%s%dk-%s%dk", prefix, minValue/1000, prefix, maxValue/1000)
	}
	if maxValue > 0 {
		return fmt.Sprintf("Up to %s%dk", prefix, maxValue/1000)
	}
	return fmt.Sprintf("From %s%dk", prefix, minValue/1000)
}

func statusFromCount(count int) string {
	if count == 0 {
		return "idle"
	}
	return "running"
}
