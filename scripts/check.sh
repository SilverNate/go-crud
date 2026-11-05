#!/usr/bin/env bash
# Phase 1 quality gate. Run from the repository root (Git Bash on Windows works).
set -euo pipefail

echo "==> gofmt (verify)"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "These files need gofmt -w:"; echo "$unformatted"; exit 1
fi

echo "==> go vet";   go vet ./...
echo "==> go test";  go test ./...

echo "==> go test -race"
if [ "$(go env CGO_ENABLED)" = "1" ] && command -v gcc >/dev/null 2>&1; then
  go test -race ./...
else
  echo "SKIPPED: the race detector needs CGO_ENABLED=1 and gcc (see README: Windows setup)."
fi

echo "==> go test -cover"; go test -cover ./...
echo "All checks completed."
