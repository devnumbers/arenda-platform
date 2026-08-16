# Agent Tooling

The living registry of this repo's agent infrastructure: which tools stand (MCP servers, skills, hooks, quality gates), why they're here, how they're wired, and how to reproduce the setup on a new machine. Every entry traces to a decision of the wayfinder map [Wayfinder: агентская инфраструктура репозитория — инструменты, правила, гейты](https://github.com/devnumbers/arenda-platform/issues/289); research fact-docs live in `docs/research/`.

**Status vocabulary:** `in force` — standing today · `accepted` — decided on the map, implementation goes through the normal pipeline (`/to-spec` → `/to-tickets` or targeted changes) · `rejected` — evaluated and ruled out, reason recorded.

**Lifecycle:** update this registry in the same change that alters the tooling, through the normal pipeline. Gate reviews are trigger-based, not calendar-based — review a gate when: bypassing it has become the norm, false positives show up, or a spike-gate fails.

## MCP servers

Configured per harness, outside the repo: Kimi Code — `~/.kimi-code/mcp.json`; ZCode — its own MCP config. API keys (context7, figma) are personal and live only in the user config, never in the repo.

| Server | Role | Status |
| --- | --- | --- |
| `lean-ctx` | Compressed reads and noisy-output compression, semantic search by meaning, dependency/diff-impact graph, session intelligence | in force — role narrows to exactly this list once Serena lands |
| `gopls` | Go semantics (navigation, references, diagnostics) | in force — **accepted: removed** when Serena passes its spike-gate; `govulncheck` moves to a pinned make target |
| `context7` | Official library/framework documentation | in force |
| `playwright` | Browser automation and UI verification | in force |
| `figma` | Figma design data and image exports | in force |
| `heroui-react` | HeroUI v3 component docs, source, theme tokens | in force |
| `jetbrains` | GoLand inspections as a quality gate | in force — **accepted: removed** with Serena (coverage already exists: golangci-lint + live gopls diagnostics via Serena) |
| `serena` | Unified semantic tool for the stack (TS + Go): navigation, references, rename, diagnostics, symbol-level editing | **accepted-conditionally** — spike-gate first, see below |

