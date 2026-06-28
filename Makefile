.PHONY: dev dev-backend dev-frontend build test docker-build migrate

SECRET_KEY ?= dev-secret-key-change-in-production

dev-backend:
	cd backend && SECRET_KEY=$(SECRET_KEY) AUTH_DISABLED=true go run ./cmd/server/

dev-frontend:
	cd frontend && npm run dev

dev:
	@echo "Run 'make dev-backend' and 'make dev-frontend' in separate terminals"

build-frontend:
	cd frontend && npm ci && npm run build
	rm -rf backend/cmd/server/static/*
	cp -r frontend/dist/* backend/cmd/server/static/

build: build-frontend
	cd backend && go build -o ../bin/backup-manager ./cmd/server/

test:
	cd backend && go test ./...
	cd frontend && npm run build

docker-build: build-frontend
	docker build -t borg-backup-manager:latest .

migrate:
	cd backend && SECRET_KEY=$(SECRET_KEY) go run ./cmd/server/ --migrate-only 2>/dev/null || true
