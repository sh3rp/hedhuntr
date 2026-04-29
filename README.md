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

It currently supports:

- Loading producer configuration from JSON.
- Fetching jobs from a `static` source for local testing.
- Fetching jobs from a Greenhouse job board.
- Normalizing listings into `JobDiscovered` event payloads.
- Creating stable idempotency keys.
- Connecting to NATS JetStream.
- Ensuring the `JOBS` stream exists for `jobs.>`.
- Publishing events to `jobs.discovered`.

## Repository Layout

```text
cmd/source-producer/             Source producer command
configs/source-producer.example.json
internal/config/                 Configuration loading
internal/events/                 Event envelopes and payloads
internal/producer/               Source producer orchestration
internal/sources/                Source adapters
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

- Add the SQLite schema and migration runner.
- Implement the Persistence Dispatcher for `jobs.discovered`.
- Add scheduler support for hourly and daily source pulls.
- Add source checkpoints and run history.
- Add the Description Fetcher Worker.
- Add the React/TypeScript dashboard shell.
