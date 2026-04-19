.PHONY: run build test tidy sqlc migrate-up migrate-down migrate-status lint

# Load .env into Make's environment if the file exists, then export every
# variable so child processes (go run, goose, etc.) inherit them.
# This is Make's built-in equivalent of "source .env" in bash.
ifneq (,$(wildcard .env))
  include .env
  export
endif

# Build the server binary
build:
	go build -o ../../bin/api ./cmd/server

# Run the server — variables from .env are automatically available because
# of the include/export block above.
run:
	go run ./cmd/server

# Run all tests
test:
	go test ./... -race -count=1

# Tidy modules
tidy:
	go mod tidy

# Regenerate sqlc types + queries
sqlc:
	sqlc generate

# Run migrations — DATABASE_URL is available from .env via the export block above
migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DATABASE_URL)" status

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...
