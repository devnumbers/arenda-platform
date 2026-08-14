# AGENTS.md

## Scope

Language of communication: Russian.

Repository-level instructions only. Backend development rules live in `apps/backend/AGENTS.md`; frontend rules live in `apps/frontend/AGENTS.md`; admin rules live in `apps/admin/AGENTS.md`.

## Workflow System

All coding agents in this repo — Kimi Code, ZCode, or any other harness — work through a single workflow built on Matt Pocock's skills, installed locally in `.agents/skills/`. Invoke a skill by its exact name through the harness's native skill mechanism (`Skill` tool, `/skill-name`, etc.). The `## Workflow (Matt Pocock skills)` section below is the required workflow; per-app `AGENTS.md` files add stack-specific skills on top of it.

## Project Map

- Arenda Platform is a fintech platform for rental-property finance management for private owners and small rental businesses in Russia. The product is record-keeping, not money movement: owners record and plan income/expense operations; the only payment processing is the SaaS subscription via T-Kassa (`docs/adr/0036-fintech-domain-language.md`).
- Backend: `apps/backend`, a separate Go module linked by root `go.work`.
- Frontend: `apps/frontend`, a Next.js React application.
- Admin: `apps/admin`, a Vite + React SPA built on react-admin 5 and MUI 7 — a separate stack from the Next.js frontend; its rules live in `apps/admin/AGENTS.md`.
- Landing: `apps/landing`, a standalone Vite + React SPA (export from Figma Make) served by nginx as the public site at `/`.
- Product docs: `docs/`; domain glossary: `CONTEXT-MAP.md` (index) + per-context `CONTEXT.md` under `apps/backend/internal/<context>/`; architecture decisions: `docs/adr/`.
- Local infrastructure runs through `docker-compose.local.yml`; run the backend on the host with Go.
- Stage/prod deploy: GitHub Actions on a self-hosted runner builds images, pushes them to GHCR and deploys by digest over SSH (`docs/adr/0024-deploy-pipeline-ghcr-runner.md`). Environment values live in GitHub Environments (`ENV_FILE` secret, key set pinned to `.env.<env>.example`); nothing is built or hand-edited on the server. Operational procedures (bootstrap, secrets map, backup/restore, rollback) — `docs/deployment.md`.
- Observability: the self-hosted Uptrace stack collecting stage/prod logs, traces, and metrics with email alerts lives in the devnumbers/observability repo (compose project `observability`, server directory `/opt/observability`). Consumer-side connection for stage/prod is documented in `docs/deployment.md` (section "Observability"); the decision history is in `docs/adr/0021-centralized-observability-uptrace.md` (superseded — transferred to devnumbers/observability ADR 0001).

## Work Rules

- Before changing behavior, read the relevant per-context `CONTEXT.md` (start from `CONTEXT-MAP.md`), relevant docs under `docs/`, and relevant ADRs.
- Check `git status` before edits and do not overwrite unrelated user changes.
- Before adding new entities, helpers, use cases, interfaces, API contracts, or abstractions, search for existing equivalents and call sites with `Grep`, `lean-ctx`, and language-aware tools where available. Prefer existing project patterns over new conventions.
- Use `lean-ctx` for broad repository exploration, large or generated files, noisy command output, and repeated reads. Before editing a file or relying on exact line-level behavior, read the relevant source in full or in precise raw ranges.
- When a task depends on MCP tools, check that the server's `mcp__<name>__*` tools are present in the agent tool set before relying on them. If a server is unavailable, tell the user (they can inspect `/mcp` themselves), state the fallback clearly, and continue with standard repository tools where safe.
- Keep high-risk actions behind explicit intent: destructive commands, credentials, migrations, external services, and dependency changes require normal repository safeguards, not broad wildcard trust such as `mcp__*`.
- Keep context small: summarize decisions, touched files, commands, and unresolved risks; clear unrelated context between separate tasks.
- Before claiming completion, run the relevant project checks and perform a fresh review of the diff for duplication, security regressions, and instruction conflicts.
- Before claiming completion, run the test suite: `make test` (full — backend unit + integration via testcontainers + frontend + admin). Requires Docker. For a fast feedback loop during work, use the granular targets: `make backend-test` (unit only, no Docker needed), `make frontend-test`, `make admin-test`. See `docs/testing-strategy.md` for the full test contract.

## Workflow (Matt Pocock skills)

