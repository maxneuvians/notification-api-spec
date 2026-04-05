.PHONY: build test lint sqlc migrate-up migrate-down

build:
	go build ./cmd/api ./cmd/worker

test:
	go test ./...

lint:
	golangci-lint run

sqlc:
	sqlc generate

migrate-up:
	golang-migrate -path db/migrations -database "$(DATABASE_URI)?sslmode=disable" up

migrate-down:
	golang-migrate -path db/migrations -database "$(DATABASE_URI)?sslmode=disable" down 1