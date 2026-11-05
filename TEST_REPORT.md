# Phase 1 Test Report

Date: 2026-11-05. Go 1.24.7 (linux/amd64), PostgreSQL 16 (throwaway local server).

## IMPORTANT: what could and could not be verified in the build sandbox

The sandbox blocked `proxy.golang.org`, so `github.com/go-chi/chi/v5` and
`github.com/jackc/pgx/v5` could not be downloaded. Consequently:

| Check | Status | How |
|---|---|---|
| domain, usecase, worker, memory, config, logger, exercise: build, vet, test, **-race**, cover | VERIFIED (real toolchain, real code) | `go test -race -cover` |
| HTTP handler + router tests | COMPILED AND PASSED against a **stand-in** for chi (net/http ServeMux based), NOT real chi | scratch copy only |
| PostgreSQL repository + Go integration tests | TYPE-CHECKED against a signature stub of pgx; **never executed** | `go vet` on stub |
| PostgreSQL SQL (migration up/down/up, every statement the repo issues, CHECK constraint, injection payload, ID non-reuse, not-found semantics) | VERIFIED against real PostgreSQL 16 using `psql` prepared statements | manual script |
| gofmt | VERIFIED clean | `gofmt -l .` |

**You must run `bash scripts/check.sh` and the PostgreSQL integration tests on your machine
to complete acceptance.** Do not treat the handler or Go-PostgreSQL tests as passed until you have.

## Defects found by testing and fixed

1. `internal/worker`: `Stop()` closed the queue while `Publish()` could still send ->
   `panic: send on closed channel` and a data race (reproduced by the new tests, then fixed
   with an RWMutex + `closed` flag; `Publish` after `Stop` now returns `false`).
2. `internal/delivery/http/handler`: `decodeJSON` accepted bodies with trailing data
   (`{"title":"x"} junk` and `{"title":"x"}{"title":"y"}` returned 201 with an empty task).
   Now only a clean EOF after the first value is accepted (reproduced, then fixed).

No other production code was changed.

## Results (real toolchain)

```
domain 100%  usecase 100%  worker 100%  memory repo 100%  config 100%  logger 100%
worker + memory: go test -race -count=30  -> pass
```

Stand-in run (see caveat): handler 100%, router (`internal/delivery/http`) 100%, `cmd/api` 12.8%
(only `buildRepository` error paths; `main()` is not unit-testable without refactoring).

## Observations for later phases (not changed)

- `.env` in the uploaded ZIP held a database password; it is excluded from the delivered ZIP
  and `.env.example` added. If that password was ever committed or reused, rotate it.
- `testdata/tasks.json` is resolved relative to the working directory (matters for Docker).
- `http.Server` only sets `ReadHeaderTimeout`; add Read/Write/Idle timeouts in Phase 2.6.
- Oversized bodies return 400 rather than 413.
- `exercise/` is learning scratch code inside the module (28% coverage); consider removing it before publishing to GitHub.
- No update lost-write protection (README item 2); events are best-effort by design.
