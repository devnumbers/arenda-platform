# Agent Tooling

The living registry of this repo's agent infrastructure: which tools stand (MCP servers, skills, hooks, quality gates), why they're here, how they're wired, and how to reproduce the setup on a new machine. Every entry traces to a decision of the wayfinder map [Wayfinder: агентская инфраструктура репозитория — инструменты, правила, гейты](https://github.com/devnumbers/arenda-platform/issues/289); research fact-docs live in `docs/research/`.

**Status vocabulary:** `in force` — standing today · `accepted` — decided on the map, implementation goes through the normal pipeline (`/to-spec` → `/to-tickets` or targeted changes) · `rejected` — evaluated and ruled out, reason recorded.

**Lifecycle:** update this registry in the same change that alters the tooling, through the normal pipeline. Gate reviews are trigger-based, not calendar-based — review a gate when: bypassing it has become the norm, false positives show up, or a spike-gate fails.

## MCP servers

Configured per harness, outside the repo: Kimi Code — `~/.kimi-code/mcp.json`; ZCode — its own MCP config. API keys (context7, figma) are personal and live only in the user config, never in the repo.

| Server | Role | Status |
| --- | --- | --- |
| `lean-ctx` | Compressed reads and noisy-output compression, semantic search by meaning, dependency/diff-impact graph, session intelligence | in force — since 2026-08-16 narrowed to exactly this list (Serena landed; boundary rule below) |
| `gopls` | Go semantics (navigation, references, diagnostics) | in force — **accepted: removed** when Serena passes its spike-gate; `govulncheck` moves to a pinned make target |
| `context7` | Official library/framework documentation | in force |
| `playwright` | Browser automation and UI verification | in force |
| `figma` | Figma design data and image exports | in force |
| `heroui-react` | HeroUI v3 component docs, source, theme tokens | in force |
| `jetbrains` | GoLand inspections as a quality gate | in force — **accepted: removed** with Serena (coverage already exists: golangci-lint + live gopls diagnostics via Serena) |
| `serena` | Unified semantic tool for the stack (TS + Go): navigation, references, rename, diagnostics, symbol-level editing | in force — spike-gate passed 2026-08-16 (results below); pinned `serena-agent` 1.7.0 |

