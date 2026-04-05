## Context

The GC Notify API is a Go/PostgreSQL/Redis application. Currently there is no standardised local development environment — developers must install tooling manually and run external services themselves. A VSCode devcontainer wrapping a Docker Compose stack eliminates this friction and guarantees every contributor uses identical tool versions.

## Goals / Non-Goals

**Goals:**
- Single `Reopen in Container` action gives a fully working dev environment
- Go 1.23 toolchain with golangci-lint, golang-migrate, sqlc, and air (live-reload) pre-installed
- PostgreSQL 15 and Redis 7 accessible from the app container at predictable hostnames (`db`, `redis`)
- Environment variables for local development pre-configured in the container
- VS Code Go extension and recommended extensions auto-installed

**Non-Goals:**
- Production Docker images
- CI pipeline changes
- Database seeding with fixture data
- Codespaces or remote-container hosting beyond standard devcontainer.json

## Decisions

### Three-service Docker Compose topology
The stack uses three named services on a shared bridge network (`notify-dev`):
- `app` — Go devcontainer built from `.devcontainer/Dockerfile`
- `db` — `postgres:15-alpine` with a persistent named volume (`pgdata`)
- `redis` — `redis:7-alpine`, no persistence needed for development

`app` declares `depends_on: [db, redis]` so the database and cache are up before the Go container starts.

### Dockerfile base: mcr.microsoft.com/devcontainers/go:1.23
Using the official Microsoft Go devcontainer base keeps VSCode remote-container integration reliable. Additional tooling is installed as a `RUN` layer:
- `golang-migrate` (latest release from GitHub releases)
- `sqlc` (via `go install`)
- `golangci-lint` (via official install script)
- `air` (via `go install`)

### Environment variables via devcontainer.json `remoteEnv`
Local connection strings are injected through `remoteEnv` in `devcontainer.json` rather than a checked-in `.env` file, avoiding accidental credential leakage. Values reference Docker Compose service hostnames:
```
DATABASE_URI=postgres://postgres:postgres@db:5432/notification_api
REDIS_URL=redis://redis:6379
```
All other config env vars default to development-safe values (test keys, local hostnames, `NOTIFY_ENVIRONMENT=development`).

### Persistent Postgres volume, ephemeral Redis
Postgres data is stored in a named Docker volume (`pgdata`) so migrations survive container restarts. Redis data is not persisted — the cache is rebuilt on each session start, consistent with how Redis is used in the application.

### docker-compose.override.yml pattern not used
A single `.devcontainer/docker-compose.yml` is used rather than layering override files, keeping the setup simple for contributors unfamiliar with Docker Compose override semantics.

## Risks / Trade-offs

- Requires Docker Desktop (or Rancher Desktop / Podman Desktop) — contributors on machines without Docker cannot use the devcontainer but can still develop manually
- First `Reopen in Container` will take several minutes to build the image; subsequent starts are fast due to layer caching
- `air` live-reload watches the full repo directory; large file trees may cause excessive CPU usage on macOS due to fsevents overhead
