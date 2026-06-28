# API Reference

Base path: `/api/v1` (authenticated unless noted).

## Auth

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/login` | Redirect to OIDC provider |
| GET | `/auth/callback` | OIDC callback |
| POST | `/auth/logout` | Clear session |
| GET | `/auth/me` | Current user (optional auth) |

Authentication: session cookie, `Authorization: Bearer <JWT>`, or `X-API-Key: {prefix}-{secret}`.

## Dashboard

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/dashboard/stats` | Stats, recent runs, failed jobs |
| GET | `/api/v1/dashboard/progress` | SSE progress stream |
| POST | `/api/v1/runs/acknowledge-failed` | Acknowledge failed runs |

## Resources

CRUD endpoints for `/sources`, `/destinations`, `/jobs`, `/notification-channels`, `/api-keys`.

### Jobs actions

- `POST /jobs/{id}/toggle` — enable/disable
- `POST /jobs/{id}/run` — queue backup
- `POST /jobs/{id}/cancel` — cancel queued/running
- `GET /jobs/{id}/runs` — run history
- `GET /runs/{id}/stream` — SSE log stream

### Destinations ops

- `POST /destinations/{id}/break-lock`
- `POST /destinations/{id}/compact`
- `POST /destinations/{id}/verify`
- `GET /destinations/storage`

### Archives

- `GET /archives?destination_id=&job_id=`
- `GET /archives/{name}/files`
- `POST /archives/{name}/restore`
- `POST /archives/delete-batch`
- `GET /archives/{name}/download-tar`

## Settings & config

| Method | Path | Description |
|--------|------|-------------|
| GET/PUT | `/api/v1/settings` | Application settings |
| GET | `/api/v1/config/export` | Export JSON config |
| POST | `/api/v1/config/import` | Import JSON config |

## Health (unauthenticated)

| Path | Purpose |
|------|---------|
| `/health/live` | Liveness |
| `/health/ready` | Readiness |
| `/health` | Full report |
| `/metrics` | Prometheus (optional `METRICS_TOKEN`) |
