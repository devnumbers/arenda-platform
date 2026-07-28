# ADR 0024: Deploy Pipeline — GHCR Digest Deploys from a Self-Hosted Runner

## Status

Accepted (amended 2026-07-28: deploy chat notifications removed entirely;
dependency vulnerability scanning moved from the osv-scanner job in
`security.yml` to Dependabot — security updates plus grouped weekly version
updates targeting `dev` (`.github/dependabot.yml`) — with govulncheck in
`ci.yml`; `security.yml` keeps only SAST (semgrep) and the nightly trivy-fs
scan)

## Context

The old deploy pipeline worked directly on the server: a git checkout of the
repository under `/opt/arenda/{stage,prod}`, images built on the host with
`docker compose up -d --build`, environment files hand-edited on the server,
and database migrations applied automatically at backend startup. This had
several problems:

- Building Next.js images on the production VPS competes for RAM with the
  running stack (shared host, swap=0, previously OOM-prone).
- What is deployed is not what was verified: server-built images are not
  reproducible artifacts, and a rollback requires rebuilding an old commit.
- Secrets live only on the server's disk; drift between the env files and the
  configuration the code expects is detected by nothing.
- Migrations run at application startup, so a failed migration blocks the
  service from starting and there is no fresh backup taken right before the
  schema changes.
- No provenance: nothing ties a running container to a specific commit that
  passed CI.

Requirements confirmed by the owner: keep the self-hosted runner on the same
VPS (accepted risk, compensated — see below); auto-deploy stage on push to
`dev` and prod on push to `main` with no manual approval gate; migrations as
a separate deploy step after a fresh `pg_dump` backup; env transferred into
GitHub Environments secrets.

## Decision

### 1. Build once on the runner, deploy by digest

All four images (`backend`, `frontend`, `admin`, `landing`) are built on the
self-hosted runner in GitHub Actions, scanned with Trivy, pushed to GHCR
(`ghcr.io/devnumbers/arenda-planform-<service>:<sha>`), and signed with cosign
keyless (GitHub Actions OIDC). Deploy pulls the images **by digest**:
`ghcr.io/devnumbers/arenda-planform-<service>@sha256:<64 hex>`. The deploy
job validates each digest ref and runs `cosign verify` with the certificate
identity pinned to the `deploy-(stage|prod).yml` workflows on the `dev`/`main`
branches — a deploy can only run images built and signed by this repository's
own pipeline. Everything happens in **one workflow run per environment**
(`ci` → `images` → `collect` → `deploy`), digests flow through artifacts and
job outputs; there is no `workflow_run` chaining.

Server-side git checkouts and `up -d --build` are removed. Compose files
reference images via `${BACKEND_IMAGE}` etc., resolved from the env file
rendered at deploy time. Rollback is a `workflow_dispatch` of the deploy
workflow with the previous release's digest refs — minutes, no rebuild.

### 2. Environment variables from GitHub Environments

Each environment (`stage`, `production`) holds a single multiline secret
`ENV_FILE` with the full variable set. At deploy time the runner renders it to
a file and **compares the key set against `.env.<env>.example` from the
repository** — the example files are the source of truth for *which* keys
exist, the secret holds the *values*; any mismatch fails the deploy (drift
protection). The rendered file is copied to the server (`chmod 600`, previous
copy kept as `.env.<env>.prev`) and swapped atomically. No secrets are edited
on the server by hand anymore.

Environments have deployment branch policies (`stage` ← `dev`,
`production` ← `main`), the repository forbids fork-PR workflows, and
`.github/` is protected by CODEOWNERS — this ties "commit on the right
branch" to "may deploy".

### 3. Migrations as a separate deploy step, after a backup

The deploy script on the server, in order:

1. records the currently running images into `.previous-images` (for
   rollback);
2. takes a `pg_dump | gzip` backup into `/opt/arenda/backups/<env>/`
   (`chmod 700`), verifies the archive (`gzip -t` + size check), keeps the
   last 10, and uploads an off-site copy to REG.RU S3 (warn-only on failure);
3. `docker compose pull`;
4. runs migrations as a dedicated compose service (`profiles: ["migrate"]`,
   `./arenda-api migrate`) — a failure here stops the deploy while the old
   version keeps running;