**Serena target configuration** (decision [«Подключать ли Serena (и в какой конфигурации)»](https://github.com/devnumbers/arenda-platform/issues/293)): one project at the monorepo root with `languages: ["typescript", "go"]` (fallback: two projects if one tsserver can't cover both `apps/frontend` and `apps/admin` — not a reject); context `ide` so Serena's file/shell tools auto-disable and don't duplicate the harness; memories off (the "no memory branch" charting decision stands); symbol editing on from day one; `.serena/project.yml` committed to the repo; mandatory with a stop-procedure (same model as gopls today — unavailable → stop and tell the user, no silent substitution); environment prerequisite `uv` + Python 3.13; `serena-agent` version pinned in the implementation docs.

**Spike-gate (run in both harnesses — Kimi Code and ZCode):**

1. Serena's tsserver (pinned TS 5.9.3) starts on `apps/frontend` (Next 16) and `apps/admin` with meaningful diagnostics.
2. One root project covers both TS apps (failure → two projects, not a reject).
3. gopls via Serena works under `go.work` (navigation/references/diagnostics on the backend).
4. The agent actually calls Serena tools on 1–2 real tasks in both harnesses, without reminder hooks.
5. tsserver/gopls memory stays within hundreds of MB, not GB.
6. Symbol editing works correctly on TS and Go.

Failing (1), (3), (4), or (5) = reject with recorded reasons.

**Boundary rule (one line):** code semantics (symbols, references, rename, diagnostics, symbol editing — TS and Go) → Serena; read/output compression, semantic search, dependency graph, session intelligence → lean-ctx.

## Skills pipeline

26 skills in `.agents/skills/` — the Matt Pocock pipeline, versioned with the repo, nothing to install. Fact from [«Надстройки над пайплайном Покока»](https://github.com/devnumbers/arenda-platform/issues/296): all skills are tool-agnostic (no references to specific MCP servers, hooks, or make targets), so tooling changes don't require skill edits.

Main flow (idea → ship):

- `grill-with-docs` — interview to sharpen an idea, creating docs (ADRs, glossary) along the way.
- `grill-me` — stateless interview to sharpen a plan or design.
- `to-spec` — conversation → spec published to the issue tracker.
- `to-tickets` — spec/plan → tracer-bullet tickets with blocking edges.
- `implement` — implement a ticket/spec; runs `tdd` inside, closes with `code-review`.
- `tdd` — red-green-refactor on pre-agreed seams; the default for every behavior change.
- `code-review` — review changes since a fixed point along Standards and Spec axes.

On-ramps:

- `triage` — move issues/PRs through the triage state machine to agent-ready briefs.
- `diagnosing-bugs` — diagnosis loop for bugs and performance regressions.
- `wayfinder` — plan multi-session efforts as a map of decision tickets on the tracker.
- `improve-codebase-architecture` — scan for module-deepening opportunities, grill through a pick.

Reference layer:

- `domain-modeling` — domain terminology, CONTEXT.md glossaries, ADR discipline.
- `codebase-design` — shared vocabulary for designing deep modules.

Utilities:

- `ask-matt` — router: which skill or flow fits the situation.
- `grilling` — relentless HITL grilling; invoked by other skills (wayfinder, implement).
- `research` — investigate against primary sources, capture findings in `docs/research/`.
- `prototype` — throwaway prototype to answer a design question.
- `to-questionnaire` — turn an unanswerable decision into a questionnaire for someone else.
- `handoff` — compact the conversation into a handoff document for another agent.
- `teach` — teach the user a new skill or concept in this workspace.
- `wait-what` — stop and re-pitch a message that didn't land.
- `resolving-merge-conflicts` — resolve in-progress merge/rebase conflicts.
- `wizard` — interactive bash wizard for human-only steps (credentials, dashboards).
- `writing-for-agents` — writing/editing skills, AGENTS.md, CLAUDE.md.
- `writing-great-skills` — reference for writing predictable skills.
- `setup-matt-pocock-skills` — one-time repo setup: tracker, triage labels, domain doc layout.

## Quality gates

Standing today (`in force`): gates run on discipline — "run the relevant checks before claiming completion" in the root AGENTS.md — with CI in `.github/workflows/` as the backstop. Everything below is `accepted` (decision [«Набор гейтов и хуков (harness + git pre-commit)»](https://github.com/devnumbers/arenda-platform/issues/294)), implementation pending.

### Git hooks — lefthook (accepted)

- lefthook pinned v2.1.10, single `lefthook.yml` at the repo root, direct make/npm calls (no wrapper scripts), bootstrap via a new `make hooks-install` target. Rejected managers: pre-commit framework (new Python dependency), husky (requires a root `package.json` the repo doesn't have).
- pre-commit (checks only, no autofix; parallel, wall ≈ 7.5 s measured 2026-08-16): staged `*.go` → full `make backend-lint` (~4 s warm); staged `*.ts/tsx` in `apps/frontend` → `npm run lint` (~7.4 s); staged `*.ts/tsx` in `apps/admin` → `npm run typecheck` (~1.9 s). No tests on pre-commit — commits stay cheap.
- pre-push ("verify everything + security; time not critical"): full `make test` (backend unit `-race` + integration via testcontainers + frontend + admin — **Docker required**, push blocks without it); `govulncheck` via a new pinned make target; `npm audit --audit-level=high` across the four lockfile packages (`apps/frontend`, `apps/admin`, `apps/landing`, `tools/property-attributes`); `trivy fs` over the repo root via docker (pinned image, `scanners: vuln`, `severity: HIGH,CRITICAL`, `ignore-unfixed`, `--exit-code 1` — mirrors CI; the existing CI image scan stays).
- Bypass documented: `LEFTHOOK=0`, `--no-verify`, personal `lefthook-local.yml` (gitignored).

### Harness hooks — personal opt-in layer (accepted)

Both harnesses fail open and Kimi Code hooks are user-level config, so harness hooks are not a team gate. Versioned harness-neutral scripts in `tools/hooks/*.mjs` (contract: JSON on stdin → exit 0/2):

- `Stop` → run the gates of the touched packages before ending the turn (blocking event in both harnesses — the agent's instant feedback).
- `PreToolUse(Bash)` → destructive-command guard (`rm -rf` and the like).

No `PostToolUse` lint per edit: observation-only event, ~2.7 s per edit — friction without a win. Wiring: Kimi Code — opt-in per instructions that land with the implementation change; ZCode — deferred until its plugin mechanism matures (project hooks are ignored by design today).

### Rules translated from prose to tools (accepted)

Decision [«Какие правила переводим из прозы в инструментальные гейты»](https://github.com/devnumbers/arenda-platform/issues/295): five packages in three waves by cost/win, plus closing the CI test gap. Translated rules get a one-line "enforced by X (config)" pointer in the relevant AGENTS.md — the prose compresses, it doesn't disappear.

- **Wave 1 — cheap, high win.** (A) depguard/forbidigo extensions in `.golangci.yml`: deny `gorm.io/*`, `entgo.io/*`, `xorm.io/*`; deny openapi-gen and `config` packages for domain/application layers; deny `github.com/minio/*`; forbidigo on `uuid.New()`/`uuid.NewV4()`. (B) freshness gates for generated code — `spec.gen.go` (oapi-codegen), sqlc output, `apps/frontend/shared/api/generated.ts`: make regenerate+diff targets after the `attributes-check` pattern + CI jobs. CI test gap: `frontend-test`, `admin-test`, backend integration (testcontainers) jobs in `ci.yml`.
- **Wave 2 — ESLint stack.** (C) ESLint into `apps/admin` from scratch: minimal flat config — `@typescript-eslint/no-explicit-any: error` + `no-restricted-imports` (dataProvider boundary, generated DTOs, URL literals). (D) FSD boundaries in `apps/frontend`: `eslint-plugin-boundaries` (v7.2.0, MIT, native flat config) for the `app→widgets→features→entities→shared` hierarchy + cross-slice ban; `no-restricted-imports` for `shared/api/generated.ts` outside `shared/api` and `@heroui/styles` in React.
- **Wave 3 — highest support cost.** (E) migration lint hybrid: Squawk (pinned binary from GitHub Releases via a make target, `.squawk.toml` at root with `assume_in_transaction = true`; excluded rules `prefer-identity`, `adding-serial-primary-key-field`, `prefer-bigint-over-int/smallint` — conflict with the uuid-v7-app-side ADR 0019) + a ~20-line domain-rules script (no `numeric/decimal/real/double/float` on money columns; no `DEFAULT` on `id`). Runs on pre-commit (staged `apps/backend/db/migrations/*.sql`), pre-push (full pass), and CI (backstop). Squawk covers `prefer-timestamptz` for free.

### Stays prose

41 audited rules stay as prose in the four AGENTS.md files (workflow/skills/TDD/process, domain language, semantic invariants) — separate coding-standards documents are not introduced: AGENTS.md is the only file guaranteed to load into every harness session. Advisory-only: `knip` (dead-code) runs as a make target + non-blocking CI report; promoting it to a blocking gate is a separate decision once its config stabilizes.

## Rejected

Evaluated and ruled out on the map — recorded so the question stays closed:

- `explicit-function-return-type` — noise on small utilities outweighs the win.
- Static money-int64 lint and money-formatting lint — unreliable without context (money is indistinguishable from int64 structurally); money schema is covered by migration lint (package E).
- Atlas lint — paid plan + dev-database requirement since v0.38.
- sqlfluff / pgspot / pganalyze-lint — wrong scope.
- steiger, dependency-cruiser — a second boundaries runner isn't justified.
- `@feature-sliced/eslint-config` — abandoned; conarti boundaries plugin — no license.
- go-arch-lint — duplicates depguard; goda — dev reconnaissance, not a gate.
- eslint-plugin-import-x — resolver duplication with eslint-config-next.
- pre-commit framework, husky — new runtimes / root `package.json` (see lefthook decision).
- `PostToolUse` lint hooks — friction without a win (see harness hooks).
- Calendar rituals (e.g. dependency-hygiene cadence) — trigger-based review instead.
- Cross-session memory branch — status quo stands: `handoff` skill + lean-ctx session functions + ADR/CONTEXT discipline (map out-of-scope).
- Paid/cloud tools — charting constraint: free and local only.

## Standing patterns

- **adopt-conditionally + spike-gate** — the standing pattern for adopting tools: define gate criteria up front, run a spike in both harnesses, failure = reject with recorded reasons, no configuration sprawl.
- **Trigger-based gate review** — review a gate when bypassing it becomes the norm, false positives appear, or a spike-gate fails; no calendar cadence.
- **Rule = tool where expressible** — otherwise prose stays in AGENTS.md with a pointer to the enforcing tool once translated.

## Why no ADRs

None of the map's decisions gets an ADR, by the "offer ADRs sparingly" rule (domain-modeling: hard to reverse + surprising without context + real trade-off — all three required). Every decision here is reversible in one PR (delete `lefthook.yml`, `tools/hooks/`, a config stanza), so the hard-to-reverse prong fails. The Serena candidate (mandatory `uv` + Python 3.13 prerequisite, dropping two MCP servers) is still one-PR reversible — the spike-gate exists precisely to keep rejection cheap. The "why" for each decision lives in this registry and the linked tickets.

## Reproduce on a new machine

1. Base prerequisites: Go toolchain (provides the `gopls` binary), Node/npx, Docker (integration tests; pre-push hooks once lefthook lands).
2. Harness MCP config — Kimi Code: `~/.kimi-code/mcp.json`; ZCode: its MCP config. Servers per the table above; personal API keys (context7, figma) go into the user config only.
3. Skills: nothing to install — `.agents/skills/` ships with the repo.
4. Once Serena lands: install `uv` + Python 3.13, pinned `serena-agent`; `.serena/project.yml` comes from the repo.
5. Once lefthook lands: `make hooks-install` (installs the pinned lefthook binary if missing + `lefthook install`).
6. Harness hooks (opt-in): wire `tools/hooks/*.mjs` per the instructions that land with the implementation change.

## Source decisions

Map: [Wayfinder: агентская инфраструктура репозитория — инструменты, правила, гейты](https://github.com/devnumbers/arenda-platform/issues/289).

- [[research] Serena поверх lean-ctx + gopls: пересечение, цена, подключение](https://github.com/devnumbers/arenda-platform/issues/290) — facts: `docs/research/2026-08-16-serena-mcp-server.md`.
- [[research] Механизмы гейтов: хуки Kimi Code / ZCode, git pre-commit (lefthook)](https://github.com/devnumbers/arenda-platform/issues/291) — facts: `docs/research/2026-08-16-quality-gates-hooks.md`.
- [[research] Аудит правил AGENTS.md: инструмент / переводимо / проза](https://github.com/devnumbers/arenda-platform/issues/292) — facts: `docs/research/2026-08-16-agents-md-rules-audit.md`.
- [[grilling] Подключать ли Serena (и в какой конфигурации)](https://github.com/devnumbers/arenda-platform/issues/293) — Serena adopt-conditionally, spike-gate, MCP re-shuffle.
- [[grilling] Набор гейтов и хуков (harness + git pre-commit)](https://github.com/devnumbers/arenda-platform/issues/294) — lefthook pre-commit/pre-push, harness hooks.
- [[grilling] Какие правила переводим из прозы в инструментальные гейты](https://github.com/devnumbers/arenda-platform/issues/295) — five packages in three waves; facts: `docs/research/2026-08-16-migration-lint-and-arch-tools.md`.
- [[grilling] Надстройки над пайплайном Покока (дефолт: без изменений)](https://github.com/devnumbers/arenda-platform/issues/296) — pipeline status quo; skills catalog, gate-review triggers, spike-gate pattern, registry lifecycle moved into this registry.
