## ADDED Requirements

### Requirement: VSCode devcontainer opens a ready Go environment
The repository SHALL include a `.devcontainer/devcontainer.json` that, when opened with "Reopen in Container", provides a Go 1.23 development environment with PostgreSQL 15 and Redis 7 available. The Go extension and project-recommended extensions SHALL be installed automatically. No manual tool installation SHALL be required after the container starts.

#### Scenario: Container opens with Go toolchain available
- **WHEN** a developer reopens the repository in a devcontainer
- **THEN** `go version` reports Go 1.23.x inside the container

#### Scenario: Development tools pre-installed
- **WHEN** the container starts
- **THEN** `air`, `sqlc`, `golang-migrate`, and `golangci-lint` are all available on `$PATH`

#### Scenario: VS Code Go extension installed automatically
- **WHEN** the devcontainer starts
- **THEN** the `golang.go` extension is active without manual installation

---

### Requirement: PostgreSQL 15 reachable at hostname `db`
The devcontainer stack SHALL run a PostgreSQL 15 service accessible from the app container at hostname `db`, port `5432`. The default database SHALL be `notification_api` with user `postgres` and password `postgres`. Data SHALL persist across container restarts via a named Docker volume.

#### Scenario: Database reachable from app container
- **WHEN** the devcontainer stack is running
- **THEN** `psql $DATABASE_URI -c '\l'` exits 0 from within the app container

#### Scenario: Data persists across container restarts
- **WHEN** the app container is stopped and restarted without removing volumes
- **THEN** previously applied migrations and data are still present

---

### Requirement: Redis 7 reachable at hostname `redis`
The devcontainer stack SHALL run a Redis 7 service accessible from the app container at hostname `redis`, port `6379`. Redis data need not persist across container restarts.

#### Scenario: Redis reachable from app container
- **WHEN** the devcontainer stack is running
- **THEN** `redis-cli -h redis ping` returns `PONG` from within the app container

---

### Requirement: Local environment variables pre-configured
The devcontainer SHALL inject the following environment variables into the app container without requiring any manual `.env` file setup:

| Variable | Value |
|---|---|
| `DATABASE_URI` | `postgres://postgres:postgres@db:5432/notification_api` |
| `REDIS_URL` | `redis://redis:6379` |
| `NOTIFY_ENVIRONMENT` | `development` |

#### Scenario: DATABASE_URI set inside container
- **WHEN** the devcontainer starts
- **THEN** `echo $DATABASE_URI` prints the postgres connection string pointing to `db:5432`

#### Scenario: No .env file required
- **WHEN** the repository is cloned fresh and opened in the devcontainer
- **THEN** the application can connect to the database and Redis without creating any additional config file

---

### Requirement: Services start in dependency order
The `app` container SHALL declare `depends_on: [db, redis]` so that PostgreSQL and Redis are started before the Go container. All three services SHALL share a Docker network named `notify-dev`.

#### Scenario: app container starts after db and redis
- **WHEN** `docker compose up` is run for the devcontainer
- **THEN** the `db` and `redis` containers are running before the `app` container reaches its entrypoint
