# Go Task API — Phase 1

A learning project implementing CRUD with Go, go-chi, Clean Architecture, SOLID principles, concurrency, channels, structured logging, and interchangeable memory/PostgreSQL repositories.

## Architecture

```text
HTTP / go-chi
    -> HTTP Handler (transport concerns, DTO validation, status codes)
        -> Task Use Case (application orchestration)
            -> TaskRepository interface
                -> memory.Repository (RWMutex)
                -> postgres.Repository (pgx pool)
```

- **Domain:** task entity, validation and domain errors. No HTTP or database imports.
- **Use case:** application operations and repository/event-publisher contracts.
- **Repository adapters:** memory and PostgreSQL implement the same interface.
- **Delivery:** go-chi routes, handlers and HTTP middleware.
- **Worker:** bounded channel consumes task events asynchronously.
- **Composition root:** `cmd/api/main.go` wires concrete dependencies.

## Requirements

- Go 1.23+
- PostgreSQL 13+ only when using PostgreSQL mode

## Run with in-memory storage

```bash
# Optional: export configuration values in your shell.
export STORAGE_DRIVER=memory
go mod tidy
go run ./cmd/api
```

`.env.example` documents the supported environment variables; this starter does not automatically parse `.env` files.

`testdata/tasks.json` is loaded at startup as fake seed data. New API writes are held in memory only and are lost when the process exits. The fixture is for local learning, not a production data source.

## API

Base URL: `http://localhost:8080/api/v1`

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Health check |
| POST | `/tasks` | Create task |
| GET | `/tasks` | List tasks |
| GET | `/tasks/{id}` | Get task |
| PUT | `/tasks/{id}` | Update task |
| DELETE | `/tasks/{id}` | Delete task |

Create example:

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learn Go","description":"Build clean CRUD"}'
```

Update example:

```bash
curl -X PUT http://localhost:8080/api/v1/tasks/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Learn Go deeply","description":"Add PostgreSQL","completed":true}'
```

List:

```bash
curl http://localhost:8080/api/v1/tasks
```

Delete:

```bash
curl -i -X DELETE http://localhost:8080/api/v1/tasks/1
```

## PostgreSQL mode

Create a database named `taskdb`, apply the migration, then configure:

```bash
export STORAGE_DRIVER=postgres
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/taskdb?sslmode=disable'
```

Apply schema:

```bash
# 1. Fix code page warning
chcp 65001

# 2. Export variables safely ignoring comments
set -a
source .env
set +a

psql "$DATABASE_URL" -f migrations/001_create_tasks.up.sql
```

Start:

```bash
go run ./cmd/api
```

The migration is intentionally plain SQL so its lifecycle is explicit. A migration runner can be added in a later refinement.

## Test and quality checks

```bash
gofmt -w .
go test ./...
go test -race ./...
go test -cover ./...
go vet ./...
```

PostgreSQL integration tests should run against a disposable database; unit tests do not require PostgreSQL.

## Concurrency and channels

The memory adapter protects its map and ID counter with `sync.RWMutex`. The worker uses a bounded channel and a goroutine to consume task events. Publishing is non-blocking: if the queue is full, the event is dropped and a warning is logged. This is deliberately best-effort, not durable messaging. For guaranteed delivery, introduce a transactional outbox or external broker.

## Logging

`slog` JSON logs include HTTP method, path, status, duration and request ID. Application failures and asynchronous task events are also logged. Domain entities remain independent of logging infrastructure; the application layer emits operation logs.

## Important learning notes / next improvements

1. Add pagination and filtering to `List`.
2. Add optimistic concurrency (version column / `If-Match`) to prevent lost updates between clients.
3. Add PostgreSQL integration tests and automated migration execution.
4. Add request body size limits, strict trailing JSON validation, and a consistent error envelope.
5. Add graceful worker draining with producer shutdown coordination.
6. Add observability metrics and tracing in Phase 2.

## Phase 2 (not included)

GitHub workflow, Docker image/Compose, GitHub Actions CI, Grafana/Prometheus, and Kubernetes manifests are intentionally deferred until Phase 1 acceptance.
