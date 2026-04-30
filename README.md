# Hedhuntr

Hedhuntr is an event-driven job search and application workflow system. It discovers job listings from job aggregators and applicant tracking systems, captures job descriptions, stores application state in SQLite, and coordinates enrichment, notifications, resume tailoring, assisted applications, and interview tracking through NATS JetStream.

The backend is written in Go. The frontend is React and TypeScript with an Electron desktop shell.

## Goals

- Pull new job listings on hourly and daily schedules.
- Publish discovered jobs to NATS JetStream.
- Persist normalized jobs and job descriptions to SQLite.
- Parse job descriptions into structured requirements and skills.
- Match jobs against a candidate profile.
- Generate truthful resume and cover letter drafts for review.
- Notify configured channels such as Discord, Slack, and email.
- Assist with applications while keeping a human approval step before submission.
- Track applications, interview stages, outcomes, and follow-ups.

## Architecture

The full design is documented in [ARCHITECTURE.md](ARCHITECTURE.md).

Primary components:

- React/TypeScript Web App
- Go API Service
- Go Scheduler Service
- Go Source Producer Service
- NATS JetStream Broker
- Go Persistence Dispatcher
- Go Description Fetcher Worker
- Go Parser / Enrichment Worker
- Go Matching / Resume Worker
- Go Notification Worker
- Go Automation Worker
- SQLite Store
- Document Store
- External Integrations

## Current Implementation

The initial implementation covers the ingestion pipeline, worker commands, API service, web UI, and desktop shell.

The Source Producer currently supports:

- Loading producer configuration from JSON.
- Fetching jobs from a `static` source for local testing.
- Fetching jobs from a Greenhouse job board.
- Normalizing listings into `JobDiscovered` event payloads.
- Creating stable idempotency keys.
- Connecting to NATS JetStream.
- Ensuring the `JOBS` stream exists for `jobs.>`.
- Publishing events to `jobs.discovered`.

The Persistence Dispatcher currently supports:

- Creating and migrating the SQLite database.
- Consuming `jobs.discovered` from JetStream with a durable pull consumer.
- Deduplicating by event ID, idempotency key, source/external ID, and canonical URL.
- Persisting jobs, descriptions, event audit records, and FTS search rows.
- Publishing `jobs.saved`.
- Publishing `jobs.description.fetch.requested` when the discovered job has no description text.

The Scheduler Service currently supports:

- Loading scheduled source definitions from JSON.
- Referencing the source producer config for adapter settings.
- Creating and updating `job_sources`, `job_source_runs`, and `source_checkpoints` schema.
- Calculating due hourly/daily source runs from SQLite state.
- Running due sources through the Source Producer Service.
- Recording success/failure, duration, jobs seen, and events published.
- Running continuously or once with `-run-once`.

The Description Fetcher Worker currently supports:

- Consuming `jobs.description.fetch.requested` from JetStream with a durable pull consumer.
- Loading the target job from SQLite.
- Fetching the application URL or source URL over HTTP.
- Extracting readable text from HTML job pages.
- Persisting fetched text and optional raw HTML.
- Updating job status to `description_fetched`.
- Publishing `jobs.description.fetched`.

The Parser / Enrichment Worker currently supports:

- Consuming `jobs.description.fetched` from JetStream with a durable pull consumer.
- Extracting skills with a deterministic local ruleset.
- Extracting requirements and responsibilities sections.
- Extracting salary hints, remote policy, seniority, and employment type.
- Persisting parsed metadata to `job_requirements` and `job_descriptions`.
- Updating job status to `parsed`.
- Publishing `jobs.parsed`.

The Matching Worker currently supports:

- Consuming `jobs.parsed` from JetStream with a durable pull consumer.
- Loading a candidate profile from SQLite, or creating a default local profile when none exists.
- Scoring jobs against candidate skills, title preferences, location preferences, remote preference, and salary floor.
- Persisting match scores, matched skills, missing skills, and notes.
- Publishing `jobs.matched`.
- Creating `ready_to_apply` application records and publishing `applications.ready` when a score meets the configured threshold.

The Notification Worker currently supports:

- Consuming `jobs.matched` and `applications.ready` from JetStream with durable pull consumers.
- Filtering `jobs.matched` notifications by score threshold.
- Sending Discord webhook messages.
- Sending Slack webhook messages.
- Persisting notification channels, rules, delivery attempts, failures, status codes, and response bodies.

The Profile Import command currently supports:

- Loading a candidate profile from JSON.
- Validating required profile fields.
- Persisting core profile preferences used by matching.
- Persisting work history, projects, education, certifications, and links.
- Printing the stored profile after import for verification.

The Resume command currently supports:

- Importing a base resume Markdown file.
- Storing the resume artifact under a local document directory.
- Recording document metadata, checksum, and size in SQLite.
- Creating resume source records.
- Listing stored resume sources.

