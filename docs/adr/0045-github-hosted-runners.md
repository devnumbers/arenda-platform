# ADR 0045: Deploy Pipeline on GitHub-Hosted Runners

## Status

Accepted — supersedes §5 of ADR 0024 (the self-hosted runner on the production
VPS). Everything else in ADR 0024 stands: digest deploys from GHCR, cosign
signing and verification, the `ENV_FILE` GitHub-Environments model with
example-file drift checks, backup → migrate → rollout ordering, and the
expand-contract migration policy.

## Context

ADR 0024 §5 deliberately placed the Actions runner on the production VPS
(cost/simplicity for a single-owner project), compensating with
`max-parallel: 2` on the image build matrix — the host has limited RAM and no
swap, and a Next.js build alone takes 1.5–2 GB. ADR 0024 §6 deferred the move
to GitHub-hosted runners "when the load or the team grows".

On 2026-08-21 the deferred item became current: a deploy wedged on the runner —
the frontend image build failed in ~3 minutes while the three remaining builds
hung for close to an hour — with deploys on this runner having failed
repeatedly since 2026-08-18. Building images on the box that runs prod is the
root cause the §5 compensations were papering over. At the same time the CI
jobs, already on GitHub-hosted runners, were green and fast in the very same
runs.

The owner confirmed: move everything off the self-hosted runner (including the
`security.yml` semgrep/trivy jobs), accept the Actions-minutes cost and watch
it for the first month, open the server's SSH to the internet so deploy jobs
can reach it from GitHub's ephemeral runner IPs, and decommission the
VPS runner afterwards.

## Decision

All workflow jobs run on `ubuntu-latest` (4 vCPU / 16 GB):

- `deploy-stage.yml` / `deploy-prod.yml`: the `images` build matrix and
  `collect` jobs. The `max-parallel: 2` cap is removed — its only rationale
  was VPS RAM; hosted runners build all four images in parallel.
- `_deploy.yml`: the deploy job. scp/ssh now arrive from the internet; the
  server accepts SSH from any source, compensated by key-only auth, the pinned
  host-key fingerprint already in secrets, and fail2ban on the host.
- `security.yml`: semgrep and the nightly trivy-fs scan. The semgrep
  container's `--user 997:981` mapping — a workaround for the self-hosted
  runner's checkout ownership — is dropped.

The self-hosted runner is decommissioned from the VPS (service stop, config
removal, deregistration in repository settings); the runbook lives in
`docs/deployment.md`. This frees the RAM the builds were competing with prod
for.

Billing: the repository is private, so every hosted-runner minute counts
against the plan. Roughly +15 billed minutes per push to `dev` on top of the
~30 the CI phase already spends. Accepted; to be revisited if the first
month's usage says otherwise.

## Consequences

- (+) Builds no longer compete with the running stack for RAM — the observed
  failure mode (wedged/hung image builds on the VPS) is structurally gone.
- (+) ADR 0024 §5's accepted risk is gone entirely: a runner compromise is no
  longer a prod compromise, and the fork-PR/`max-parallel` compensations it
  required become moot.
- (+) Deploys get faster and more predictable: four parallel image builds on
  uniform machines with the existing `type=gha` build cache.
- (~) The server's SSH is reachable from the internet; the exposure is
  compensated (key-only, pinned host key, fail2ban) but is a real change from
  effectively-localhost deploys.
- (~) Actions minutes are a recurring cost and shared-availability dependency
  on GitHub's runner fleet.
- (−) Deploy SSH latency from external runners is slightly higher than from
  the on-host runner; negligible against build times.

## See also

- [`docs/adr/0024-deploy-pipeline-ghcr-runner.md`](./0024-deploy-pipeline-ghcr-runner.md)
  — the pipeline this ADR re-hosts; §5 is superseded, the rest stands.
- [`docs/deployment.md`](../deployment.md) — updated flow description and the
  runner decommission runbook.
