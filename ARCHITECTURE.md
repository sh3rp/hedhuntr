# Architecture

## Overview

This application discovers job listings from job search aggregators and applicant tracking systems, captures job descriptions, stores job and application state in SQLite, and uses an event-driven workflow to enrich jobs, notify users, prepare application materials, and track interviews.

The system is designed as a local-first or small-team application. SQLite is the system of record for application state, while NATS JetStream is the message broker for asynchronous job ingestion and workflow events.

All backend services, workers, schedulers, source adapters, persistence code, and API handlers should be written in Go. The frontend dashboard should be written in React and TypeScript.

The application should keep human review points before any job application is submitted. Resume tuning and application answers must use only truthful candidate information.

## Goals

- Pull new job listings on hourly and daily schedules.
- Publish discovered jobs to JetStream as events.
- Persist normalized jobs and descriptions to SQLite.
- Fetch and parse full job descriptions.
- Match jobs against a candidate profile.
- Generate tailored resume and cover letter drafts for review.
- Notify configured channels when relevant jobs or workflow events occur.
- Assist with job applications without bypassing site protections.
- Track the full application and interview lifecycle.

## Non-Goals

- Bypassing CAPTCHAs, paywalls, login protections, or anti-bot systems.
- Silently submitting applications without user approval.
- Fabricating resume content, credentials, employment history, or application answers.
- Using scraping where a source's terms prohibit it.
- Treating SQLite as a high-concurrency multi-tenant database.

## Technology Stack

Backend:

- Go for all backend code.
- NATS JetStream for event streams, durable consumers, retries, and replay.
- SQLite for application state and local persistence.
- Go SQLite driver such as `modernc.org/sqlite` or `github.com/mattn/go-sqlite3`.
- Go migrations using a tool such as `golang-migrate`, `goose`, or a lightweight internal migration runner.
- Go HTTP API using the standard `net/http` package or a small router such as `chi`.
- Go background workers for scheduling, ingestion, dispatch, parsing, matching, notifications, and automation coordination.
- Playwright automation should be controlled from Go where practical, or isolated behind a Go-owned worker boundary if a non-Go browser automation runtime is required.

Frontend:

- React for the dashboard UI.
- TypeScript for all frontend code.
- Vite for local development and bundling unless the project later needs a larger framework.
- A typed API client generated from an OpenAPI schema or maintained as shared TypeScript interfaces.
- Operational screens for job discovery, job details, candidate profile, resume review, application queue, notifications, and interview tracking.

Interfaces:

- REST or JSON HTTP API between React and the Go backend.
- JSON event payloads on JetStream.
- SQLite repositories hidden behind Go interfaces so database access stays testable and replaceable.

## Discrete Components

The architecture is split into discrete components with clear ownership. Components may initially run as separate Go commands in one repository and later be deployed independently if needed.

### Web App

Type: React/TypeScript frontend.

Responsibilities:

- Render the job dashboard, job detail views, application queue, resume review screens, profile settings, notifications, and interview tracker.
- Call the Go API over JSON HTTP.
- Provide human approval flows for generated resumes, cover letters, application answers, and final submissions.
- Show source run status, event processing state, notification results, and automation failures.

Owns:

- UI routes.
- Frontend state management.
- TypeScript API client.
- Form validation and review workflows.

Does not own:

- Direct SQLite access.
- Direct JetStream publishing.
- Background job execution.

### API Service

Type: Go HTTP service.

Responsibilities:

- Serve the React application in production or expose an API for a separately hosted frontend.
- Provide JSON endpoints for jobs, applications, candidate profile, documents, interviews, notifications, and settings.
- Enforce validation and user approval gates.
- Read and write application state through repository interfaces.
- Publish user-triggered events to JetStream when needed.

Owns:

- HTTP routing.
- Request validation.
- Application-facing read models.
- User actions such as save job, approve resume, mark applied, schedule interview, and retry automation.

Does not own:

- Periodic source pulls.
- Long-running enrichment work.
- Browser automation execution.

### Scheduler Service

Type: Go background service.

Responsibilities:

- Run hourly and daily source schedules.
- Load source configuration from SQLite.
- Track source run metadata, cursors, and checkpoints.
- Trigger source producers for configured searches, aggregators, ATS systems, company career pages, and manual import queues.

Owns:

- Schedule definitions.
- Source run lifecycle records.
- Checkpoint handoff to producers.

Example schedules:

- Hourly: saved searches, high-priority sources, recently active companies.
- Daily: broad aggregator searches, lower-priority company pages, stale source refreshes.

### Source Producer Service

Type: Go worker service.

Responsibilities:

- Fetch listings from supported external sources.
- Normalize listings into shared event payloads.
- Generate stable idempotency keys.
- Publish `JobDiscovered` events to JetStream.
- Avoid direct job persistence except source run and checkpoint updates where needed.

Owns:

- Source adapters.
- Rate limits and polite fetch behavior.
- Listing normalization.
- `jobs.discovered` event production.

Supported source types:

- Official aggregator APIs where available.
- ATS postings from Greenhouse, Lever, Ashby, Workday, and similar systems.
- Company career pages where access is permitted.
- Manual URL imports.

