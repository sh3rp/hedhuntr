package store

import (
	"context"
	"database/sql"
	"fmt"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var Migrations = []Migration{
	{
		Version: 1,
		Name:    "create_jobs_dispatcher_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source TEXT NOT NULL,
	external_id TEXT,
	title TEXT NOT NULL,
	company TEXT NOT NULL,
	location TEXT,
	remote_policy TEXT,
	employment_type TEXT,
	source_url TEXT NOT NULL,
	application_url TEXT,
	status TEXT NOT NULL DEFAULT 'discovered',
	idempotency_key TEXT NOT NULL UNIQUE,
	source_external_key TEXT,
	canonical_url TEXT,
	published_at TEXT,
	discovered_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_source_external_key
	ON jobs(source_external_key)
	WHERE source_external_key IS NOT NULL AND source_external_key != '';

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_company_title ON jobs(company, title);
CREATE INDEX IF NOT EXISTS idx_jobs_canonical_url ON jobs(canonical_url);

CREATE TABLE IF NOT EXISTS job_descriptions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id INTEGER NOT NULL UNIQUE,
	raw_text TEXT,
	raw_html TEXT,
	detected_skills_json TEXT NOT NULL DEFAULT '[]',
	fetched_at TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS job_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_id TEXT NOT NULL UNIQUE,
	event_type TEXT NOT NULL,
	event_version INTEGER NOT NULL,
	subject TEXT NOT NULL,
	source TEXT NOT NULL,
	correlation_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	processed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_job_events_idempotency_key ON job_events(idempotency_key);

CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
	title,
	company,
	location,
	description,
	skills,
	tokenize='porter'
);
`,
	},
	{
		Version: 2,
		Name:    "create_scheduler_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS job_sources (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	schedule TEXT NOT NULL,
	interval_seconds INTEGER NOT NULL,
	timeout_seconds INTEGER NOT NULL,
	last_run_at TEXT,
	last_success_at TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_job_sources_enabled ON job_sources(enabled);
CREATE INDEX IF NOT EXISTS idx_job_sources_schedule ON job_sources(schedule);

CREATE TABLE IF NOT EXISTS job_source_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER NOT NULL,
	source_name TEXT NOT NULL,
	status TEXT NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT,
	duration_ms INTEGER,
	jobs_seen INTEGER NOT NULL DEFAULT 0,
	events_published INTEGER NOT NULL DEFAULT 0,
	error TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(source_id) REFERENCES job_sources(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_source_runs_source_id ON job_source_runs(source_id);
CREATE INDEX IF NOT EXISTS idx_job_source_runs_status ON job_source_runs(status);

CREATE TABLE IF NOT EXISTS source_checkpoints (
	source_id INTEGER PRIMARY KEY,
	checkpoint_json TEXT NOT NULL DEFAULT '{}',
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(source_id) REFERENCES job_sources(id) ON DELETE CASCADE
);
`,
	},
	{
		Version: 3,
		Name:    "create_parser_schema",
		SQL: `
ALTER TABLE job_descriptions ADD COLUMN parsed_metadata_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE job_descriptions ADD COLUMN parsed_at TEXT;

CREATE TABLE IF NOT EXISTS job_requirements (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id INTEGER NOT NULL UNIQUE,
	skills_json TEXT NOT NULL DEFAULT '[]',
	requirements_json TEXT NOT NULL DEFAULT '[]',
	responsibilities_json TEXT NOT NULL DEFAULT '[]',
	salary_min INTEGER,
	salary_max INTEGER,
	salary_currency TEXT,
	salary_period TEXT,
	remote_policy TEXT,
	seniority TEXT,
	employment_type TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_requirements_remote_policy ON job_requirements(remote_policy);
CREATE INDEX IF NOT EXISTS idx_job_requirements_seniority ON job_requirements(seniority);
CREATE INDEX IF NOT EXISTS idx_job_requirements_employment_type ON job_requirements(employment_type);
`,
	},
	{
		Version: 4,
		Name:    "create_matching_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS candidate_profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	headline TEXT,
	skills_json TEXT NOT NULL DEFAULT '[]',
	preferred_titles_json TEXT NOT NULL DEFAULT '[]',
	preferred_locations_json TEXT NOT NULL DEFAULT '[]',
	remote_preference TEXT,
	min_salary INTEGER,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS job_matches (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id INTEGER NOT NULL,
	candidate_profile_id INTEGER NOT NULL,
	score INTEGER NOT NULL,
	matched_skills_json TEXT NOT NULL DEFAULT '[]',
	missing_skills_json TEXT NOT NULL DEFAULT '[]',
	notes_json TEXT NOT NULL DEFAULT '[]',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	UNIQUE(job_id, candidate_profile_id),
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_matches_job_id ON job_matches(job_id);
CREATE INDEX IF NOT EXISTS idx_job_matches_score ON job_matches(score);

CREATE TABLE IF NOT EXISTS applications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id INTEGER NOT NULL UNIQUE,
	candidate_profile_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'ready_to_apply',
	match_score INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);
`,
	},
	{
		Version: 5,
		Name:    "create_notification_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS notification_channels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	webhook_url TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS notification_rules (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	event_subject TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	min_score INTEGER,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	channel_name TEXT NOT NULL,
	channel_type TEXT NOT NULL,
	event_id TEXT NOT NULL,
	event_subject TEXT NOT NULL,
	status TEXT NOT NULL,
	status_code INTEGER,
	error TEXT,
	response_body TEXT,
	delivered_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_event_id ON notification_deliveries(event_id);
CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status ON notification_deliveries(status);
`,
	},
	{
		Version: 6,
		Name:    "create_candidate_profile_detail_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS candidate_work_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER NOT NULL,
	company TEXT NOT NULL,
	title TEXT NOT NULL,
	location TEXT,
	start_date TEXT,
	end_date TEXT,
	current INTEGER NOT NULL DEFAULT 0,
	summary TEXT,
	highlights_json TEXT NOT NULL DEFAULT '[]',
	technologies_json TEXT NOT NULL DEFAULT '[]',
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS candidate_projects (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	role TEXT,
	url TEXT,
	summary TEXT,
	highlights_json TEXT NOT NULL DEFAULT '[]',
	technologies_json TEXT NOT NULL DEFAULT '[]',
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS candidate_education (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER NOT NULL,
	institution TEXT NOT NULL,
	degree TEXT,
	field TEXT,
	start_date TEXT,
	end_date TEXT,
	summary TEXT,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS candidate_certifications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	issuer TEXT,
	issued_at TEXT,
	expires_at TEXT,
	url TEXT,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS candidate_links (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER NOT NULL,
	label TEXT NOT NULL,
	url TEXT NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_candidate_work_history_profile ON candidate_work_history(candidate_profile_id);
CREATE INDEX IF NOT EXISTS idx_candidate_projects_profile ON candidate_projects(candidate_profile_id);
CREATE INDEX IF NOT EXISTS idx_candidate_education_profile ON candidate_education(candidate_profile_id);
CREATE INDEX IF NOT EXISTS idx_candidate_certifications_profile ON candidate_certifications(candidate_profile_id);
CREATE INDEX IF NOT EXISTS idx_candidate_links_profile ON candidate_links(candidate_profile_id);
`,
	},
	{
		Version: 7,
		Name:    "create_resume_document_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS documents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,
	format TEXT NOT NULL,
	path TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE IF NOT EXISTS resume_sources (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_profile_id INTEGER,
	name TEXT NOT NULL,
	format TEXT NOT NULL,
	document_id INTEGER NOT NULL,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE SET NULL,
	FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS resume_versions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	resume_source_id INTEGER NOT NULL,
	job_id INTEGER,
	document_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	notes TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(resume_source_id) REFERENCES resume_sources(id) ON DELETE CASCADE,
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE SET NULL,
	FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_documents_kind ON documents(kind);
CREATE INDEX IF NOT EXISTS idx_resume_sources_candidate ON resume_sources(candidate_profile_id);
CREATE INDEX IF NOT EXISTS idx_resume_versions_source ON resume_versions(resume_source_id);
`,
	},
	{
		Version: 8,
		Name:    "create_application_materials_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS application_materials (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	application_id INTEGER NOT NULL,
	job_id INTEGER NOT NULL,
	candidate_profile_id INTEGER NOT NULL,
	kind TEXT NOT NULL,
	document_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	notes TEXT,
	source_event_id TEXT,
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(application_id) REFERENCES applications(id) ON DELETE CASCADE,
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE,
	FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_application_materials_application ON application_materials(application_id);
CREATE INDEX IF NOT EXISTS idx_application_materials_job ON application_materials(job_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_application_materials_unique_draft
	ON application_materials(application_id, kind, source_event_id)
	WHERE source_event_id IS NOT NULL AND source_event_id != '';
`,
	},
	{
		Version: 9,
		Name:    "create_automation_handoff_schema",
		SQL: `
ALTER TABLE applications ADD COLUMN selected_resume_material_id INTEGER;
ALTER TABLE applications ADD COLUMN selected_cover_letter_material_id INTEGER;

CREATE TABLE IF NOT EXISTS automation_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	application_id INTEGER NOT NULL,
	job_id INTEGER NOT NULL,
	candidate_profile_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'requested',
	resume_material_id INTEGER NOT NULL,
	cover_letter_material_id INTEGER,
	final_url TEXT,
	error TEXT,
	requested_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	started_at TEXT,
	review_required_at TEXT,
	finished_at TEXT,
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(application_id) REFERENCES applications(id) ON DELETE CASCADE,
	FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
	FOREIGN KEY(candidate_profile_id) REFERENCES candidate_profiles(id) ON DELETE CASCADE,
	FOREIGN KEY(resume_material_id) REFERENCES application_materials(id) ON DELETE RESTRICT,
	FOREIGN KEY(cover_letter_material_id) REFERENCES application_materials(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_automation_runs_application ON automation_runs(application_id);
CREATE INDEX IF NOT EXISTS idx_automation_runs_status ON automation_runs(status);
`,
	},
	{
		Version: 10,
		Name:    "create_automation_log_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS automation_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	automation_run_id INTEGER NOT NULL,
	level TEXT NOT NULL DEFAULT 'info',
	message TEXT NOT NULL,
	details_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	FOREIGN KEY(automation_run_id) REFERENCES automation_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_automation_logs_run ON automation_logs(automation_run_id);
`,
	},
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	for _, migration := range Migrations {
		var exists int
		err := db.QueryRowContext(ctx, "SELECT 1 FROM schema_migrations WHERE version = ?", migration.Version).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %d: %w", migration.Version, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.Version, err)
		}

		if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES(?, ?)", migration.Version, migration.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}
	}

	return nil
}
