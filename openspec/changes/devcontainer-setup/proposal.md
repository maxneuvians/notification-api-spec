## Why

Contributors need a consistent, zero-setup local development environment. Without a devcontainer, each developer must manually install Go, run Postgres and Redis locally, and align config — leading to "works on my machine" problems and slow onboarding.

## What Changes

- `.devcontainer/devcontainer.json` — VSCode devcontainer configuration pointing to the Docker Compose setup; installs Go and recommended extensions
- `.devcontainer/docker-compose.yml` — defines three services: `app` (Go devcontainer), `db` (PostgreSQL 15), `redis` (Redis 7); all on a shared `notify-dev` network
- `.devcontainer/Dockerfile` — Go 1.23 devcontainer image with development tooling (golangci-lint, golang-migrate, sqlc, air for live-reload)
- Environment variable defaults for local development (DB connection, Redis URL, etc.) wired into the devcontainer

## Capabilities

### New Capabilities

- `devcontainer-setup`: VSCode devcontainer with Go 1.23, PostgreSQL 15, and Redis 7 for local development

### Modified Capabilities

## Non-goals

- Production Docker images or Kubernetes configs
- CI pipeline changes
- Seeding the database with fixture data (covered by migration tasks in `data-model-migrations`)
- Cloud-hosted dev environments (e.g., Codespaces configuration beyond devcontainer.json basics)

## Impact

- New `.devcontainer/` directory at repo root
- No changes to application code or existing configs
- Requires Docker Desktop (or compatible Docker runtime) on developer machines
