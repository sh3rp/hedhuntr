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
