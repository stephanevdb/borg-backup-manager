# Technical Documentation

## Architecture

Borg Backup Manager is a Go + Vue 3 monorepo:

- **backend/** — REST API (`/api/v1`), Borg subprocess engine, SQLite persistence, cron scheduler, SSE streaming
- **frontend/** — Vue 3 SPA embedded into the Go binary for production

## Execution model

1. Manual or scheduled job triggers create a `backup_runs` row with `status=queued`
2. A single worker goroutine picks queued runs sequentially (prevents repo lock contention)
3. Borg runs with `--log-json`; stderr is parsed for progress and broadcast via SSE
4. On completion, notifications fire for linked channels; audit log records user actions

## Data storage

SQLite database at `/app/data/backup-tool.db` (Docker) or `./data/backup-tool.db` (local).

SSH private keys and S3 credentials are encrypted at rest using Fernet-compatible AES derived from `SECRET_KEY`.

## Authentication

- OIDC browser sessions (Keycloak, Authentik, Pocket ID, etc.)
- Bearer JWT for automation
- `X-API-Key` header for long-lived API keys
- `AUTH_DISABLED=true` injects a dev user for local testing

## Docker requirements

Remote SSH/S3 backups require FUSE (`sshfs`, `rclone mount`):

- `privileged: true`, `cap_add: [SYS_ADMIN]`, `/dev/fuse` device
- Volume mounts: `./data:/app/data`, `/backup:/backup`

## Health endpoints

| Endpoint | Use |
|----------|-----|
| `/health/live` | Liveness |
| `/health/ready` | Readiness (DB, worker, borg) |
| `/health` | Full component report |
| `/metrics` | Prometheus exposition |
