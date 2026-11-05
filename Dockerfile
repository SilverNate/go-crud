# ---------- Stage 1: build ----------
FROM golang:1.24-alpine AS build
WORKDIR /src

# Copy dependency manifests first: this layer is cached until go.mod/go.sum change,
# so code-only edits don't re-download modules.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd ./cmd
COPY internal ./internal

# Static binary (no libc) so it runs on a minimal image. -trimpath removes local
# paths; -s -w strips symbol/debug tables (smaller image).
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---------- Stage 2: runtime ----------
# distroless/static: CA certs, tzdata, /etc/passwd with a "nonroot" user; no shell,
# no package manager -> tiny attack surface. The :nonroot tag runs as UID 65532.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/api /app/api
# Only needed when STORAGE_DRIVER=memory (fixture path is relative to WORKDIR).
COPY testdata ./testdata

USER nonroot:nonroot
EXPOSE 8080

# Defaults are non-secret. DATABASE_URL (a secret) is injected at runtime, never baked in.
ENV HTTP_ADDR=:8080 \
    STORAGE_DRIVER=postgres \
    LOG_LEVEL=info

# No curl in distroless, so the binary probes itself.
HEALTHCHECK --interval=15s --timeout=4s --start-period=10s --retries=3 \
    CMD ["/app/api", "healthcheck"]

# Exec form: the Go process is PID 1 and receives SIGTERM directly -> graceful shutdown.
ENTRYPOINT ["/app/api"]
