# apps/admin/AGENTS.md

## Scope

Rules for the react-admin back-office SPA in `apps/admin`. Also follow the root `AGENTS.md`, the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), relevant product docs, and ADRs.

## Stack & References

- Vite 6, React 19, TypeScript `^5.7`, react-admin 5 (`react-admin`, `ra-i18n-polyglot`, `ra-language-russian`), MUI 7 with Emotion.
- Entry point `src/main.tsx`; root component `src/App.tsx`; backend integration lives in `src/dataProvider.ts`; authentication in `src/authProvider.ts`; login UI in `src/LoginPage.tsx`; resources are declared per domain (for example `src/leases.tsx`, `src/operations.tsx`).
- Russian UI localization via `ra-i18n-polyglot` + `ra-language-russian`; keep new user-facing strings localized.
- Official react-admin docs (https://marmelab.com/react-admin/documentation.html) and MUI docs (https://mui.com/material-ui/) take precedence over training data.
- For non-obvious third-party behavior, use `context7` for current docs.

## Required Skills

- For TypeScript questions and type design, invoke `typescript`.
- For current library docs before relying on non-obvious APIs, use `context7`.

## MCP Servers

- `playwright` — use for browser automation and UI verification when the `mcp__playwright__*` tools are available. Check desktop and mobile layouts, visible interaction states, loading/error states, and that text does not overlap or overflow. If unavailable, fall back to manual inspection and build logs.
- `lean-ctx` — use for broad exploration, large generated files, repeated reads, and noisy build or typecheck output. Code semantics (symbols, references, rename, diagnostics) is Serena's, not lean-ctx's. Before editing exact TypeScript, component, or config code, read the target source in raw/full form.
- `serena` — mandatory for TypeScript semantic work: symbol navigation, references, rename, diagnostics, and symbol-level editing (one root project covers `apps/admin`, `apps/frontend`, and the Go backend — `.serena/project.yml`). If the `mcp__serena__*` tools are not available, stop and tell the user (they can inspect `/mcp`) — no silent substitution with text search. Serena is not a replacement for `npm run lint`, `npm run typecheck`, or direct code review.

Before adding resources, fields, inputs, helpers, or API wrappers, search existing resources and call sites with `Grep`/`lean-ctx` to avoid duplicate patterns.

## Coding Standards

Before implementing or reviewing admin code, read `CODING_STANDARDS.md` (same directory): resource file anatomy, the read-only design and its dataProvider conventions, `fields.tsx` choice-catalog sync rules, i18n practice, the accepted quality-bar track (map #326), testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture

- This is a standalone Vite SPA, not Next.js: no App Router, no Server Components, no file-based routing. Do not apply the Next.js rules from `apps/frontend/AGENTS.md` here.
- Follow react-admin conventions: declare resources on the `<Admin>` component, keep list/edit/create/show views colocated per resource, and route all backend calls through `dataProvider` and all auth state through `authProvider`. Enforced by ESLint `no-restricted-globals` on `fetch` (`apps/admin/eslint.config.mjs`): raw `fetch` is allowed only in the sanctioned HTTP boundary files — `src/dataProvider.ts`, `src/authProvider.ts`, `src/lib/report-error.ts` (client-error telemetry, fire-and-forget).
- The quality bar is tool-enforced (admin tracks I–II, bar #330, spec #378): `typescript-eslint` `recommendedTypeChecked` (plus `prefer-nullish-coalescing`) and `eslint-plugin-react-hooks` `recommended` run at error level over all manual code — the HTTP boundary files included since track II (#392): their client wrappers hand out response JSON as `unknown`, target types and primitive narrowing live at the edge, and react-admin's `any`-typed params are anchored (param generics `GetOneParams<T>`/`GetManyParams<T>`, `Record<string, unknown>` filters); unsupported provider methods return rejected promises. Custom provider methods are reached via `useDataProvider<AdminDataProvider>()`. Async handlers in void-signature props (`onClick`, `onSubmit`) go through `void`-wrappers — the handlers themselves catch everything.
- Keep components small and explicit; prefer react-admin and MUI building blocks over custom widgets.
- Map backend DTOs at the `dataProvider` boundary; do not leak API response shapes into resource components.
- Money values are integer kopecks; display them via `MoneyField` / `formatKopecks` from `src/fields.tsx` (ru-RU, RUB), never ad-hoc formatting. Enforced by money selectors in `apps/admin/eslint.config.mjs`: raw `/100`/`*100` arithmetic and `.toFixed` are banned outside `src/fields.tsx` (generated code excepted — it is not fixed by hand).
- The property attributes catalog (`src/lib/generated/`) is generated from `tools/property-attributes/catalog.json`. Regenerate with `make attributes-gen` (or `cd tools/property-attributes && npm run generate`); the gate `make attributes-check` fails in CI if a `catalog.json` change was not committed with its regenerated artifacts. Do not hand-edit `generated/`. Imports of `generated/` are enforced by ESLint `no-restricted-imports` (`apps/admin/eslint.config.mjs`): only the `src/lib/propertyAttributes.ts` seam may import them.
- Runtime configuration comes from Vite env vars (see `.env.example`); never hardcode backend URLs or secrets. Hardcoded URL literals are enforced by ESLint `no-restricted-syntax` (`apps/admin/eslint.config.mjs`): no absolute `http(s)://` literals and no `/api/...` path literals — request URLs are built from `import.meta.env.VITE_API_PREFIX`.

## TypeScript

- Keep `strict: true` and the existing strict compiler options in `tsconfig.json` — including `noUncheckedIndexedAccess` and `verbatimModuleSyntax` (quality bar, spec #378): indexed access carries `| undefined`, and type-only imports are spelled `import type` / `type` specifiers. `npm run typecheck` (`tsc --noEmit`) must stay clean.
- Zero tolerance for suppressions in manual code (`src` outside `src/lib/generated/`): `eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` are forbidden — fix the code, never suppress (quality bar, spec #378; a blocking CI counter is the accepted gate). Explicit `any` is already a gate: `@typescript-eslint/no-explicit-any: error` (`apps/admin/eslint.config.mjs`, `npm run lint`). Prefer `unknown` with narrowing.

## Quality Gates

- Run before claiming admin work complete:

```bash
make admin-typecheck
npm --prefix apps/admin run lint
make admin-build
make admin-test
```

CI backstop: the `admin` job in `.github/workflows/ci.yml` runs lint
(ESLint boundary gates), typecheck and build on every PR; the `admin-test`
job runs this vitest suite.

## Commands

```bash
# from the repository root
make admin-install
make admin-dev
make admin-build
make admin-typecheck
npm --prefix apps/admin run lint
make admin-test
```
