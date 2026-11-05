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
export DATABASE_URL='postgres://USER:PASSWORD@localhost:5432/taskdb?sslmode=disable'
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

## Run with Docker Compose

```bash
cp .env.example .env     # set POSTGRES_PASSWORD
docker compose up -d --build
bash scripts/smoke.sh    # CRUD smoke test against http://localhost:8080
docker compose down
```

Details, concepts and troubleshooting: [docs/docker.md](docs/docker.md).

## CI/CD

GitHub Actions: `.github/workflows/ci.yml` (format, vet, race tests, PostgreSQL integration tests, Docker smoke test) and `.github/workflows/cd.yml` (publish image to GHCR on release tags, environment-gated deploy). See [docs/cd.md](docs/cd.md).

## Test and quality checks

All commands work in Git Bash on Windows:

```bash
gofmt -w .          # format
go vet ./...        # static checks
go test ./...       # all tests (PostgreSQL integration tests skip if not configured)
go test -race ./... # race detector (needs CGO + gcc, see below)
go test -cover ./...
# or everything at once:
bash scripts/check.sh
```

### Test layout

| Layer | Location | Style |
|---|---|---|
| Domain | `internal/domain/task_test.go` | table-driven, no mocks |
| Use case | `internal/usecase/task_test.go` | hand-written fake repository + fake event publisher |
| HTTP handlers | `internal/delivery/http/handler/task_test.go` | `httptest` + stub service, real chi routing |
| Router / full stack | `internal/delivery/http/router_test.go` | real router + use case + memory repo; unknown routes, 405s, request logging |
| Memory repository | `internal/repository/memory/*_test.go` | CRUD, ID uniqueness, concurrency (`-race`) |
| PostgreSQL repository | `internal/repository/postgres/task_test.go` | integration tests against a real database |
| Worker | `internal/worker/task_worker_test.go` | queue-full, drain on stop, publish-after-stop, concurrent publishers |
| Config / logger / wiring | `internal/config`, `internal/logger`, `cmd/api` | small unit tests |

### PostgreSQL integration tests

They only run when `TEST_DATABASE_URL` is set, and **refuse to run unless the database name contains `test`**. Each run creates its own schema (`it_<pid>_<time>`), applies `migrations/001_create_tasks.up.sql` inside it, and drops the schema afterwards, so no existing data is touched.

```bash
# one-time: create a disposable database
psql "postgres://postgres:YOUR_PASSWORD@localhost:5432/postgres" -c "CREATE DATABASE taskdb_test"

export TEST_DATABASE_URL='postgres://postgres:YOUR_PASSWORD@localhost:5432/taskdb_test?sslmode=disable'
go test -v ./internal/repository/postgres/
```

### Race detector on Windows

`go test -race` needs `CGO_ENABLED=1` and a C compiler on `PATH`.

1. Install a MinGW-w64 GCC (for example via MSYS2: `pacman -S mingw-w64-ucrt-x86_64-gcc`, or the WinLibs build).
2. Add its `bin` directory to `PATH` and confirm in Git Bash: `gcc --version`.
3. Run `CGO_ENABLED=1 go test -race ./...`.

If GCC is unavailable, `scripts/check.sh` prints a SKIPPED notice instead of failing; CI on Linux (Phase 2) runs the race detector unconditionally.

## Concurrency and channels

The memory adapter protects its map and ID counter with `sync.RWMutex`. The worker uses a bounded channel and a goroutine to consume task events. Publishing is non-blocking: if the queue is full, the event is dropped and a warning is logged. This is deliberately best-effort, not durable messaging. For guaranteed delivery, introduce a transactional outbox or external broker.

## Logging

`slog` JSON logs include HTTP method, path, status, duration and request ID. Application failures and asynchronous task events are also logged. Domain entities remain independent of logging infrastructure; the application layer emits operation logs.

## Important learning notes / next improvements

1. Add pagination and filtering to `List`.
2. Add optimistic concurrency (version column / `If-Match`) to prevent lost updates between clients.
3. Automated migration execution (integration tests currently apply the SQL file themselves).
4. Return `413` (not `400`) for request bodies over the 1 MiB limit.
5. Make the memory fixture path (`testdata/tasks.json`) configurable; it is currently relative to the working directory.
6. Add observability metrics and tracing in Phase 2.

Done in Phase 1: worker shutdown no longer panics or races with publishers; strict single-JSON-value request parsing; integration tests for PostgreSQL.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (branching, commit conventions, PRs, releases) and [docs/git-workflow.md](docs/git-workflow.md). Copy `.env.example` to `.env` for local settings; `.env` is never committed.

## Roadmap

Prometheus/Grafana observability and Kubernetes manifests come next (roadmap phases 2.4 and 2.5).
