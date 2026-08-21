# apps/frontend/AGENTS.md

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.

## Scope

Rules for the Next.js frontend in `apps/frontend`. Also follow the root `AGENTS.md`, the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), relevant product docs, and ADRs.

## Stack & References

- Next.js `16.3.1`, React `19.2.4`, TypeScript `^5`.
- React Compiler enabled in `next.config.ts`.
- Official Next.js docs (`https://nextjs.org/docs`) take precedence over training data.
- For non-obvious third-party behavior, use `context7` for current docs.

## Required Skills

- For all frontend work, invoke `frontend` — this repository's FSD architecture, slice boundaries, and data-flow rules, with pointers to primary sources (Next.js 16, React, FSD, Vercel conventions).
- For TypeScript questions and type design, invoke `typescript`.
- For current library docs before relying on non-obvious APIs, use `context7`.

## MCP Servers

- `playwright` — use for browser automation and UI verification when the `mcp__playwright__*` tools are available. Check desktop and mobile layouts, visible interaction states, loading/error states, and that text does not overlap or overflow. If they are unavailable, fall back to manual inspection, build logs, and native browser tools.
- `heroui-react` — mandatory documentation source for HeroUI v3: `@heroui/react` v3 is beta and not covered by model training data. Before writing HeroUI code, verify the component with `list_components`, then read `get_component_docs`; never mix v2 APIs or BEM classes from `@heroui/styles` into React components (enforced by `no-restricted-imports` in `eslint.config.mjs`).
- `lean-ctx` — use for broad exploration, large generated files, repeated reads, and noisy build or lint output. Code semantics (symbols, references, rename, diagnostics) is Serena's, not lean-ctx's. Before editing exact TypeScript, component, route, or config code, read the target source in raw/full form.
- `serena` — mandatory for TypeScript semantic work: symbol navigation, references, rename, diagnostics, and symbol-level editing (one root project covers `apps/frontend`, `apps/admin`, and the Go backend — `.serena/project.yml`). If the `mcp__serena__*` tools are not available, stop and tell the user (they can inspect `/mcp`) — no silent substitution with text search. Serena is not a replacement for `npm run lint`, `npm run build`, or direct code review.

Before adding components, hooks, helpers, entity types, feature state, or API wrappers, search existing FSD slices and call sites with `Grep`/`lean-ctx` to avoid duplicate patterns.

## TypeScript & Linting

