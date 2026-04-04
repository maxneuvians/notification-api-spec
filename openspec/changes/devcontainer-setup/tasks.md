## 1. Dockerfile

- [ ] 1.1 Create `.devcontainer/Dockerfile` — base `mcr.microsoft.com/devcontainers/go:1.23`; add `RUN` layer installing `golang-migrate` (latest GitHub release), `sqlc` (`go install`), `golangci-lint` (official install script), and `air` (`go install`)
- [ ] 1.2 Verify all four tools are on `$PATH` inside the built image by running `air -v && sqlc version && golang-migrate -version && golangci-lint --version`

## 2. Docker Compose

- [ ] 2.1 Create `.devcontainer/docker-compose.yml` — define `app`, `db` (`postgres:15-alpine`), and `redis` (`redis:7-alpine`) services on network `notify-dev`; `app` declares `depends_on: [db, redis]`; `db` mounts named volume `pgdata`
- [ ] 2.2 Set `db` environment: `POSTGRES_DB=notification_api`, `POSTGRES_USER=postgres`, `POSTGRES_PASSWORD=postgres`

## 3. devcontainer.json

- [ ] 3.1 Create `.devcontainer/devcontainer.json` — reference `docker-compose.yml`, set `service: app` and `workspaceFolder: /workspaces/${localWorkspaceFolderBasename}`
- [ ] 3.2 Add `remoteEnv` block: `DATABASE_URL`, `REDIS_URL`, `NOTIFY_ENVIRONMENT=development`
- [ ] 3.3 Add `customizations.vscode.extensions` list: `golang.go`, `golang.go-nightly` (optional), `ms-azuretools.vscode-docker`, `mtxr.sqltools`, `mtxr.sqltools-driver-pg`
- [ ] 3.4 Add `postCreateCommand` to run `go mod download` so module cache is warm after container build

## 4. Validation

- [ ] 4.1 Open repo in devcontainer; confirm `go version`, `air -v`, `sqlc version`, `golang-migrate -version`, `golangci-lint --version` all succeed
- [ ] 4.2 Confirm `psql $DATABASE_URL -c '\l'` exits 0 and `redis-cli -h redis ping` returns `PONG`
- [ ] 4.3 Confirm `go build ./...` succeeds from `/workspaces/...` inside the container
