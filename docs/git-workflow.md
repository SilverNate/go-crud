# Git & GitHub workflow: how it works and why

## Mental model
- A commit is an immutable snapshot with a parent pointer; a branch is just a movable pointer to a commit; a tag is a fixed pointer. That is why branching is cheap and why history rewriting (rebase, amend) creates *new* commits.
- `origin/main` is your local copy of the remote's pointer. `git fetch` updates it; `git pull` = fetch + merge (or rebase).

## Strategy used here: simplified Git Flow
`main` (releases) <- `develop` (integration) <- `feature/*`, `fix/*`.
- Alternatives: **trunk-based development** (short-lived branches straight to `main`, feature flags, needs strong CI; best for continuous deployment) and **GitHub Flow** (main + feature branches). For a solo/small-team service with versioned releases, develop+main adds a staging point at small cost. If you deploy on every merge, prefer GitHub Flow.
- Trade-off: long-lived `develop` can drift and cause painful merges. Keep feature branches short (days) and rebase/merge `develop` into them often.

## Merge strategies
| Strategy | History | Use when | Downside |
|---|---|---|---|
| Merge commit | Preserves branch topology | Releases | Noisy graph |
| Squash | One commit per PR | Feature PRs | Loses intermediate commits, `git blame` is per-PR |
| Rebase + FF | Linear | Small clean branches | Rewrites commits; never on shared branches |

## Daily flow (Git Bash)
```bash
git switch develop && git pull
git switch -c feature/prometheus-metrics
# ... edit, test ...
bash scripts/check.sh
git add -p                       # stage deliberately; review what you stage
git commit -m "feat(metrics): expose HTTP request counters"
git push -u origin feature/prometheus-metrics
# open a PR into develop on GitHub; squash-merge after review + green CI
```

## Branch protection (configure on GitHub: Settings > Branches)
For `main` and `develop`: require a pull request, require status checks (CI, added in 2.3), require branches up to date, block force pushes and deletions.

## Interview questions
1. *Merge vs rebase? When is rebase dangerous?* Rebase rewrites commits (new SHAs); rebasing a branch others have pulled forces them to reconcile diverged history. Rebase local/private branches only.
2. *You committed a password. What now?* Rotate the credential immediately (history rewriting does not un-leak it; forks/clones/caches exist). Then remove it with `git filter-repo` or BFG, force-push, and ask collaborators to re-clone. Add secret scanning/push protection.
3. *`git reset` vs `revert`?* `reset` moves a branch pointer (rewrites history; local use). `revert` adds a new commit that undoes a change (safe on shared branches).
4. *How do you find the commit that introduced a bug?* `git bisect` with a failing test.
5. *Why Conventional Commits?* Machine-readable history: automated changelogs and semantic version bumps, easier review.
6. *Squash vs merge commits for releases?* Squash feature work for a clean log; merge commit for release branches to retain the release boundary.
7. *What does `.gitattributes eol=lf` fix?* Windows `core.autocrlf` converting shell scripts/Dockerfiles to CRLF, which breaks them in Linux containers (`bad interpreter: ^M`).