The Resume Tuning Worker currently supports:

- Consuming `applications.ready` from JetStream with a durable pull consumer.
- Loading the ready application, parsed job context, candidate profile, and base resume source.
- Generating deterministic Markdown resume and cover letter drafts from stored candidate data.
- Ranking stored work history, projects, and certifications against job skills.
- Rendering relevant technologies, education, certifications, and links into tailored resume drafts.
- Persisting generated documents under local document storage.
- Creating draft `resume_versions` and `application_materials` records for human review.
- Publishing `applications.materials.drafted`.

The Automation Worker currently supports:

- Consuming `automation.run.requested` from JetStream with a durable pull consumer.
- Loading approved automation packets from SQLite.
- Recording automation run status and audit log entries.
- Preparing packet-only runs and stopping at `review_required`.
- Publishing `automation.run.started`, `automation.run.review_required`, and `automation.run.failed`.
- Never submitting an application.

The API Service currently supports:

- Serving dashboard data from SQLite over HTTP.
- Exposing health, jobs, pipeline, profile, resume source, notification, and worker endpoints.
- Updating candidate profile core fields and structured sections through the API.
- Reporting candidate profile completeness and quality checks for matching and resume tuning readiness.
- Exposing review queue endpoints for generated application materials.
- Updating generated material status for approve, reject, needs-changes, and regeneration-requested actions.
- Creating automation handoff packets from approved application materials.
- Exposing automation run state, logs, and run control actions.
- Tracking interviews, interview status changes, follow-up tasks, and task completion.
- Publishing durable automation handoff events to JetStream.
- Providing WebSocket subscriptions for React and Electron clients.
- Subscribing to NATS workflow events and broadcasting live dashboard updates over WebSockets.
- Enforcing configured HTTP/WebSocket origins.
- Falling back gracefully in the UI when the API is unavailable.

The React/Electron UI currently supports:

- Loading live dashboard data from the Go API.
- Displaying job pipeline status, match scores, notifications, worker state, candidate profile, and resume sources.
- Editing candidate profile name, headline, skills, preferences, salary floor, work history, projects, education, certifications, and links.
- Showing candidate profile completeness score and missing profile inputs.
- Reviewing generated resume and cover letter drafts with rendered preview, raw Markdown, structural checks, section summaries, keyword chips, links, and generator review notes.
- Approving, rejecting, requesting changes, or requesting regeneration for generated materials.
- Approving reviewed materials for an assisted automation handoff.
- Viewing automation runs, logs, final URLs, and review-required state.
- Marking automation runs submitted, failed, or retrying them with durable worker events.
- Scheduling interviews, updating interview status, adding follow-up tasks, and marking tasks done or open.
- Running as a browser UI through Vite.
- Running as a desktop shell through Electron.

## Repository Layout

```text
cmd/source-producer/             Source producer command
cmd/persistence-dispatcher/       Persistence dispatcher command
cmd/scheduler/                    Scheduler command
cmd/description-fetcher/          Description fetcher command
cmd/parser-worker/                Parser / enrichment worker command
cmd/matching-worker/              Matching worker command
cmd/notification-worker/          Notification worker command
cmd/resume-tuning-worker/         Resume and cover letter draft worker command
cmd/automation-worker/            Automation handoff worker command
cmd/api/                          API service command
cmd/profile/                      Candidate profile import command
cmd/resume/                       Resume source import/list command
configs/candidate-profile.example.json
configs/api.example.json
configs/source-producer.example.json
configs/persistence-dispatcher.example.json
configs/scheduler.example.json
configs/description-fetcher.example.json
configs/parser-worker.example.json
configs/matching-worker.example.json
configs/notification-worker.example.json
configs/resume-tuning-worker.example.json
configs/automation-worker.example.json
examples/resume.example.md
electron/                         Electron desktop shell
internal/config/                 Configuration loading
internal/api/                    HTTP and WebSocket API service
internal/automationworker/       Automation handoff worker orchestration
internal/document/               Local document storage helpers
internal/events/                 Event envelopes and payloads
internal/broker/                 JetStream setup helpers
internal/descriptionfetcher/      Description fetcher orchestration and text extraction
internal/dispatcher/             Persistence dispatcher orchestration
internal/matcher/                Candidate-to-job scoring
internal/matchingworker/         Matching worker orchestration
internal/notification/           Notification formatting and webhook senders
internal/notificationworker/     Notification worker orchestration
internal/parser/                 Deterministic job description parser
internal/parserworker/           Parser worker orchestration
internal/profile/                Candidate profile model and validation
internal/producer/               Source producer orchestration
internal/resumetuner/            Deterministic resume and cover letter drafting
internal/resumetuningworker/     Resume tuning worker orchestration
internal/sources/                Source adapters
internal/store/                  SQLite migrations and repositories
src/                             React/TypeScript frontend
ARCHITECTURE.md                  System architecture
README.md                        Project overview
```

