# AGENTS.md

## Scope

Language of communication: Russian.

Repository-level instructions only. Backend development rules live in `apps/backend/AGENTS.md`; frontend rules live in `apps/frontend/AGENTS.md`; admin rules live in `apps/admin/AGENTS.md`.

## Project

Arenda Platform is a fintech platform for rental-property finance management for private owners and small rental businesses in Russia. The product is record-keeping, not money movement: owners record and plan income/expense operations; the only payment processing is the SaaS subscription via T-Kassa (`docs/adr/0036-fintech-domain-language.md`).

## Map

- Backend: `apps/backend` — a Go module linked by root `go.work`; local infra runs through `apps/backend/docker-compose.local.yml`, the backend itself on the host with Go.
- Frontend: `apps/frontend` — a Next.js React application.
- Admin: `apps/admin` — a Vite + React SPA on react-admin 5 and MUI 7, a separate stack from the frontend.
- Landing: `apps/landing` — a standalone Vite + React SPA served by nginx as the public site at `/`.
- Product docs: `docs/`; domain glossary: `CONTEXT-MAP.md` (index) + per-context `CONTEXT.md` under `apps/backend/internal/<context>/`; architecture decisions: `docs/adr/`; agent workflow registry: `docs/agents/`.
- Stage/prod deploy: GitHub Actions → GHCR → SSH, nothing built or hand-edited on the server (`docs/deployment.md`, `docs/adr/0024`, `docs/adr/0045`); observability — the Uptrace stack in devnumbers/observability (`docs/deployment.md`, section "Observability").

## Workflow System

All coding agents in this repo — Kimi Code, ZCode, or any other harness — work through a single workflow built on Matt Pocock's skills, installed locally in `.agents/skills/`. Invoke a skill by its exact name through the harness's native skill mechanism (`Skill` tool, `/skill-name`, etc.). The workflow below is required; per-app `AGENTS.md` files add stack-specific skills on top of it.

## Workflow (Matt Pocock skills)

Main flow (idea → ship): `/grill-with-docs` — interview to sharpen the idea against `CONTEXT-MAP.md`, `docs/`, and `docs/adr/` (start here when there is a working directory; `/grill-me` when there is not) → for multi-session builds `/to-spec` → `/to-tickets` (tracer-bullet vertical slices with blocking edges) → `/implement` per ticket — it runs `/tdd` inside on pre-agreed seams and closes with `/code-review`; `/clear` between tickets. For a single-session task, run `/implement` directly after grilling. Frontend tickets with screens or interactions additionally close through `/ui-walkthrough` — live acceptance in the visible browser — before the commit. A finished multi-ticket effort — a `/wayfinder` map or a `/to-tickets` build — closes through `/pre-merge`: one dynamic workflow runs the whole gate (branch-scoped architecture pass, a review loop that auto-fixes every finding until none remain, full test suites, fresh `dev` integration) and merges the branch into local `dev` when everything is clean. Push stays the owner's explicit word.

On-ramps: `/triage` — incoming issues and external requests (not ones you created); `/diagnosing-bugs` — something is broken, throwing, failing, or slow; `/wayfinder` — a huge, foggy effort (when the map is clear, hand off to `/to-spec`, not straight to `/implement`); `/improve-codebase-architecture` — codebase health, a picked candidate feeds back into `/grill-with-docs`.

Reference layer other skills invoke: `/domain-modeling` (domain language in per-context `CONTEXT.md` files), `/codebase-design` (deep-module vocabulary). `/tdd` is the default: build every behavior change test-first (red-green-refactor) on pre-agreed seams, without waiting to be asked.

## Repo rules

- Before changing behavior, read the relevant per-context `CONTEXT.md` (start from `CONTEXT-MAP.md`), relevant docs under `docs/`, and relevant ADRs.
- Check `git status` before edits and do not overwrite unrelated user changes.
- Before adding new entities, helpers, use cases, interfaces, API contracts, or abstractions, search for existing equivalents and call sites. Prefer existing project patterns over new conventions.
- Money is stored as `BIGINT` in kopecks across every layer — integer-only arithmetic, all amounts in RUB, no multi-currency (schema-side: `make migrations-lint`); format for display only at the UI layer: `formatMoneyKopecks` (frontend) / `formatKopecks` / `MoneyField` (admin). The two money vocabularies — record-keeping (Payments) vs Billing/T-Kassa — never mix; canonical terms and `_Avoid_` lists: `CONTEXT-MAP.md` ("Money vocabularies"), `docs/adr/0036`.
- Before claiming completion, run the test suite: `make test` (full; requires Docker). Fast loop during work: `make backend-test` (unit only, no Docker), `make frontend-test`, `make admin-test`, `make tools-test`. Test contract: `docs/testing-strategy.md`. Also review the fresh diff for duplication, security regressions, and instruction conflicts.
- No git worktrees by default — work in the current checkout and branch; this overrides generic skills. Worktree isolation runs only on the user's explicit request via `/using-git-worktrees`; the verified parallel-dev process (slots, `make worktree-new`, integration, teardown) lives in `docs/agents/parallel-dev.md`.
- All repository browser work goes through the playwright MCP — the built-in Browser Use pane is for ordinary web surfing only (`docs/agents/parallel-dev.md`).
- High-risk actions stay behind explicit intent: destructive commands, credentials, migrations, external services, and dependency changes require normal repository safeguards, not broad wildcard trust such as `mcp__*`.
- Commands: `make help` prints the full list grouped by section. Tool and language versions are pinned in the Makefile (`GO_VERSION`, `NODE_VERSION`, migrate CLI) — bump there, run `make versions-sync`, commit the stamped files; never hand-edit a version inside `ci.yml`, a Dockerfile, `go.work`/`go.mod`, or `.nvmrc`. `make local-infra-reset` deletes local Docker volumes including the database — run only with that intent.
- Code semantics (symbols, references, rename, diagnostics) → `serena`; compressed reads, semantic search by meaning, dependency graph, session intelligence → `lean-ctx`. MCP servers, their config locations, and fallback rules: `docs/agents/mcp.md` — read before relying on an `mcp__<name>__*` tool or when a server is unavailable.

## Agent skills

### Issue tracker

Issues live in this repo's GitHub Issues (`devnumbers/arenda-platform`), managed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Multi-context: root `CONTEXT-MAP.md` (context index + shared kernel) + per-context `CONTEXT.md` co-located under `apps/backend/internal/<context>/` + `docs/adr/`. See `docs/agents/domain.md`.

### Skills ownership

Every skill that any `AGENTS.md` in this repo names for invocation lives in the project `.agents/skills/` and is versioned with the code. Do not reference user-level or community skills from repo docs — a mandate that depends on a machine-local skill is unenforceable. Project-level skills shadow same-named user-level ones; when a skill is vendored or replaced here, delete the user-level copy so nobody edits the wrong file.