### JetStream Broker

Type: NATS JetStream infrastructure component.

Responsibilities:

- Persist event streams.
- Deliver events to durable consumers.
- Support retries, backoff, dead-letter handling, and replay.

Owns:

- Streams.
- Consumer definitions.
- Message retention policy.
- Acknowledgement and retry behavior.

Recommended streams:

| Stream | Subjects |
| --- | --- |
| `JOBS` | `jobs.>` |
| `APPLICATIONS` | `applications.>` |
| `NOTIFICATIONS` | `notifications.>` |
| `AUTOMATION` | `automation.>` |

Reliability settings:

- Durable consumers.
- Explicit acknowledgements.
- Retry and backoff policies.
- Dead-letter subjects.
- Event idempotency keys.
- Correlation IDs.
- Replayable streams for debugging.

### Persistence Dispatcher

Type: Go JetStream consumer.

Responsibilities:

- Consume `jobs.discovered`.
- Deduplicate jobs.
- Insert or update records in SQLite.
- Store consumed event metadata for auditability and idempotency.
- Emit downstream events for description fetching, matching, and notifications.

Owns:

- Job deduplication.
- Job persistence.
- Initial job lifecycle state.
- `jobs.saved` and `jobs.description.fetch.requested` event production.

Deduplication should use a combination of:

- Source and external job ID.
- Canonical application URL.
- Normalized company, title, location, and source hash.

### Description Fetcher Worker

Type: Go JetStream consumer.

Responsibilities:

- Consume `jobs.description.fetch.requested`.
- Fetch full job description text and optional raw HTML.
- Store fetched content in SQLite.
- Emit `jobs.description.fetched`.

Owns:

- Description retrieval.
- HTML-to-text extraction.
- Fetch failure classification.
- Raw description persistence.

### Parser / Enrichment Worker

Type: Go JetStream consumer.

Responsibilities:

- Consume `jobs.description.fetched`.
- Extract responsibilities, requirements, skills, salary, seniority, location policy, and employment type.
- Store parsed metadata in SQLite.
- Update the job search index.
- Emit `jobs.parsed`.

Owns:

- Job description parsing.
- Structured metadata extraction.
- Search index updates.
- Parser confidence and error reporting.

### Matching / Resume Worker

Type: Go JetStream consumer.

Responsibilities:

- Consume `jobs.parsed`.
- Score job fit against candidate profile, preferences, and resume sources.
- Identify matched skills, missing requirements, and risk notes.
- Generate tailored resume and cover letter drafts when configured.
- Store generated document metadata and files.
- Emit `jobs.matched` and, when appropriate, `applications.ready`.

Owns:

- Candidate-to-job fit scoring.
- Resume draft generation.
- Cover letter draft generation.
- Truthfulness checks against approved candidate source material.

Generated materials must be based only on the candidate's source profile, resume history, projects, and approved facts.

### Notification Worker

Type: Go JetStream consumer.

Responsibilities:

- Consume notification-worthy events.
- Apply notification rules.
- Send messages to Discord, Slack, email, and in-app channels.
- Store delivery attempts and results.
- Emit `notifications.sent` or `notifications.failed`.

Owns:

- Notification routing.
- Channel adapters.
- Delivery retries.
- Notification templates.

Potential inputs:

- `jobs.saved`
- `jobs.matched`
- `applications.ready`
- `applications.submitted`
- `interviews.updated`
- `automation.failed`
- `notifications.requested`

### Automation Worker

Type: Go worker service.

Responsibilities:

- Consume `applications.ready`.
- Prepare application packets.
- Coordinate Playwright-assisted form filling for supported sites.
- Attach approved resume and cover letter documents.
- Pause for user review before final submission.
- Store automation logs, outcomes, and confirmation details when available.
- Emit application lifecycle events.

Owns:

- Automation run state.
- ATS automation adapters.
- Browser automation logs.
- Application packet preparation.

Supported adapters can be added incrementally:

- Greenhouse.
- Lever.
- Ashby.
- Workday.
- Generic form adapter.

### SQLite Store

Type: Embedded database.

Responsibilities:

- Store durable application state.
- Support full-text search with FTS5.
- Provide audit records for event processing, automation, and notifications.

Owns:

- Candidate profile state.
- Jobs and descriptions.
- Applications and interviews.
- Source configuration and checkpoints.
- Notification configuration and delivery records.
- Automation logs.
- Document metadata.

Does not own:

- Event delivery.
- Long-term binary document storage beyond local file references.

### Document Store

Type: Local filesystem initially, object storage later if needed.

Responsibilities:

- Store generated resumes, cover letters, source resumes, and application packet artifacts.
- Preserve the exact files used for each application.

Owns:

- Document paths.
- File checksums.
- Versioned generated artifacts.

SQLite stores document metadata and paths, not large binary blobs.

### External Integrations

Type: Third-party systems called by Go adapters.

Responsibilities:

- Provide job listing data.
- Receive notifications.
- Host application forms.

Examples:

