#!/usr/bin/env bash
# End-to-end CRUD smoke test against a RUNNING API (Docker or local).
#   bash scripts/smoke.sh [base_url]     default: http://localhost:8080/api/v1
# Needs only bash + curl (both ship with Git Bash). Exits non-zero on first failure.
set -euo pipefail
BASE="${1:-http://localhost:8080/api/v1}"

fail() { echo "FAIL: $*" >&2; exit 1; }
status() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
expect() { [ "$1" = "$2" ] || fail "$3: got HTTP $1, want $2"; echo "ok   $3 ($1)"; }

expect "$(status "$BASE/health")" 200 "health"

body=$(curl -s -X POST "$BASE/tasks" -H 'Content-Type: application/json' \
  -d '{"title":"smoke test","description":"created by smoke.sh"}')
echo "$body" | grep -q '"title":"smoke test"' || fail "create body: $body"
id=$(echo "$body" | sed -E 's/.*"id":([0-9]+).*/\1/')
[ -n "$id" ] || fail "could not parse id from: $body"
echo "ok   create (id=$id)"

expect "$(status "$BASE/tasks/$id")" 200 "get"
expect "$(status "$BASE/tasks")" 200 "list"
expect "$(status -X PUT "$BASE/tasks/$id" -H 'Content-Type: application/json' \
  -d '{"title":"smoke test v2","description":"updated","completed":true}')" 200 "update"
curl -s "$BASE/tasks/$id" | grep -q '"completed":true' || fail "update was not persisted"
echo "ok   update persisted"
expect "$(status -X POST "$BASE/tasks" -H 'Content-Type: application/json' -d '{"title":"   "}')" 400 "blank title rejected"
expect "$(status -X DELETE "$BASE/tasks/$id")" 204 "delete"
expect "$(status "$BASE/tasks/$id")" 404 "get after delete"
echo "SMOKE TEST PASSED"
