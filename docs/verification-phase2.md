# Phase 2 verification log

## 2.2 Docker / Compose  (status: NOT yet verified end-to-end)

| Item | Status | Evidence |
|---|---|---|
| `healthcheck` subcommand (Go) | Verified against stand-in router, 5 tests pass | sandbox, `go test -race ./cmd/...` |
| `docker-compose.yml` syntax and interpolation | Verified | `docker compose config` (and it refuses to start without `POSTGRES_PASSWORD`) |
| Dockerfile builds | **NOT verified** | sandbox blocks Docker Hub, gcr.io and the Go module proxy |
| Image runs as non-root, healthy, size | **NOT verified** | needs a build |
| API reaches PostgreSQL via `db` | **NOT verified** | needs `docker compose up` |
| Graceful shutdown on `docker stop` | **NOT verified** | needs a running container |

### Run these locally (Git Bash) and paste the output back
```bash
docker compose build
docker compose up -d
docker compose ps                                   # expect api + db "healthy"
bash scripts/smoke.sh                               # expect: SMOKE TEST PASSED
docker compose logs api | head -20                  # expect JSON logs, no connection errors
docker images go-task-api:local                     # note SIZE
docker inspect --format '{{.Config.User}}' go-task-api:local   # expect nonroot:nonroot
time docker compose stop api                        # expect well under 20s
docker compose logs api | tail -5                   # expect "shutdown signal received"
docker compose down                                 # volume kept
docker compose up -d && curl -s localhost:8080/api/v1/tasks   # data persisted across down/up
```