- Job aggregators and ATS platforms.
- Discord webhooks.
- Slack webhook/API.
- Email SMTP or provider APIs.
- Calendar and email APIs in later phases.

## Event Model

All events should use a common envelope.

```json
{
  "event_id": "uuid",
  "event_type": "JobDiscovered",
  "event_version": 1,
  "occurred_at": "2026-04-28T12:00:00Z",
  "source": "greenhouse",
  "correlation_id": "uuid",
  "idempotency_key": "source:external-job-id",
  "payload": {}
}
```

Core subjects:

| Subject | Purpose |
| --- | --- |
| `jobs.discovered` | A producer found a job listing. |
| `jobs.saved` | Dispatcher persisted a new or updated job. |
| `jobs.description.fetch.requested` | A job needs its full description fetched. |
| `jobs.description.fetched` | Full description content was fetched. |
| `jobs.parsed` | Description metadata was extracted. |
| `jobs.matched` | Job was scored against the candidate profile. |
| `applications.ready` | An application packet is ready for user review. |
| `applications.submitted` | Application was submitted or marked submitted. |
| `interviews.updated` | Interview state changed. |
| `notifications.requested` | A message should be routed to configured channels. |
| `notifications.sent` | A notification was delivered. |
| `notifications.failed` | Notification delivery failed. |
| `automation.failed` | Automation failed and needs attention. |

## SQLite Database

SQLite is the application state store. It should run in WAL mode with foreign keys enabled.

Recommended pragmas:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

Use SQLite for:

- Jobs and job descriptions.
- Candidate profile state.
- Resume source material and generated document metadata.
- Application lifecycle state.
- Interview tracking.
- Source schedules, checkpoints, and run history.
- Notification configuration and delivery logs.
- Automation logs and outcomes.
- Event processing audit records.

Store large generated documents as files on disk or object storage, with metadata and paths in SQLite.

Use FTS5 for search across:

- Job title.
- Company.
- Description text.
- Skills.
- Requirements.
- Notes.

### Core Tables

Candidate and document tables:

- `users`
- `candidate_profiles`
- `resume_sources`
- `resume_versions`
- `documents`

Job ingestion tables:

- `job_sources`
- `job_source_runs`
- `source_checkpoints`
- `jobs`
- `job_descriptions`
- `job_requirements`
- `job_events`
- `jobs_fts`

Application workflow tables:

- `applications`
- `application_events`
- `interviews`
- `contacts`
- `automation_runs`
- `automation_logs`

Notification tables:

- `notification_channels`
- `notification_rules`
- `notification_deliveries`
- `message_outbox`

### Job Statuses

Suggested job and application statuses:

- `discovered`
- `saved`
- `description_fetched`
- `parsed`
- `matched`
- `ready_to_apply`
- `applied`
- `recruiter_screen`
- `technical_interview`
- `onsite`
- `offer`
- `rejected`
- `withdrawn`

## End-to-End Flow

1. Scheduler starts an hourly or daily source run.
2. Producer fetches new listings from a configured source.
3. Producer publishes one `jobs.discovered` event per listing.
4. Dispatcher consumes `jobs.discovered`, deduplicates it, and persists it to SQLite.
5. Dispatcher emits `jobs.saved` and `jobs.description.fetch.requested`.
6. Description fetcher retrieves full posting content and emits `jobs.description.fetched`.
7. Parser extracts structured metadata and emits `jobs.parsed`.
8. Matching worker scores the job and optionally creates resume and cover letter drafts.
9. Notification dispatcher sends relevant events to Discord, Slack, email, or in-app channels.
10. User reviews the job, tuned resume, and application packet.
11. Automation worker assists with form filling where supported.
12. Application and interview updates are tracked in SQLite and emitted as events.

## MVP Scope

The first version should include:

- Go backend service with HTTP API.
- React/TypeScript frontend dashboard.
- SQLite database with migrations.
- NATS JetStream setup for `jobs.>`, `applications.>`, `notifications.>`, and `automation.>`.
- Hourly scheduler.
- Daily scheduler.
- One or two source producers.
- Dispatcher that persists `jobs.discovered`.
- Description fetcher.
- Parser/enrichment worker.
- Candidate profile storage.
- Basic job matching.
- Discord and email notifications.
- Dashboard for discovered, saved, matched, ready-to-apply, applied, and interview states.

## Later Phases

- Slack notifications.
- Email and calendar sync.
- Chrome extension for one-click job capture.
- More ATS-specific automation adapters.
- Recruiter/contact CRM.
- Response-rate analytics.
- Follow-up reminders.
- Interview prep summaries.
- Multi-resume version control.
- PostgreSQL migration path if multi-user scale becomes necessary.

## Operational Guardrails

- Prefer official APIs and permitted source integrations.
- Do not bypass anti-bot or access controls.
- Require user approval before submitting applications.
- Log every automation action.
- Preserve the exact resume and cover letter used for each application.
- Store source URLs and application URLs for auditability.
- Make event consumers idempotent.
- Use dead-letter queues for repeated processing failures.
- Keep database access behind repository or service interfaces so SQLite can be replaced later if needed.
