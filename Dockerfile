# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-bookworm AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./cmd/server/static/
RUN CGO_ENABLED=0 go build -o /backup-manager ./cmd/server/

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    borgbackup fuse3 sshfs rclone openssh-client wget ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /backup-manager /app/backup-manager
RUN mkdir -p /app/data /app/sessions /backup
ENV PORT=5001
EXPOSE 5001
ENTRYPOINT ["/app/backup-manager"]
