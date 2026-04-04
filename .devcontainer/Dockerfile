FROM mcr.microsoft.com/devcontainers/go:1.23

# Install golang-migrate
RUN MIGRATE_VERSION=$(curl -s https://api.github.com/repos/golang-migrate/migrate/releases/latest | grep '"tag_name"' | cut -d'"' -f4) && \
    curl -L "https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" | tar xz && \
    mv migrate /usr/local/bin/golang-migrate

# Install sqlc
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Install golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin

# Install air (live-reload)
RUN go install github.com/air-verse/air@latest
