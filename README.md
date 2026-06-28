# Borg Backup Manager

A modern web-based management interface for [BorgBackup](https://www.borgbackup.org/), rebuilt as a **Go + Vue 3** monorepo.

## Features

- Web dashboard with backup stats, activity, and live progress (SSE)
- Sources, destinations, and backup jobs (local, SSH, S3)
- Real-time backup logs and job scheduling (cron)
- Archive browser, restore, download, and batch delete
- Repository health checks, break-lock, and compact
- SSH key management with encrypted storage
- OIDC authentication, API keys, audit log
- SMTP and webhook notifications
- Config export/import, Prometheus metrics, health probes

## Stack

| Layer | Technology |
|-------|------------|
| Backend | Go, chi, SQLite, goose |
| Frontend | Vue 3, Vite, TypeScript, Pinia |
| Backup | BorgBackup CLI |

## Quick Start (development)

```bash
cp .env.example .env
# Edit SECRET_KEY and set AUTH_DISABLED=true for local dev

# Terminal 1 — API server
make dev-backend

# Terminal 2 — Vue dev server (proxies API)
make dev-frontend
```

Open http://localhost:5173

## Production build

```bash
make build
./bin/backup-manager
```

The Go binary embeds the Vue SPA and serves it on port 5001.

## Docker

```bash
make docker-build
docker compose up -d
```

Requires `SYS_ADMIN`, `/dev/fuse`, and Borg/sshfs/rclone in the container for full remote backup support.

## API

REST API under `/api/v1`. Health at `/health/live`, `/health/ready`, `/health`. Metrics at `/metrics`.

See [docs/api.md](docs/api.md) for endpoint reference.

## Project layout

```
backend/     Go API, Borg engine, scheduler
frontend/    Vue 3 SPA
docs/        API and technical documentation
```
