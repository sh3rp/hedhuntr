# Hedhuntr

Hedhuntr is an event-driven job search and application workflow system. It discovers job listings from job aggregators and applicant tracking systems, captures job descriptions, stores application state in SQLite, and coordinates enrichment, notifications, resume tailoring, assisted applications, and interview tracking through NATS JetStream.

The backend is written in Go. The frontend will be React and TypeScript.

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

The initial implementation starts with the Source Producer Service.

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

## Repository Layout

```text
cmd/source-producer/             Source producer command
cmd/persistence-dispatcher/       Persistence dispatcher command
cmd/scheduler/                    Scheduler command
cmd/description-fetcher/          Description fetcher command
cmd/parser-worker/                Parser / enrichment worker command
cmd/matching-worker/              Matching worker command
configs/source-producer.example.json
configs/persistence-dispatcher.example.json
configs/scheduler.example.json
configs/description-fetcher.example.json
configs/parser-worker.example.json
configs/matching-worker.example.json
internal/config/                 Configuration loading
internal/events/                 Event envelopes and payloads
internal/broker/                 JetStream setup helpers
internal/descriptionfetcher/      Description fetcher orchestration and text extraction
internal/dispatcher/             Persistence dispatcher orchestration
internal/matcher/                Candidate-to-job scoring
internal/matchingworker/         Matching worker orchestration
internal/parser/                 Deterministic job description parser
internal/parserworker/           Parser worker orchestration
internal/producer/               Source producer orchestration
internal/sources/                Source adapters
internal/store/                  SQLite migrations and repositories
ARCHITECTURE.md                  System architecture
README.md                        Project overview
```

## Requirements

- Go 1.22 or newer.
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

- Add explicit candidate profile management commands or API endpoints.
- Add resume source storage and document generation.
- Add notification worker for `jobs.matched` and `applications.ready`.
- Add the React/TypeScript dashboard shell.