**Serena target configuration** (decision [«Подключать ли Serena (и в какой конфигурации)»](https://github.com/devnumbers/arenda-platform/issues/293)): one project at the monorepo root with `language_servers: ["typescript", "go"]` (fallback: two projects if one tsserver can't cover both `apps/frontend` and `apps/admin` — not a reject); context `ide` so Serena's file/shell tools auto-disable and don't duplicate the harness; memories off (the "no memory branch" charting decision stands) via the `no-memories` mode in `added_modes`; symbol editing on from day one; `.serena/project.yml` committed to the repo; mandatory with a stop-procedure (same model as gopls today — unavailable → stop and tell the user, no silent substitution); environment prerequisite `uv` + Python 3.13; `serena-agent` pinned at 1.7.0 (`uv tool install -p 3.13 'serena-agent==1.7.0'`).

**Spike-gate (run in both harnesses — Kimi Code and ZCode; for the 2026-08-16 run the ZCode leg was waived by user decision, see results):**

1. Serena's tsserver (pinned TS 5.9.3) starts on `apps/frontend` (Next 16) and `apps/admin` with meaningful diagnostics.
2. One root project covers both TS apps (failure → two projects, not a reject).
3. gopls via Serena works under `go.work` (navigation/references/diagnostics on the backend).
4. The agent actually calls Serena tools on 1–2 real tasks in both harnesses, without reminder hooks.
5. tsserver/gopls memory stays within hundreds of MB, not GB.
6. Symbol editing works correctly on TS and Go.

Failing (1), (3), (4), or (5) = reject with recorded reasons.

**Spike-gate results (run 2026-08-16, issue #299).** Setup: `serena-agent` 1.7.0 on uv-managed Python 3.13; one root project (`.serena/project.yml`), context `ide`, `no-memories` mode; Serena-pinned TS stack verified on disk: typescript 5.9.3 + typescript-language-server 5.1.3; gopls v0.23.0 from PATH. Exercised through a raw MCP stdio driver plus headless `kimi -p` sessions in Kimi Code. The ZCode leg was **waived by the user**: ZCode CLI headless cannot reuse the desktop app's OAuth tokens (the refresh flow lives in the desktop app; standalone headless needs its own `zcode login` plus an explicit `provider`+`model` section in `~/.zcode/cli/config.json`). Serena's entry is present in the ZCode MCP config and the running desktop starts the server with the target args, but tool *use* by the ZCode agent is unverified until first real use — if it misbehaves there, treat it as a late criterion-4 failure. Criteria 1, 2, 3, 5, 6 are harness-neutral (same Serena server, same project config, same tool surface for any MCP client) and hold for ZCode to the extent its MCP client is standards-conformant; only criterion 4 is genuinely per-harness, so per-criterion ZCode rows below read "waived".

1. **pass** — one tsserver cluster serves both `apps/frontend` (Next 16) and `apps/admin` from the root project: overview/find_symbol/find_referencing_symbols work in both; diagnostics are meaningful — a planted `TS2322` in a scratch file was reported with code/range/source, clean files return `{}`.
2. **pass, no fallback needed** — one root project covers both TS apps.
3. **pass** — gopls via Serena works under `go.work`: overview/find_symbol/find_referencing_symbols/diagnostics on `apps/backend` (probe: `PaymentLifecycle` in billing; cross-file references found).
4. **pass in Kimi Code** — across three headless tasks the agent called `get_diagnostics_for_file`, `rename_symbol`, `replace_symbol_body` and read back the results; no reminder hooks involved. Routing note: on a fully unhinted "find usages" task the agent picked text `Grep` over `find_referencing_symbols` — steering it is exactly what the boundary rule below is for. ZCode: not run (waived, see above).
5. **pass, measured** — Serena process-tree RSS after mixed TS+Go symbol work: tsserver cluster ≈426 MB warm (≈219 MB after editing/GC), gopls ≈517 MB (568 MB post-edit), serena python ≈90 MB. Hundreds of MB, not GB.
6. **pass** — symbol editing verified on disk: TS `replace_symbol_body` + `insert_after_symbol`; Go `replace_symbol_body` + `rename_symbol` with `go build`/`go vet`/`gofmt` clean. Contract note: in Go the symbol body excludes the leading doc comment — retrieve with `include_body` first (as Serena's own docs demand) or the comment gets duplicated.

Outcome: **adopt** — one project, no fallback; the MCP re-shuffle (remove `gopls` and `jetbrains` MCP servers, move `govulncheck` to a pinned make target) is confirmed and goes through the normal implementation pipeline. Operational notes: Kimi Code headless sessions can leave the Serena MCP process running after exit (kill leftover `serena start-mcp-server` processes); Serena self-writes `.serena/.gitignore` (`/cache`, `/project.local.yml`), which is committed.

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

Standing today (`in force`): gates run on discipline — "run the relevant checks before claiming completion" in the root AGENTS.md — with git hooks (lefthook, below), harness hooks (below) and CI in `.github/workflows/` as the enforcement backstop. The rules-translation packages at the end of this section are `accepted` (decision [«Какие правила переводим из прозы в инструментальные гейты»](https://github.com/devnumbers/arenda-platform/issues/295)), implementation pending.

### Git hooks — lefthook (in force since 2026-08-17)

- lefthook pinned v2.1.10, single `lefthook.yml` at the repo root, direct make/npm calls (no wrapper scripts), bootstrap via `make hooks-install` (installs the pinned binary when missing + `lefthook install`; idempotent, and the installed hook shims work even without GOPATH/bin on PATH via an absolute-path fallback). Rejected managers: pre-commit framework (new Python dependency), husky (requires a root `package.json` the repo doesn't have).
- pre-commit (checks only, no autofix; parallel, wall ≈ 7.5 s): staged `*.go` → full `make backend-lint` (~4 s warm; the first ever run compiles golangci-lint, ~40 s once); staged `*.ts/tsx` in `apps/frontend` → `npm run lint` (~7 s); staged `*.ts/tsx` in `apps/admin` → `npm run typecheck` (~2 s). No tests on pre-commit — commits stay cheap. Re-measured on hook validation 2026-08-17: single-trigger runs 3.3 / 6.9 / 2.1 s, all-three parallel 7.45 s, empty staged skips everything in 0.03 s.
- pre-push ("verify everything + security; time not critical", parallel): full `make test` (backend unit `-race` + integration via testcontainers + frontend + admin + tools — **Docker required**, `make test` blocks with a clear message and a `--no-verify` hint without it); `make backend-vulncheck` (pinned govulncheck v1.7.0); `make npm-audit` (`--audit-level=high` across the lockfile packages `apps/frontend`, `apps/admin`, `apps/landing`, `tools/property-attributes`, `tools/hooks`); `make trivy-fs` (pinned aquasec/trivy 0.74.0 image via docker, `scanners: vuln`, `severity: HIGH,CRITICAL`, `ignore-unfixed`, `--exit-code 1` — mirrors CI; the vuln DB is cached in the `arenda-trivy-cache` docker volume). Full run measured 2026-08-17: `make test` 68 s, govulncheck 6 s, trivy 59 s in parallel.
- **Known red state (2026-08-17):** `make npm-audit` and `make trivy-fs` fail on the current tree from pre-existing dependency advisories (frontend/admin/landing; the nightly CI trivy-fs has been failing on main for the same reason) — tracked in [#311](https://github.com/devnumbers/arenda-platform/issues/311). Until that debt is fixed, pushes need the documented bypass (`--no-verify`).
- Bypass documented: `LEFTHOOK=0`, `--no-verify`, personal `lefthook-local.yml` (gitignored).

### Harness hooks — personal opt-in layer (in force since 2026-08-17)

Both harnesses fail open and Kimi Code hooks are user-level config, so harness hooks are not a team gate. Versioned harness-neutral scripts in `tools/hooks/*.mjs` (contract: JSON on stdin → exit 0/2; internal errors fail open), covered by contract tests on vitest — `make tools-test`, part of `make test` and the CI `tools` job; the package carries its own package.json with vitest in devDependencies (the `tools/property-attributes` package pattern, extended with tests):

- `Stop` (`stop-gate.mjs`) → run the gates of the packages with **uncommitted** changes (staged + unstaged + untracked) before ending the turn. Uncommitted-only by design: committed changes are already gated at commit time by the lefthook pre-commit hook, so the two layers compose without double-gating the same diff (`--no-verify` remains the one documented deliberate bypass of both). Gates mirror pre-commit exactly — `make backend-lint`, `npm --prefix apps/frontend run lint`, `make admin-typecheck`, in parallel; keep in sync with `lefthook.yml`. Self-scoping: the script silently allows unless the session repo shares a git common dir with the script's own checkout (worktrees included) — the user-level config fires hooks in every session, and this keeps other projects quiet.
- `PreToolUse(Bash)` (`bash-guard.mjs`) → destructive-command guard: recursive `rm` (incl. `-rf`, sudo/env/xargs wrappers, `$(…)` substitutions, `bash -c`), `git clean` without dry-run, `git reset --hard`, `git push --force`/`-f` (`--force-with-lease` allowed), `docker system|volume prune`, `docker compose down -v`. Best-effort shell segmentation — a safety net against common agent mistakes, not a shell parser. Repo-agnostic by design: it guards every project on the machine, not just this monorepo.

No `PostToolUse` lint per edit: observation-only event, ~2.7 s per edit — friction without a win. Wiring: Kimi Code — opt-in `[[hooks]]` stanzas from `tools/hooks/README.md` into `~/.kimi-code/config.toml`; ZCode — deferred until its plugin mechanism matures (project hooks are ignored by design today).

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

1. Base prerequisites: Go toolchain (provides the `gopls` binary), Node/npx, Docker (integration tests; the pre-push hook runs the full `make test` and blocks without it).
2. Harness MCP config — Kimi Code: `~/.kimi-code/mcp.json`; ZCode: its MCP config. Servers per the table above; personal API keys (context7, figma) go into the user config only.
3. Skills: nothing to install — `.agents/skills/` ships with the repo.
4. Serena (in force since 2026-08-16): install `uv`, then `uv tool install -p 3.13 'serena-agent==1.7.0'` (uv provides the managed Python 3.13); `.serena/project.yml` comes from the repo; add the `serena` MCP entry (`serena start-mcp-server --context ide --project <abs repo root>`) to each harness config. ZCode note: headless CLI use needs a separate `zcode login` and an explicit `provider`+`model` section in `~/.zcode/cli/config.json`; the desktop app needs neither.
5. Git hooks (in force since 2026-08-17): `make hooks-install` — installs the pinned lefthook binary (v2.1.10) when missing via `go install`, then `lefthook install`; idempotent, re-run after cloning.
6. Harness hooks (opt-in, in force since 2026-08-17): copy the `[[hooks]]` stanzas from `tools/hooks/README.md` into `~/.kimi-code/config.toml`, substituting the absolute path to your checkout (Node ≥ 20.11; no install step — the scripts are dependency-free). ZCode wiring is deferred by decision #294.

## Source decisions

Map: [Wayfinder: агентская инфраструктура репозитория — инструменты, правила, гейты](https://github.com/devnumbers/arenda-platform/issues/289).

- [[research] Serena поверх lean-ctx + gopls: пересечение, цена, подключение](https://github.com/devnumbers/arenda-platform/issues/290) — facts: `docs/research/2026-08-16-serena-mcp-server.md`.
- [[research] Механизмы гейтов: хуки Kimi Code / ZCode, git pre-commit (lefthook)](https://github.com/devnumbers/arenda-platform/issues/291) — facts: `docs/research/2026-08-16-quality-gates-hooks.md`.
- [[research] Аудит правил AGENTS.md: инструмент / переводимо / проза](https://github.com/devnumbers/arenda-platform/issues/292) — facts: `docs/research/2026-08-16-agents-md-rules-audit.md`.
- [[grilling] Подключать ли Serena (и в какой конфигурации)](https://github.com/devnumbers/arenda-platform/issues/293) — Serena adopt-conditionally, spike-gate, MCP re-shuffle.
- [[grilling] Набор гейтов и хуков (harness + git pre-commit)](https://github.com/devnumbers/arenda-platform/issues/294) — lefthook pre-commit/pre-push, harness hooks.
- [[grilling] Какие правила переводим из прозы в инструментальные гейты](https://github.com/devnumbers/arenda-platform/issues/295) — five packages in three waves; facts: `docs/research/2026-08-16-migration-lint-and-arch-tools.md`.
- [[grilling] Надстройки над пайплайном Покока (дефолт: без изменений)](https://github.com/devnumbers/arenda-platform/issues/296) — pipeline status quo; skills catalog, gate-review triggers, spike-gate pattern, registry lifecycle moved into this registry.