- Keep `strict: true` and the strictest practical compiler options in `tsconfig.json`.
- Use `eslint-config-next` (`core-web-vitals` + `typescript` presets) in `eslint.config.mjs`.
- The react-hooks v7 compiler rules are already active transitively through `eslint-config-next` 16.3.1 (the plugin's `recommended` preset is spread whole) — manual-memoization, effect-synchronizer, and derived-state smells are lint errors today, not review-only calls. Rule list and levels: `CODING_STANDARDS.md`, React Compiler section (`exhaustive-deps` at error — quality bar wave A).
- Zero tolerance for suppressions in manual code: `eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` are forbidden — fix the code, never suppress (quality bar, spec #378; a blocking CI counter is the accepted gate). Prefer `unknown` with narrowing. Generated code (`shared/api/generated.ts`, `features/property-attributes/lib/generated/`) and build artifacts are outside the counter's scope.
- Security contour gates at error level (decision #331, enforced by `eslint.config.mjs`): react-markdown stays secure by default — no `rehype-raw` import, no `urlTransform` prop; `NEXT_PUBLIC_*` reads require adding the variable to `PUBLIC_ENV_ALLOWLIST` in the config; `localStorage`/`sessionStorage` live only in `features/auth/lib/**` and `shared/lib/hooks/useDraftStore` (tokens are httpOnly cookies, never web storage); the XSS-class browser APIs (`dangerouslySetInnerHTML`, `innerHTML`, `insertAdjacentHTML`, `document.write`/`writeln`, `eval`, `new Function`, incl. `window.`/`globalThis.` alias forms) are banned outright — quality bar wave A.
- Use explicit return types for functions exported from `shared/`, `entities/`, `features/`, and `widgets/`.
- Prefer `readonly`, `ReadonlyArray`, and `as const` for immutable data.

## Coding Standards

Before implementing or reviewing frontend code, read `CODING_STANDARDS.md` (same directory): FSD slice anatomy, the DTO/command mapping patterns, react-query conventions, forms, styling and React Compiler rules, navigation flow rules, the accepted quality bar (waves A/B/C), testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture (FSD)

- `app/` — Next.js App Router pages, layouts, loading/error boundaries, and route handlers.
- `pages/` — only if an explicit ADR adds the Pages Router; the default is App Router.
- `widgets/` — self-contained page blocks composed of features, entities, and shared UI.
- `features/` — user scenarios and use cases (for example: "create operation", "pay subscription").
- `entities/` — domain models mapped from the backend: owner, property, lease, operation, subscription.
- `shared/` — reusable infrastructure: UI kit, API client, config, helpers, types, and hooks not tied to a specific feature. Cross-entity model types referenced by several entity slices (`shared/model/`) and the react-query key registry (`shared/api/query-keys.ts`) live here because cross-slice imports are banned above `shared`.
- Dependency direction is inward only: `app/widgets` → `features` → `entities` → `shared`. No imports upward or sideways between slices — cross-slice imports inside a layer are banned too (enforced by `boundaries/dependencies` in `eslint.config.mjs`, `eslint-plugin-boundaries`).
- Every slice in `widgets/`, `features/`, and `entities/` is consumed from outside only through its public API — the slice's `index.ts` (enforced by the `fileInternalPath: "!index.ts"` policy of `boundaries/dependencies` in `eslint.config.mjs`). Keep the index minimal: export only what other elements actually import. `shared/` modules are entry points themselves and are imported directly.
- Keep UI dumb; business logic lives in `features/` and `entities/`. Server calls live in `shared/api` or Next.js route handlers.

## API & Data Flow

- Use Next.js server-side data fetching (Server Components and route handlers) by default.
- Browser-side state only when interactivity requires it.
- Map backend DTOs to entity models at the API boundary; do not leak generated DTOs into widgets or features. The generated client `shared/api/generated.ts` is imported only inside `shared/api` — everywhere else import DTO types from `shared/api/dto` (enforced by `no-restricted-imports` in `eslint.config.mjs`). `widgets/**` and `app/**` must not import `@/shared/api/dto` at all: widgets consume camelCase entity models and pass camelCase request commands; the mapping and wire-serialization patterns live in `CODING_STANDARDS.md`.
- Money arrives from the API as integer kopecks (`number`, safe below 2^53); format for display only via `formatMoneyKopecks` from `shared/lib/format-money.ts` — never hand-roll `/100` formatting.
- Reuse backend types from OpenAPI where possible; keep frontend entity types explicit and minimal.
- Do not edit generated API client files by hand; update the backend OpenAPI contract and regenerate the frontend client (freshness enforced by `make frontend-api-check` in `.github/workflows/ci.yml`; regenerate with `cd apps/frontend && npm run generate:api`).
- The property attributes catalog (`features/property-attributes/lib/generated/`) is generated from `tools/property-attributes/catalog.json`. Regenerate with `make attributes-gen` (or `cd tools/property-attributes && npm run generate`); the gate `make attributes-check` fails in CI if a `catalog.json` change was not committed with its regenerated artifacts. Do not hand-edit `generated/`.

## Components & State

- Server Components by default; use `'use client'` only for interactivity, browser APIs, or hooks that require a client boundary.
- Keep components small, explicit, and focused on one responsibility.
- Avoid prop drilling; prefer composition and, when necessary, feature-scoped context.
- Do not use global state for local UI state. Use URL state for shareable page state.
- Prefer explicit event handlers and reducers over implicit side effects.

## Quality Gates

- Run before claiming frontend work complete:

```bash
cd apps/frontend && npm run lint
cd apps/frontend && npm run build
make frontend-test
```

CI backstop: the `frontend-test` job in `.github/workflows/ci.yml` runs this
vitest suite on every PR.

## Commands

```bash
# from the repository root
make frontend-test

# from apps/frontend
npm install
npm run dev
npm run build
npm run lint
npm run test
```
