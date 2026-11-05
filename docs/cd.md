# CI/CD

## Pipelines at a glance
| | CI (`.github/workflows/ci.yml`) | CD (`.github/workflows/cd.yml`) |
|---|---|---|
| Question answered | Is this change safe to merge? | Ship this exact, already-tested artifact |
| Triggers | every PR; push to `develop` and `main` | tag `vX.Y.Z`; manual "Run workflow" |
| Output | pass/fail, coverage artifact | image in GHCR, deployment to an environment |
| Permissions | `contents: read` | `packages: write` for the build job only |

**CI steps:** checkout -> setup Go (cached by `go.sum`) -> `gofmt` check -> `go vet` -> build -> unit tests with `-race` -> integration tests against a PostgreSQL service container with coverage -> upload coverage -> (second job) `docker compose up --build --wait`, run `scripts/smoke.sh` against the containers, tear down. That last job is what proves "CRUD works through the containerized application" on every PR.

**CD flow:** tag `v0.1.0` on `main` -> verify the commit is reachable from `main` -> build and push `ghcr.io/<owner>/<repo>:v0.1.0` and `:sha-<full sha>` (no `latest`) -> deploy to **staging** -> post-deploy health check. Promotion to **production** is a *manual* run with `environment=production` and `image_tag=v0.1.0`: the same image is promoted, never rebuilt, so what you tested is what ships.

## One-time GitHub setup
1. **Branch protection** (Settings > Branches) for `main` and `develop`: require a PR, require status checks `Format, vet, test` and `Docker build and Compose smoke test`, block force-push.
2. **Environments** (Settings > Environments): create `staging` and `production`. On `production` add *Required reviewers* (manual approval gate) and restrict deployment branches/tags. Put environment-specific secrets there.
3. **Variables** (per environment): `DEPLOY_ENABLED=true` (the deploy job is skipped until you set it), `HEALTHCHECK_URL=https://<host>/api/v1/health`.
4. Actions > General > Workflow permissions: leave default read-only; the CD job requests `packages: write` explicitly.
5. After the first publish, the GHCR package is private by default; link it to the repo or make it public as you prefer.

Release (Git Bash):
```bash
git switch main && git pull
git tag -a v0.1.0 -m "Phase 1 + Docker"
git push origin v0.1.0        # triggers CD
```

## Choosing a deployment provider (no provider is assumed yet)
Costs are rough orders of magnitude; check current pricing before deciding.

| Option | Typical cost | You operate | Good for | Watch out for |
|---|---|---|---|---|
| **VPS + Docker Compose** (Hetzner, DigitalOcean, Linode...) | ~$5-15/mo | OS patching, Docker, backups, TLS (Caddy/Traefik), firewall, monitoring | Learning, small production, full control, cheapest | Single point of failure, you own security updates and DB backups, downtime during deploy unless scripted |
| **Managed PaaS / serverless containers** (Cloud Run, Fly.io, Render, App Runner) | ~$0-30/mo at low traffic | Almost nothing but your app/config | Fast path to HTTPS + rollbacks | Provider lock-in, cold starts/scale-to-zero, DB is a separate paid service |
| **Managed Kubernetes** (GKE, EKS, AKS, DOKS) | cluster fee (often ~$70+/mo on some clouds, less on others) + nodes + LB | Manifests, upgrades of node pools, ingress/cert tooling, cost control | Teams already on K8s, multi-service, autoscaling | Expensive and complex for one small API; excellent for learning and interviews |
| **Local kind/minikube** | free | your laptop | Phase 2.5 practice | not a real deployment |

**Recommendation for this project:** use Kubernetes locally (2.5) for the learning goals, and deploy "for real" to a **single VPS with Compose** (cheapest, teaches the operations you'd otherwise hide) or a **PaaS** if you prefer zero server maintenance. Move to managed Kubernetes only when you have a reason (several services, autoscaling needs, team standardisation). In every case use a **managed PostgreSQL** or a scheduled `pg_dump` backup you have actually restored. Tell me which one you pick and I'll fill in the placeholder deploy step.

## Rollback strategy
- Images are **immutable and versioned**; rollback = redeploy a previous tag. Run CD manually with `image_tag=<previous good tag>` (skips the build) and the same `environment`.
- Health check failure fails the job and logs the rollback instruction. Kubernetes (`rollout undo`) and Compose-with-`--wait` can automate this later.
- **The database is the hard part.** Code rollback is easy; schema rollback is not. Use expand/contract migrations: add columns/tables first (old and new code both work), deploy, backfill, only later remove old columns. Never ship a migration that breaks the previous version.
- Keep at least the last 3 release images; GHCR retains them unless you delete them.

## Security notes
- No long-lived registry credentials: the workflow uses the short-lived `GITHUB_TOKEN`.
- Workflow `permissions` default to read-only; `${{ inputs.* }}` is passed via `env:` rather than interpolated into shell (prevents script injection).
- Third-party actions are pinned to major versions and kept fresh by Dependabot (`.github/dependabot.yml`). For stricter supply-chain control pin to full commit SHAs.
- CI's PostgreSQL password is a throwaway value for an ephemeral service container, not a real secret. Real secrets (SSH keys, cloud credentials) belong in Environment secrets.
- Pull requests from forks do not receive secrets; CI is designed to need none.

## What could fail in production
- Deploy succeeds but app is unhealthy -> health check step fails the run; rollback by tag.
- Two deploys race -> `concurrency` group serialises per environment.
- Tag pushed from a feature branch -> "Only release from main" guard rejects it.
- Flaky integration tests block merges -> keep them hermetic (per-run schema) and never share a DB between jobs.
- Cache poisoning/stale cache -> `setup-go` cache key includes `go.sum`.
- GHCR image private, server cannot pull -> use a read-only PAT/deploy token on the server.

## Interview questions
1. CI vs continuous delivery vs continuous deployment?
2. Why build the image once and promote it, rather than rebuilding per environment?
3. Why not tag images `latest` in production? How do digests differ from tags?
4. How do you keep secrets out of logs and forked-PR workflows? What is script injection in Actions?
5. How do you do a zero-downtime deploy on a single VPS? (second container + proxy switch, or rolling with Compose healthchecks.)
6. Explain expand/contract migrations and why rollback needs them.
7. What does `permissions:` do and why scope it per job?
8. Why run `-race` in CI, and what does it not catch?
9. A deploy passed CI but breaks in prod. What differs between CI and prod and how do you find out?