Main flow (idea → ship):
1. `/grill-with-docs` — interview to sharpen the idea before implementation (start here when there is a working directory; use `/grill-me` stateless when there is not). It challenges the plan against the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), `docs/`, and `docs/adr/`, and updates documentation only when a real glossary or ADR decision changes.
2. For multi-session builds: `/to-spec` → `/to-tickets` (tracer-bullet vertical slices with blocking edges).
3. `/implement` per ticket — runs `/tdd` inside on pre-agreed seams and closes with `/code-review`. `/clear` between tickets. For a single-session task, run `/implement` directly after grilling.

On-ramps (merge into the main flow):
- `/triage` — incoming issues and external requests (not ones you created).
- `/diagnosing-bugs` — something is broken, throwing, failing, or slow.
- `/wayfinder` — a huge, foggy effort; when the map is clear, hand off to `/to-spec`, not straight to `/implement`.
- `/improve-codebase-architecture` — codebase health; a picked candidate feeds back into `/grill-with-docs`.

Reference layer other skills invoke: `/domain-modeling` (domain language in per-context `CONTEXT.md` files), `/codebase-design` (deep-module vocabulary).

`/tdd` is the default: build every behavior change test-first (red-green-refactor) on pre-agreed seams, without waiting to be asked.

## MCP Servers

The following MCP servers are configured in the harness MCP config (`~/.kimi-code/mcp.json` for Kimi Code, the ZCode MCP config for ZCode):

- `lean-ctx` — broad repository exploration, compressed reads, semantic search, file trees, shell compression, and session intelligence. Fallback when unavailable: native `Read`, `Grep`, `Glob`, `Bash`.
- `gopls` — Go semantic navigation, definitions, references, diagnostics, package APIs, and impact checks. Mandatory for backend work; if unavailable, follow the stop procedure in `apps/backend/AGENTS.md` rather than substituting other tools.
- `context7` — official library and framework documentation. Fallback when unavailable: official docs via `FetchURL` or `WebSearch`.
- `playwright` — browser automation and UI verification. Used by frontend work. Fallback when unavailable: manual inspection, build logs, or native browser tools.
- `figma` — Figma design data and image exports. Fallback when unavailable: manual design references.
- `heroui-react` — HeroUI v3 component docs, source, and theme tokens for the Next.js frontend. HeroUI v3 is beta and not covered by model training data, so verify components through this server before writing HeroUI code. Fallback when unavailable: official docs at https://v3.heroui.com via `FetchURL` or `WebSearch`.
- `jetbrains` — GoLand's built-in MCP server (IDE 2025.2+). Runs IDE inspections (`get_file_problems`) on files as a quality gate for agent work. Requires GoLand running with this project open. Fallback when unavailable: note the skipped gate in the final report.

## Commands

```bash
cp .env.example .env
# Fill required local values in .env before backend-run.
make local-infra-up
make local-infra-down
make backend-run
make backend-lint
make backend-test
make backend-test-integration
make frontend-test
make admin-test
make test
make admin-install
make admin-dev
make admin-build
make admin-typecheck
make migrate-up
make migrate-down
make landing-install
make landing-dev
make landing-build
```

Use `make local-infra-reset` only when intentionally deleting local Docker volumes, including database data.

## Repository Conventions

- Do not create or switch to a git worktree by default. Work in the current checkout and current branch unless the user explicitly asks for a worktree or branch isolation.
- If a generic skill recommends a worktree, this repository rule overrides it.
- Money is stored as `BIGINT` in kopecks across the backend and crosses every layer (API, frontend, admin) as integer kopecks — never floats; money arithmetic is integer-only. See `docs/adr/0008-subscription-lifecycle.md`.
- All amounts are in RUB; there is no multi-currency support (`docs/adr/0036-fintech-domain-language.md`).
- Format money for display only at the UI layer: frontend — `formatMoneyKopecks` (`apps/frontend/shared/lib/format-money.ts`); admin — `formatKopecks` / `MoneyField` (`apps/admin/src/fields.tsx`).
- Two money vocabularies (`docs/adr/0036-fintech-domain-language.md`): rental money records are Операции — «платёж»/«транзакция» belong to Billing (T-Kassa processing) only. Canonical terms and `_Avoid_` lists live in `CONTEXT-MAP.md` and the per-context `CONTEXT.md` files.

## Agent skills

### Issue tracker

Issues live in this repo's GitHub Issues (`devnumbers/arenda-platform`), managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Multi-context: root `CONTEXT-MAP.md` (context index + shared kernel) + per-context `CONTEXT.md` co-located under `apps/backend/internal/<context>/` + `docs/adr/`. See `docs/agents/domain.md`.
