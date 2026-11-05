# Docker & Compose

## Quick start (Git Bash, Docker Desktop running)
```bash
cp .env.example .env            # then edit POSTGRES_PASSWORD (long, random; avoid @ : / ? #)
docker compose build
docker compose up -d
docker compose ps               # api and db should become "healthy"
docker compose logs -f api      # Ctrl+C stops following, not the service
bash scripts/smoke.sh           # end-to-end CRUD through the container
docker compose down             # stop + remove containers/network; data volume is KEPT
docker compose down -v          # ALSO deletes the database volume (destroys data)
```
Port 8080 busy? Set `API_PORT=8081` in `.env`. Inspect the database: `docker compose exec db psql -U taskuser -d taskdb -c "SELECT * FROM tasks"` (in Git Bash prefix with `MSYS_NO_PATHCONV=1` if you pass paths).

Prove the API reaches PostgreSQL via the service name: the stack only works if `DATABASE_URL` host is `db`; `docker compose exec api` is not available (distroless has no shell), so use the smoke test plus `docker compose logs api` (no connection errors) and `docker compose exec db psql ... SELECT count(*) FROM tasks` after creating a task.

## Concepts
- **Image vs container.** An image is an immutable, layered filesystem + metadata (a build artifact). A container is a running (or stopped) instance of an image with a thin writable layer, its own process/network/mount namespaces and cgroup limits. Many containers can share one image.
- **Multi-stage build.** Stage 1 (`golang:alpine`) has the compiler and sources; stage 2 copies only the static binary into `distroless`. The final image has no compiler, shell or package manager: typically tens of MB instead of ~1 GB (measure with `docker images`) and far less to exploit. Layer order matters: `go.mod`/`go.sum` are copied before sources so dependency downloads stay cached.
- **Why distroless/static (alternatives).** `scratch` is smallest but has no CA certs, tzdata or non-root user entry. `alpine` adds a shell (handy for debugging, more attack surface, musl). `distroless:debug` adds a busybox shell when you must `exec` in. We use `CGO_ENABLED=0` so the binary needs no libc.
- **Non-root.** The process runs as UID 65532. Compromise of the app then does not hand over root in the container. Compose also drops all Linux capabilities, sets `no-new-privileges` and a read-only root filesystem (the app writes nothing to disk).
- **Networking.** Compose creates a private bridge network `backend`; containers resolve each other by **service name** via Docker's embedded DNS. `localhost` inside a container is that container itself, so the API must use host `db`, never `localhost`. Only the API publishes a port, bound to `127.0.0.1`; PostgreSQL has no published port at all.
- **Volumes.** Container filesystems are ephemeral. The named volume `pgdata` holds the database and survives `down` and image upgrades. The SQL file in `/docker-entrypoint-initdb.d` runs **only when the data directory is empty** (first start). Later schema changes need a migration tool (e.g. golang-migrate/goose), which we should add before the schema evolves.
- **Environment variables.** Config comes from env (12-factor). Secrets live in the git-ignored `.env`, never in the image or Dockerfile. Caveat: env values show up in `docker inspect`; for production use orchestrator secrets (Kubernetes Secrets in 2.5, Docker/Swarm secrets, or a vault).
- **Health checks.** The API's `HEALTHCHECK` runs `/app/api healthcheck` (the binary probes `/api/v1/health`) because distroless has no curl. Postgres uses `pg_isready`. `depends_on: condition: service_healthy` makes the API wait for the DB. Note `/health` is *liveness*: it does not check the database. A readiness endpoint that pings the DB is planned for Kubernetes (2.5).
- **Lifecycle and shutdown.** `docker stop` sends SIGTERM, waits `stop_grace_period`, then SIGKILL. The Go process is PID 1 (exec-form `ENTRYPOINT`, no shell wrapper that would swallow signals); `signal.NotifyContext` triggers `server.Shutdown` (10s budget), then the worker is drained and the pool closed. `stop_grace_period: 20s` leaves headroom over the app's 10s; Docker's default 10s would race it.
- **Dev vs prod.** Dev: `.env`, ports on loopback, `docker compose up --build`, seed data allowed. Prod: pinned image tags/digests pushed to a registry (2.3), secrets from a secret store, managed or backed-up PostgreSQL, resource limits, TLS terminated at a proxy/ingress, no bind mounts, non-`latest` tags.

## What could fail in production
- API starts before DB is ready -> mitigated by healthcheck + `depends_on`; also keep `restart` policy and make the app tolerate DB restarts (the pgx pool reconnects).
- `docker compose down -v` or a deleted volume wipes data: back up with `pg_dump`.
- Password with `@` or `:` breaks the connection URL -> percent-encode it.
- Init script silently not re-run on an existing volume -> "relation tasks does not exist" never happens on fresh volumes but schema edits won't apply.
- PID 1 zombie reaping / signals: fine here (single static Go process); would matter for shell wrappers.
- Image drift: unpinned `postgres:16-alpine` floats within 16.x; pin by digest in prod.

## Interview questions
1. Why exec-form ENTRYPOINT? What breaks with `CMD ./api` in shell form? (shell becomes PID 1, doesn't forward SIGTERM -> 10s hang then SIGKILL, no graceful shutdown.)
2. Why copy `go.mod` before the source? (layer caching.)
3. `scratch` vs `distroless` vs `alpine`: trade-offs?
4. Container can't reach DB at `localhost:5432`, why? (network namespace; use service name.)
5. Difference between a volume, a bind mount and tmpfs?
6. `depends_on` vs healthcheck: what does `depends_on` alone guarantee? (start order only, not readiness.)
7. How would you shrink the image further / speed builds? (cache mounts, `-ldflags -s -w`, `.dockerignore`.)
8. How do you run a debug shell in a distroless container? (`kubectl debug` / `docker run --pid=container:... busybox`, or a `:debug` image variant.)
9. Is putting `DATABASE_URL` in an env var secure? What are the alternatives?
