# Agent Tooling

The living registry of this repo's agent infrastructure: which tools stand (MCP servers, skills, hooks, quality gates), why they're here, how they're wired, and how to reproduce the setup on a new machine. The source decisions live at the bottom; research fact-docs in `docs/research/`; the full decision history — in the issue tracker.

**Status vocabulary:** `in force` — standing today · `accepted` — decided, implementation goes through the normal pipeline (`/to-spec` → `/to-tickets` or targeted changes) · `removed` — was in force, then dropped (replacement or reason recorded) · `rejected` — evaluated and ruled out, reason recorded.

**Lifecycle:** update this registry in the same change that alters the tooling, through the normal pipeline. Gate reviews are trigger-based, not calendar-based — review a gate when: bypassing it has become the norm, false positives show up, or a spike-gate fails.

## MCP servers

Server roles, config locations, and fallback rules live in `docs/agents/mcp.md` (single source); this table records only the adoption status and the decisions behind it. API keys (context7, figma) are personal and live only in the user config, never in the repo.

| Server | Status |
| --- | --- |
| `lean-ctx` | in force |
| `context7` | in force |
| `playwright` | in force — the mechanism for **all** repo browser work; the rule text: `docs/agents/parallel-dev.md` |
| Browser Use (ZCode plugin, built-in pane) | restricted — barred from repo browser tasks: all chats share one cookie-partition (auth sessions on `127.0.0.1` cross) and there is one visible pane per window, so parallel-session isolation is impossible |
| `figma` | in force |
| `heroui-react` | removed — снят вместе со слоем: HeroUI снесён целиком (ADR 0050, аменд 2026-09-26, тикет #901); для компонентов источник — доки shadcn/ui и Radix |
| `serena` | in force — mandatory with a stop-procedure; pinned `serena-agent` 1.7.0 |
| `gopls` | removed — replaced by Serena (same gopls binary underneath, launched by Serena from PATH); `govulncheck` lives only as the pinned make target `make backend-vulncheck` |
| `jetbrains` | removed — coverage already exists: golangci-lint + live diagnostics via Serena |

**Serena configuration:** one project at the monorepo root with `language_servers: ["typescript", "go"]`; context `ide` so Serena's file/shell tools auto-disable; memories off (`no-memories` mode); symbol editing on; `.serena/project.yml` committed to the repo; mandatory with a stop-procedure (unavailable → stop and tell the user, no silent substitution); environment prerequisite `uv` + Python 3.13; `serena-agent` pinned at 1.7.0 (`uv tool install -p 3.13 'serena-agent==1.7.0'`). The `gopls` and `jetbrains` MCP entries are gone from both harness configs — Serena is the only semantic server. Adoption went through the adopt-conditionally + spike-gate pattern (criteria and results: issue #299); operational notes: Kimi Code headless sessions can leave the Serena MCP process running after exit (kill leftover `serena start-mcp-server` processes); Serena self-writes `.serena/.gitignore` (`/cache`, `/project.local.yml`), which is committed.

**Boundary rule (one line):** code semantics (symbols, references, rename, diagnostics, symbol editing — TS and Go) → Serena; read/output compression, semantic search, dependency graph, session intelligence → lean-ctx.

## Skills pipeline

Skills in `.agents/skills/` — the Matt Pocock pipeline, versioned with the repo, nothing to install (the live list is the directory). All skills are tool-agnostic (no references to specific MCP servers, hooks, or make targets), so tooling changes don't require skill edits.

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
- `wizard` — interactive bash wizard for human-only steps (credentials, dashboards).
- `writing-for-agents` — writing/editing skills, AGENTS.md, CLAUDE.md.
- `setup-matt-pocock-skills` — one-time repo setup: tracker, triage labels, domain doc layout.

## Quality gates

Standing gates run on discipline — "run the relevant checks before claiming completion" in the root AGENTS.md — with git hooks (lefthook, below), harness hooks (below), and CI in `.github/workflows/` as the enforcement backstop.

### Git hooks — lefthook

- lefthook pinned v2.1.10, single `lefthook.yml` at the repo root, direct make/npm calls (no wrapper scripts), bootstrap via `make hooks-install` (installs the pinned binary when missing + `lefthook install`; idempotent; hook shims work without GOPATH/bin on PATH via an absolute-path fallback). Rejected managers: pre-commit framework (new Python dependency), husky (requires a root `package.json` the repo doesn't have).
- pre-commit (checks only, no autofix; parallel, ≈7.5 s wall): staged `*.go` → full `make backend-lint` and `make backend-nolint FILES="{staged_files}"` (zero-nolint gate); staged `*.ts/tsx` in `apps/frontend` → `npm run lint`; staged `*.ts/tsx` in `apps/admin` → `make admin-typecheck` + `npm run lint` (both jobs parallel); staged migrations → `make migrations-lint FILES="{staged_files}"`. No tests on pre-commit — commits stay cheap.
- pre-push ("verify everything + security; time not critical", parallel): full `make test` (backend unit `-race` + integration via testcontainers + frontend + admin + tools — **Docker required**; `make test` blocks with a `--no-verify` hint without it); `make migrations-lint` (full pass); `make backend-vulncheck` (pinned govulncheck); `make npm-audit` (verdicts from `tools/npm-audit-gate.mjs` over `npm audit --json` across the `NPM_AUDIT_DIRS` lockfile packages in the Makefile: high/critical with an available fix fail; unfixed advisories (`fixAvailable === false`) and major-shift-only "fixes" are tolerated — aligned with the trivy `ignore-unfixed` policy; 3 retries per directory for registry flakiness); `make trivy-fs` (pinned trivy image via docker, `scanners: vuln`, `severity: HIGH,CRITICAL`, `ignore-unfixed`, `--exit-code 1` — mirrors CI; the vuln DB is cached in the `arenda-trivy-cache` docker volume). npm audit additionally blocks PRs with lockfile changes (dependency-contour entry below).
- Bypass documented: `LEFTHOOK=0`, `--no-verify`, personal `lefthook-local.yml` (gitignored) — the documented deliberate bypass for red windows.

### Harness hooks — personal opt-in layer

Both harnesses fail open and Kimi Code hooks are user-level config, so harness hooks are not a team gate. Versioned harness-neutral scripts in `tools/hooks/*.mjs` (contract: JSON on stdin → exit 0/2; internal errors fail open), covered by contract tests on vitest — `make tools-test`, part of `make test` and the CI `tools` job:

- `Stop` (`stop-gate.mjs`) → run the gates of the packages with **uncommitted** changes (staged + unstaged + untracked) before ending the turn. Uncommitted-only by design: committed changes are already gated at commit time; the two layers compose without double-gating (`--no-verify` remains the one documented deliberate bypass of both). Gates mirror pre-commit (backend lint, nolint, frontend lint, landing lint, ts-suppressions, admin typecheck + lint, migrations-lint) — keep in sync with `lefthook.yml`. Self-scoping: the script silently allows unless the session repo shares a git common dir with the script's own checkout (worktrees included) — the user-level config fires hooks in every session, and this keeps other projects quiet.
- `PreToolUse(Bash)` (`bash-guard.mjs`) → destructive-command guard: recursive `rm` (incl. `-rf`, sudo/env/xargs wrappers, `$(…)` substitutions, `bash -c`), `git clean` without dry-run, `git reset --hard`, `git push --force`/`-f` (`--force-with-lease` allowed), `docker system|volume prune`, `docker compose down -v`. Best-effort shell segmentation — a safety net against common agent mistakes, not a shell parser. Repo-agnostic by design: it guards every project on the machine.

No `PostToolUse` lint per edit: observation-only event, ~2.7 s per edit — friction without a win. Wiring: Kimi Code — opt-in `[[hooks]]` stanzas from `tools/hooks/README.md` into `~/.kimi-code/config.toml`; ZCode — deferred until its plugin mechanism matures (project hooks are ignored by design today).

### knip — dead-code advisory report

knip (pinned, run via `npx` — nothing installed) reports unused files, exports and dependencies. Advisory by design, not a gate: `make knip` loops `apps/frontend`, `apps/admin`, and the `tools/*` packages standalone — findings never fail the run, the printed report is the signal; only an infrastructure failure exits non-zero. The CI `knip` job mirrors it with `continue-on-error: true`. Monorepo mode was rejected (requires a root `package.json` the repo deliberately doesn't have). `apps/landing` is out of scope — a small hand-maintained marketing one-pager where the dead-code signal doesn't pay for the config upkeep (map #888, ticket T1). Per-package `knip.json` configs are calibrated to real false positives only (entries invoked by make/harness config are declared as entry, never imported). Promoting knip to a blocking gate is a separate decision once these configs stabilize.

### goleak — leaked-goroutine check in TestMain

`go.uber.org/goleak` wires `goleak.VerifyTestMain(m)` into `TestMain` of the test binaries that run long-lived components: `internal/platform/scheduler` (the five platform workers share `runTickerLoop`) and `internal/identity/adapters/scheduler` (the login-codes/sessions/attempts cleaner). The http-server candidate was dropped: no test binary runs `ListenAndServe`. The "every goroutine has an owner" rule stays prose in `apps/backend/CODING_STANDARDS.md` — goleak is a runtime backstop for the two packages where leaks are most likely, not a repo-wide gate. The scheduler integration build carries the single exemption: the testcontainers reaper connection blocks on its termination signal for the whole process lifetime by design, ignored via `IgnoreTopFunction` — the function name is pinned to the testcontainers version, and an upgrade that renames it fails this gate loudly (update the string then). `testdb.Reset` excludes `schema_migrations` from its TRUNCATE list, pinned by `TestReset_PreservesMigrationJournal`, so consecutive runs against one external `TEST_DATABASE_URL` stay clean.

### Version pins — `make versions-sync` / `make versions-check`

The Makefile is the single source of truth for the language versions and the migrate-CLI pin (`GO_VERSION`, `NODE_VERSION`, `MIGRATE_VERSION`, top of the Makefile, next to the tool pins). `make versions-sync` stamps them into every file they own: `ci.yml` (node-version, the `golang:…-bookworm` test containers, the migrations-job migrate pin), the base images of the four `apps/*/Dockerfile`, the `go` directive of `go.work` + `apps/backend/go.mod`, and the root `.nvmrc` (Node stays major-only so LTS patches float). `perl -pi` instead of `sed -i`: BSD and GNU sed take incompatible in-place flags; the sync is idempotent. `make versions-check` re-runs the sync and compares `git hash-object` hashes before/after (the freshness-gate idiom); the CI `versions-freshness` job runs it on every PR. Pinning the Dockerfile Go tag to the full patch closed the CI↔prod drift. Bumping a version happens in the Makefile, never in a stamped file.

### Quality mode of the frontend/admin

The quality bar of both apps is decided and `in force` — the current rule lists, levels, and canonical spellings live in the apps' `CODING_STANDARDS.md` (frontend: Configuration gates, Type-checked core, Money and condition gates, Zero tolerance, Security contour; admin: Quality bar). This registry records the enforcement wiring and the decisions behind the gates:

- **Rejections with measured prices** are recorded against the bar's source issues (`explicit-function-return-type`, `exactOptionalPropertyTypes`, `noPropertyAccessFromIndexSignature`, stylistic presets, Biome, jsx-a11y in the admin, …) — see "Rejected" and the source decisions below; the react-hooks v7 compiler rules were already active via `eslint-config-next` before the waves and stay documented-not-activated.
- **Remediation policy:** fix-in-place only — zero rewrites; mechanical classes as cross-slice tickets, hot slices vertically, shared patterns consolidated into shared helpers.
- **Security contour** — four blocking lint gates (markdown secure by default, `NEXT_PUBLIC_*` via the `PUBLIC_ENV_ALLOWLIST`, web storage only in its two owners, dangerous browser APIs banned) + served security headers on all three apps (XCTO, XFO DENY, Referrer-Policy, Permissions-Policy, `poweredByHeader: false`, CSP step 1; the admin nginx conf carries the headers with its cache policies; the landing serves the same set from `SECURITY_HEADERS` in its `next.config.ts` (ADR 0063 retired its nginx.conf) and deliberately keeps no CSP, like its old nginx self; no CSP on the nginx app (admin) — step 1 is frontend-only per the spec). HSTS stays on Caddy (execution is an ops procedure outside the spec).
- **CSP step 2** runs as the stage Report-Only recon: with `CSP_REPORT_ONLY=true`, `proxy.ts` serves the future strict policy (nonce + `strict-dynamic`, built from the shared `CSP_BASE_DIRECTIVES` in `shared/lib/csp.ts`) as `Content-Security-Policy-Report-Only`; violations POST to `/api/csp-report` → stdout → Uptrace. Prod doesn't carry the flag and behaves byte-for-byte as before; the blocking flip stays deferred until the stage report data backs it. Recon findings live in `docs/research/2026-08-23-csp-step2-report-only.md`.
- **Dependency contour** — blocking and reproducible: (1) `npm-audit` blocks PRs that change any `package-lock.json` (paths filter computed from `merge-base` — advisory DBs update independently of code, so an unfiltered blocking audit would go red overnight); (2) every dependabot entry carries `assignees` (the CI/CD owner per CODEOWNERS); (3) both deploy workflows generate a CycloneDX SBOM of each release image (artifact `sbom-<service>`, 90-day retention); (4) the nightly semgrep scan runs `p/xss` next to `p/ci` — taint rules are MEDIUM/LOW confidence and nightly-only by decision.

### Suppression counter — zero-tolerance gate

Two tools scripts (own packages, contract tests on vitest via `make tools-test`) enforce zero in manual code: `make backend-nolint` — any `nolint` word in a Go comment under `apps/backend` (comments scanned in line and block form; string literals excluded; no whitelist — a genuine exclusion belongs in `.golangci.yml` with an ADR); `make ts-suppressions` — `eslint-disable*` comments, `@ts-ignore`/`@ts-expect-error`/`@ts-nocheck`, and explicit `any` in type position in the manual TS/JS of the three apps (comment/string-aware scanning, not grep; generated code, build artifacts, and tool-managed env declarations excluded by the script itself). Both wire into lefthook pre-commit (staged files), CI (full pass), and the Stop-gate. golangci's `nolintlint` stays in force alongside (validates the form of any directive that slips in); the ESLint presets validate what exists — these gates enforce that nothing exists to validate.

### Rules translated from prose to tools

Five packages in three waves by cost/win, plus closing the CI test gap. Translated rules get a one-line "enforced by X (config)" pointer in the relevant AGENTS.md — the prose compresses, it doesn't disappear.

- **depguard/forbidigo extensions** (in `.golangci.yml`): `no-orm` (deny `gorm.io/*`, `entgo.io/*`, `xorm.io/*` — module-wide, tests included), `no-minio` (deny `github.com/minio/*`), explicit `platform/openapi` and `platform/config` denies in `domain-clean`/`application-clean`, and a forbidigo pattern banning `uuid.New()`/`uuid.NewV4()`/`uuid.NewString()`/`uuid.NewRandom()`/`uuid.NewRandomFromReader()` at every layer (ADR 0019; `uuid.Must(uuid.NewV7())` is the panicking replacement). `testifylint` enabled; the "no testify" comment corrected to fact (testify is test-only — depguard `testify-test-only`).
- **Freshness gates for generated code**: `make backend-openapi-check` (pinned oapi-codegen → `generated.gen.go`), `make backend-sqlc-check` (pinned sqlc → `internal/platform/generated/postgres`, artifact list as a runtime shell glob), `make frontend-api-check` (openapi-typescript → `shared/api/generated.ts`) — content hashes via `git hash-object` compared before/after regeneration, so the gates work on a dirty tree; each with a CI freshness job that fails with a regenerate-and-commit instruction. `backend-tkassa-spec-check` and `attributes-check`/`categories-check` predate the pattern.
- **CI test gap**: `frontend-test` and `admin-test` (vitest suites) and `backend-integration` (`-tags=integration -race`, testcontainers docker-out-of-docker: socket mounted with the host docker gid, sibling PostgreSQL 18, `TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal` + host-gateway routing, ryuk disabled in CI for determinism — local runs keep ryuk on; if a local integration run fails with "unexpected container status", re-run with `TESTCONTAINERS_RYUK_DISABLED=true`).
- **ESLint into `apps/admin`**: minimal flat config, no presets — only the translated rules (`no-explicit-any`, generated-imports seam, raw-`fetch` boundary files, URL-literal bans) — plus, from the quality mode, the type-checked preset and react-hooks (see the admin CODING_STANDARDS).
- **FSD boundaries in `apps/frontend`**: `eslint-plugin-boundaries` — element types `app → widgets → features → entities → shared`, one `boundaries/dependencies` policy per layer (downward + same slice allowed, everything else disallowed), slice public APIs via `fileInternalPath: "!index.ts"`, `no-unknown-dependencies` for pattern-less local files; alias-import resolution backed by `import/no-unresolved` (type-only imports stay covered by tsc); `no-restricted-imports` — `@/shared/api/generated*` outside `shared/api/**` across all source files (`@heroui/styles` boundary снята вместе со сносом пакета, #901). Flat-config last-write-wins is the recurring trap: a files-scoped block replaces (not merges) base rules, so security/money selectors are restated in every overriding block (`SECURITY_RESTRICTED_SYNTAX` shared array).
- **Wave 2E — DTO ban in `widgets/**` and `app/**`**: a files-scoped block bans `@/shared/api/dto` there — entities/features/shared keep importing DTO legally in mappers/hooks; widgets consume camelCase entity models and commands.
- **Мутационный гейт `useMutation` в `apps/frontend`** (#1146): named-import `useMutation` из `@tanstack/react-query` разрешён только в `features/**/api/hooks.ts` (канон-хууки) и `shared/lib/hooks/use-guarded-mutation.ts` (единственная реализация) — `no-restricted-imports` с `importNames`, правило сразу на error (счётчик нарушений на момент ввода 0, факт-чек #1120). Паттерны rehype-raw/generated ре-декларированы в обоих блоках (база и widgets/app) по конвенции last-write-wins; allowlist-файлы остаются под базовым блоком; `shared/api/**` накрыт отдельным узким блоком с `paths` (базовый блок игнорирует зону generated-клиента — находка /code-review). Проза: секция «Mutations and buttons» в `apps/frontend/CODING_STANDARDS.md`.
- **Migration lint — the Squawk + domain-rules hybrid**: `make migrations-lint` runs squawk (pinned binary, lock-safety, config `apps/backend/.squawk.toml`) and `tools/migration-lint/domain-rules.mjs` (statement-aware parsing) over `db/migrations`. Domain rules: no `numeric/decimal/real/double/float` columns at all (money is BIGINT kopecks — the ban is total, ADR 0036), no `DEFAULT` on `id` (ADR 0019), and the timeout header (`migration-timeouts`, #1146): every up migration (>= 000108) opens with `SET lock_timeout = '1s';` then `SET statement_timeout = '5s';` — a statement_timeout value other than '5s' requires a same-line justification comment. Down migrations are skipped by both linters; historical point exclusions carry comments — no baseline file. `prefer-timestamptz` is covered by squawk for free.
- **Backend quality bar, stage 1** (curated golangci-lint set): new linters `godox`, `intrange`, `gocyclo` (30), `iface`, `dogsled`, `gochecksumtype`, `testableexamples`; `sloglint` (no-global, lowercased static msgs), gofumpt extra-rules, staticcheck all-minus-ST1000, revive `unhandled-error`, gocritic extensions; forbidigo exit policy and depguard `testify-test-only`. `dupl` threshold 200 (150 flips after the prod dupl clusters close). Stage-2 waves stay behind `# REMEDIATION:` markers in the config. Prose counterparts: `apps/backend/CODING_STANDARDS.md`.

### Stays prose

41 audited rules stay as prose in the four AGENTS.md files (workflow/skills/TDD/process, domain language, semantic invariants) — separate coding-standards documents are not introduced: AGENTS.md is the only file guaranteed to load into every harness session. Advisory-only: `knip` (dead-code) — promoting it to a blocking gate is a separate decision once its config stabilizes.

## Rejected

Evaluated and ruled out on the map — recorded so the question stays closed:

- `explicit-function-return-type` — noise on small utilities outweighs the win.
- Static money-int64 lint and money-formatting lint — unreliable without context; the backend money schema is covered by migration lint, and the frontend formatting side was later rehabilitated as the money gate with a modules allowlist.
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

None of the tooling decisions gets an ADR, by the "offer ADRs sparingly" rule (domain-modeling: hard to reverse + surprising without context + real trade-off — all three required). Every decision here is reversible in one PR (delete `lefthook.yml`, `tools/hooks/`, a config stanza), so the hard-to-reverse prong fails. The Serena adoption (mandatory `uv` + Python 3.13 prerequisite, dropping two MCP servers) is still one-PR reversible — the spike-gate exists precisely to keep rejection cheap. The "why" for each decision lives in this registry and the linked issues.

## Reproduce on a new machine

1. Base prerequisites: Go toolchain plus the `gopls` binary on PATH (`go install golang.org/x/tools/gopls@latest`) — Serena launches gopls from PATH for Go semantics and never installs it itself; Node/npx, Docker (integration tests; the pre-push hook runs the full `make test` and blocks without it).
2. Harness MCP config — Kimi Code: `~/.kimi-code/mcp.json`; ZCode: its MCP config. Servers per the table above (do not re-add gopls/jetbrains); personal API keys (context7, figma) go into the user config only.
3. Skills: nothing to install — `.agents/skills/` ships with the repo.
4. Serena (mandatory): install `uv`, then `uv tool install -p 3.13 'serena-agent==1.7.0'` (uv provides the managed Python 3.13); `.serena/project.yml` comes from the repo; add the `serena` MCP entry (`serena start-mcp-server --context ide --project <abs repo root>`) to each harness config. Desktop-launched harnesses don't inherit the shell PATH — set an explicit `env.PATH` on the serena entry covering `~/go/bin`, the Node installation, `~/.local/bin`, and Homebrew, or Go semantics fail with "gopls is not installed". ZCode note: headless CLI use needs a separate `zcode login` and an explicit `provider`+`model` section in `~/.zcode/cli/config.json`; the desktop app needs neither.
5. Git hooks: `make hooks-install` — installs the pinned lefthook binary when missing via `go install`, then `lefthook install`; idempotent, re-run after cloning.
6. Harness hooks (opt-in): copy the `[[hooks]]` stanzas from `tools/hooks/README.md` into `~/.kimi-code/config.toml`, substituting the absolute path to your checkout (Node ≥ 20.11; no install step — the scripts are dependency-free). ZCode wiring is deferred by decision #294.

## Source decisions

Map: [Wayfinder: агентская инфраструктура репозитория — инструменты, правила, гейты](https://github.com/devnumbers/arenda-platform/issues/289) — facts in `docs/research/2026-08-16-*.md`, decisions #290–#296 (Serena adoption + spike-gate, gate/hook set, prose→tools translation, pipeline status quo).

Second map: [Wayfinder: режим качества фронта и админки — бар, гейты, безопасность, ремедиация](https://github.com/devnumbers/arenda-platform/issues/326) — facts in `docs/research/2026-08-18-*.md`, decisions #330–#335 + spec #378 (the bar A/B/C + admin track, the security contour, the remediation plan, the packaging slice); each gate's landing ticket is linked from its rule in the apps' CODING_STANDARDS.md and the issue tracker.