5. rolls out with `up -d --wait --remove-orphans`.

`AUTO_MIGRATE=false` is set for the backend in stage/prod compose files, so
the application never migrates at startup there. Local development keeps the
`AUTO_MIGRATE=true` default.

Any failure during the rollout or the external smoke checks triggers an
**automatic rollback of the images** to `.previous-images`. The rollback
**never runs `migrate down`**: the database may already be on the newer
schema, and the log states this explicitly. Restoring
the database means restoring a dump manually — the procedure is documented in
`docs/deployment.md` and rehearsed on stage.

### 4. Migration policy: expand-contract

Because rollbacks never downgrade the schema, migrations must be
backward-compatible with the previously deployed code:

- Destructive changes (drop column/table, rename, tighten constraints) ship
  in **two releases**: first the expand release (new shape + code that works
  with both), then, after the old code is gone everywhere, the contract
  release that removes the old shape.
- On large tables use `CREATE INDEX CONCURRENTLY` (outside a transaction)
  instead of plain `CREATE INDEX`.
- `down` migrations still exist and are exercised in CI against an ephemeral
  database (`up → down -all → up`); they are never run automatically against
  a real database.

The pull request template carries a checklist for these rules.

### 5. Accepted risk: the self-hosted runner lives on the production VPS

The runner builds images on the same host that runs prod. This is a
deliberately accepted risk (cost/simplicity for a single-owner project), with
compensating controls:

- `max-parallel: 2` on the image build matrix (a Next.js build takes
  1.5–2 GB; the host has limited RAM and no swap);
- fork pull request workflows are disabled, so untrusted code never executes
  on the runner;
- image cleanup on the server is targeted (only our
  `ghcr.io/devnumbers/arenda-planform-*` images, keeping current + previous) —
  never a global `docker image/system prune`, since the host runs other
  projects;
- environment branch policies, branch protection with required checks, and
  CODEOWNERS on `.github/` bound who can trigger builds and deploys.

### 6. Deferred

- Encrypting backups (age) — dumps contain PII; for now they are protected by
  `chmod 700` directories and a private S3 bucket.
- A separate `MIGRATION_DATABASE_URL` with a DDL-only database user —
  migrations currently run as the application user.
- Moving to GitHub-hosted runners when the load or the team grows.

## Consequences

- (+) Deploys are reproducible and provable: a digest-pinned, cosign-verified
  image that passed CI is exactly what runs; `/healthz` reports the deployed
  commit sha, and the smoke check asserts it.
- (+) Rollback is fast and rehearsed (`workflow_dispatch` with previous
  digests); image-only rollback is automatic on smoke failure.
- (+) Every deploy starts from a fresh, verified database backup with an
  off-site copy; schema changes are decoupled from application startup.
- (+) Env drift between GitHub and the repository fails the deploy instead of
  silently breaking the stack; rotating env values is a single-secret update.
- (+) The server no longer needs the repository checkout, build toolchain, or
  write access to source code — only a read-only GHCR token.
- (~) Each deploy causes 5–15 s of backend downtime (single instance);
  accepted.
- (~) The pipeline depends on Sigstore (Fulcio/Rekor) reachability from the
  runner for cosign sign/verify.
- (~) Runner compromise is prod compromise (accepted risk, compensated as
  above); this must be revisited if the team or the threat model changes.
- (−) Rolling back the database is a manual dump restore; expand-contract
  discipline is required from every migration author, enforced by review and
  the PR checklist, not by tooling.

## See also

- [`docs/plans/2026-07-28-deploy-pipeline-redesign-design.md`](../plans/2026-07-28-deploy-pipeline-redesign-design.md)
  — the approved design (goals, architecture, differences from the
  byron-menu reference).
- [`docs/deployment.md`](../deployment.md) — operational procedures:
  bootstrap, secrets map, backup/restore, rollback.
- [`docs/adr/0021-centralized-observability-uptrace.md`](./0021-centralized-observability-uptrace.md)
  — the observability stack on the same VPS; deploy markers are exported via
  `OTEL_RESOURCE_ATTRIBUTES` `service.version`.