## Requirements

- Go 1.22 or newer.
- Node.js 20 or newer.
- NATS server with JetStream enabled.

Example local NATS server:

```bash
nats-server -js
```

## Running the Source Producer

Install dependencies:

```bash
go mod tidy
```

Run tests:

```bash
go test ./...
```

Run the source producer with the example config:

```bash
go run ./cmd/source-producer -config configs/source-producer.example.json
```

The example config includes an enabled `static` source and a disabled Greenhouse source. To use Greenhouse, update `configs/source-producer.example.json` with a real Greenhouse `board_token` and set that source to `enabled: true`.

Run the persistence dispatcher continuously:

```bash
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json
```

Process one pending discovery event and exit:

```bash
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
```

Local smoke test flow:

```bash
nats-server -js
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
```

## Running the Scheduler

Run due scheduled sources once and exit:

```bash
go run ./cmd/scheduler -config configs/scheduler.example.json -run-once
```

Run the scheduler continuously:

```bash
go run ./cmd/scheduler -config configs/scheduler.example.json
```

The scheduler uses `configs/scheduler.example.json` for schedule metadata and references `configs/source-producer.example.json` for source adapter settings.

## Running the Description Fetcher

Run the description fetcher continuously:

```bash
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json
```

Process one pending description fetch request and exit:

```bash
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
```

Extended local smoke test flow:

```bash
nats-server -js
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
```

## Running the Parser Worker

Run the parser worker continuously:

```bash
go run ./cmd/parser-worker -config configs/parser-worker.example.json
```

Process one pending fetched-description event and exit:

```bash
go run ./cmd/parser-worker -config configs/parser-worker.example.json -max-messages 1
```

Parser worker smoke test flow:

```bash
nats-server -js
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
go run ./cmd/parser-worker -config configs/parser-worker.example.json -max-messages 1
```

## Running the Matching Worker

Run the matching worker continuously:

```bash
go run ./cmd/matching-worker -config configs/matching-worker.example.json
```

Process one pending parsed-job event and exit:

```bash
go run ./cmd/matching-worker -config configs/matching-worker.example.json -max-messages 1
```

Full local pipeline smoke test flow:

```bash
nats-server -js
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
go run ./cmd/parser-worker -config configs/parser-worker.example.json -max-messages 1
go run ./cmd/matching-worker -config configs/matching-worker.example.json -max-messages 1
```

## Running the Notification Worker

Run the notification worker continuously:

```bash
go run ./cmd/notification-worker -config configs/notification-worker.example.json
```

Process one pending notification-worthy event and exit:

```bash
go run ./cmd/notification-worker -config configs/notification-worker.example.json -max-messages 1
```

Notification worker pipeline flow:

```bash
nats-server -js
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
go run ./cmd/parser-worker -config configs/parser-worker.example.json -max-messages 1
go run ./cmd/matching-worker -config configs/matching-worker.example.json -max-messages 1
go run ./cmd/notification-worker -config configs/notification-worker.example.json -max-messages 2
```

The example notification channels are disabled by default. Enable a channel and replace its webhook URL before expecting outbound Discord or Slack messages.

## Importing a Candidate Profile

Import the example candidate profile into SQLite:

```bash
go run ./cmd/profile -db hedhuntr.db -profile configs/candidate-profile.example.json
```

Import and print the stored profile:

```bash
go run ./cmd/profile -db hedhuntr.db -profile configs/candidate-profile.example.json -print
```

The matching worker uses the first stored candidate profile when `candidate_profile_id` is `0` in `configs/matching-worker.example.json`.

## Importing a Resume Source

Import the example base resume into local document storage and SQLite:

```bash
go run ./cmd/resume import -db hedhuntr.db -documents data/documents -file examples/resume.example.md -name "Base Resume"
```

Attach the resume source to a candidate profile:

```bash
go run ./cmd/resume import -db hedhuntr.db -documents data/documents -file examples/resume.example.md -name "Base Resume" -candidate-profile-id 1
```

List stored resume sources:

```bash
go run ./cmd/resume list -db hedhuntr.db
```

## Running the Resume Tuning Worker

Run the resume tuning worker continuously:

```bash
go run ./cmd/resume-tuning-worker -config configs/resume-tuning-worker.example.json
```

Process one pending ready-application event and exit:

```bash
go run ./cmd/resume-tuning-worker -config configs/resume-tuning-worker.example.json -max-messages 1
```

Resume tuning requires a candidate profile, at least one imported resume source, and an `applications.ready` event. Generated drafts are stored as Markdown documents and remain in `draft` status for human review.

