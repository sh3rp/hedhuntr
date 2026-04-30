# Hedhuntr Deployment

## Docker Compose

Build and start the full local stack:

```bash
docker compose up --build
```

The compose stack starts NATS JetStream, the Go API, the React web UI through nginx, the scheduler, and all durable workers. SQLite data, generated documents, browser handoff files, and NATS stream data are kept in named volumes.

Open the UI at:

```text
http://localhost:5173
```

The API is also exposed at:

```text
http://localhost:8080
```

The production-style configs live in `configs/deploy`. They use `nats://nats:4222`, `/data/hedhuntr.db`, `/data/documents`, and `/data/automation-handoffs` so containers can share state through the `hedhuntr-data` volume.

## Systemd

The unit files in `deploy/systemd` assume:

- binaries are installed in `/opt/hedhuntr/bin`
- configs are installed in `/etc/hedhuntr`
- runtime data is stored somewhere writable by the `hedhuntr` user, usually `/var/lib/hedhuntr`
- NATS is already installed as `nats.service`

Example worker enablement:

```bash
sudo systemctl enable --now hedhuntr-api.service
sudo systemctl enable --now hedhuntr-scheduler.service
sudo systemctl enable --now hedhuntr-worker@persistence-dispatcher.service
sudo systemctl enable --now hedhuntr-worker@description-fetcher.service
sudo systemctl enable --now hedhuntr-worker@parser-worker.service
sudo systemctl enable --now hedhuntr-worker@matching-worker.service
sudo systemctl enable --now hedhuntr-worker@resume-tuning-worker.service
sudo systemctl enable --now hedhuntr-worker@notification-worker.service
sudo systemctl enable --now hedhuntr-worker@automation-worker.service
```

## Assisted Browser Mode

Container deployment leaves browser execution disabled. On a workstation, set the automation worker to `assisted-browser` or set `automation.browser.enabled=true`, then configure a local browser opener such as `xdg-open`, `open`, or a browser executable. The worker writes a JSON handoff file and launches the application URL, but it still marks the run `review_required` and does not submit applications.
