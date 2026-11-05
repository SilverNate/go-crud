# Contributing

## Setup
1. Install Go 1.23+, Git, and (for integration tests) PostgreSQL 13+.
2. `cp .env.example .env` and edit values. `.env` is git-ignored; never commit it.
3. Run the app: see [README](README.md). Run the quality gate: `bash scripts/check.sh`.

## Branching
| Branch | Purpose | Rules |
|---|---|---|
| `main` | Always releasable; tagged releases come from here | Protected. PR only, CI green, no direct pushes |
| `develop` | Integration branch for the next release | PR only |
| `feature/<short-name>` | New work, branched from `develop` | Deleted after merge |
| `fix/<short-name>` | Bug fixes, branched from `develop` | Deleted after merge |
| `hotfix/<short-name>` | Urgent production fix, branched from `main`, merged to `main` AND `develop` | Rare |

## Commit messages (Conventional Commits)
```
<type>(<optional scope>): <imperative summary, <= 72 chars>

<optional body: why, not what>
```
Types: `feat`, `fix`, `test`, `docs`, `refactor`, `perf`, `build`, `ci`, `chore`.
Add `!` or a `BREAKING CHANGE:` footer for breaking changes. Examples:
`fix(worker): reject Publish after Stop instead of panicking`, `ci: add race detector job`.

## Pull requests
- Small and focused (aim for < 400 changed lines); one logical change.
- Fill in the PR template. CI must pass before review.
- At least one approval. The author does not merge their own unreviewed PR.
- Reviewers check: correctness, tests, error handling, concurrency/resource lifecycle, naming, docs.

## Merge strategy
- `feature/*` / `fix/*` -> `develop`: **squash merge** (one clean commit per change; the PR title becomes a Conventional Commit).
- `develop` -> `main` (release): **merge commit** (`--no-ff`) so releases are visible in history.
- Never force-push `main` or `develop`.

## Releases and tags
Semantic Versioning: `vMAJOR.MINOR.PATCH`. Tag on `main` with an annotated tag:
```bash
git tag -a v0.1.0 -m "Phase 1: tested core API"
git push origin v0.1.0
```
Tags will trigger image publishing in the CD pipeline (Phase 2.3).

## Security
Never commit credentials. If one leaks, **rotate it first**, then clean history. Report vulnerabilities privately to the maintainer rather than in a public issue.