Application materials flow:

```bash
nats-server -js
go run ./cmd/profile -db hedhuntr.db -profile configs/candidate-profile.example.json
go run ./cmd/resume import -db hedhuntr.db -documents data/documents -file examples/resume.example.md -name "Base Resume" -candidate-profile-id 1
go run ./cmd/source-producer -config configs/source-producer.example.json
go run ./cmd/persistence-dispatcher -config configs/persistence-dispatcher.example.json -max-messages 1
go run ./cmd/description-fetcher -config configs/description-fetcher.example.json -max-messages 1
go run ./cmd/parser-worker -config configs/parser-worker.example.json -max-messages 1
go run ./cmd/matching-worker -config configs/matching-worker.example.json -max-messages 1
go run ./cmd/resume-tuning-worker -config configs/resume-tuning-worker.example.json -max-messages 1
```

## Running the Automation Worker

Run the automation worker continuously:

```bash
go run ./cmd/automation-worker -config configs/automation-worker.example.json
```

Process one pending automation request and exit:

```bash
go run ./cmd/automation-worker -config configs/automation-worker.example.json -max-messages 1
```

The current automation worker runs in `packet-only` mode. It loads the approved application packet, records audit logs, marks the run `review_required`, and stops before any final submission.

## Running the API Service

Start the Go API service:

```bash
go run ./cmd/api -config configs/api.example.json
```

The example config listens on `127.0.0.1:8080`, reads from `hedhuntr.db`, allows the local Vite frontend, and allows `http://zoe.ts.shep.run:5173`.
It also enables the realtime bridge, which subscribes to NATS subjects such as `jobs.saved`, `jobs.parsed`, `jobs.matched`, `applications.ready`, `applications.materials.drafted`, and `automation.run.review_required`.

Available endpoints:

```text
GET /api/health
GET /api/jobs
GET /api/pipeline
GET /api/profile
GET /api/profile/quality
PUT /api/profile
GET /api/resume-sources
GET /api/review/applications
POST /api/review/materials/{id}/status
POST /api/applications/{id}/approve-automation
GET /api/applications/{id}/packet
GET /api/automation/runs
POST /api/automation/runs/{id}/mark-submitted
POST /api/automation/runs/{id}/fail
POST /api/automation/runs/{id}/retry
GET /api/interviews
POST /api/interviews
POST /api/interviews/{id}/status
POST /api/interviews/{id}/tasks
POST /api/interview-tasks/{id}/status
GET /api/notifications
GET /api/workers
GET /ws
```

The WebSocket endpoint accepts dashboard clients and sends a subscription acknowledgement. It is intended for browser and Electron clients; both must connect from an allowed origin.
When the realtime bridge is enabled, the API forwards matching NATS events to subscribed clients with topic names such as `jobs`, `applications`, and `notifications`.
Review status changes are also broadcast to `applications` WebSocket subscribers.
Automation handoff creates an `automation_runs` record, selects the approved resume and optional approved cover letter, and publishes `applications.automation.approved` plus `automation.run.requested` to JetStream. The automation worker must still stop before final submission.

## Running the React UI

Install frontend dependencies:

```bash
npm install
```

Run the browser UI:

```bash
npm run dev
```

The Vite server is configured to allow `zoe.ts.shep.run`, so the UI can be reached at:

```text
http://localhost:5173/
http://zoe.ts.shep.run:5173/
```

Configure API URLs with environment variables when the API is not on the default local address:

```bash
VITE_HEDHUNTR_API_URL=http://localhost:8080 npm run dev
```

The WebSocket URL defaults from `VITE_HEDHUNTR_API_URL` when it is set. It can be overridden explicitly:

```bash
VITE_HEDHUNTR_WS_URL=ws://localhost:8080/ws npm run dev
```

## Running the Electron App

Build the web and Electron bundles:

```bash
npm run build
```

Run the desktop shell during development:

```bash
npm run electron:dev
```

## Event Output

The source producer publishes `JobDiscovered` events to:

```text
jobs.discovered
```

Events use the shared envelope:

```json
{
  "event_id": "uuid-or-stable-id",
  "event_type": "JobDiscovered",
  "event_version": 1,
  "occurred_at": "2026-04-28T12:00:00Z",
  "source": "example-greenhouse",
  "correlation_id": "correlation-id",
  "idempotency_key": "source:external-job-id",
  "payload": {}
}
```

## Guardrails

- Prefer official APIs and permitted integrations.
- Do not bypass CAPTCHAs, authentication, paywalls, or anti-bot protections.
- Do not silently submit applications.
- Keep user approval before final submission.
- Do not generate false resume or application content.
- Preserve the exact resume and cover letter used for each application.

## Next Implementation Steps

- Add ATS-specific automation adapters for supported application systems.
- Add notification settings UI.
