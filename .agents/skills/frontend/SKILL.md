---
name: frontend
description: Frontend rules of this repository — FSD architecture in Next.js 16 App Router, slice import boundaries, DTO/entity mapping, money and HeroUI v3 conventions. Use for all frontend work in apps/frontend.
---

# Frontend (this repository)

A thin orientation skill: the rules live in repo documents and ESLint gates; this skill points at them and at the primary sources. It replaces generic Next/React skills — they do not know FSD, and Next.js 16 breaks training-data assumptions.

## Read first (repo sources of truth)

1. `apps/frontend/AGENTS.md` — stack (Next.js 16.3.1, React 19.2.4, React Compiler), FSD architecture with enforced boundaries, API & data flow, quality gates. It also directs you to the bundled Next docs in `node_modules/next/dist/docs/` before writing code — Next 16 has breaking changes versus training data.
2. `apps/frontend/CODING_STANDARDS.md` — FSD slice anatomy, DTO/command mapping patterns, react-query conventions, forms, styling and React Compiler rules, testing patterns, review rubric.
3. `apps/frontend/eslint.config.mjs` — `boundaries/dependencies` (layer direction, public API via `index.ts`) and `no-restricted-imports` (generated client, DTO leaks); enforced, so review does not re-report them.
4. Research series (map #326): `docs/research/2026-08-18-frontend-benchmark.md`, `2026-08-18-eslint-ts-strictness.md`, `2026-08-18-frontend-security.md`, `2026-08-18-frontend-remediation-grid.md`.

## Sharpest rules agents get wrong

- Layers run `app/widgets → features → entities → shared`, inward only; no cross-slice imports inside a layer; a slice is consumed only through its `index.ts`, kept minimal.
- UI stays dumb; business logic lives in `features/` and `entities/`.
- Map backend DTOs to entity models at the `shared/api` boundary; `widgets/**` and `app/**` never import DTO types. Money arrives as integer kopecks and is formatted only via `formatMoneyKopecks` (`shared/lib/format-money.ts`).
- Server Components by default; `'use client'` only for interactivity, browser APIs, or hooks requiring a client boundary.
- HeroUI v3 is beta and absent from training data: verify components through the `heroui-react` MCP (`list_components`, `get_component_docs`) before writing code; never mix v2 APIs or `@heroui/styles` BEM classes into React components.
- Generated files (`shared/api/generated.ts`, attributes catalog) are never hand-edited — change the contract or catalog and regenerate.

## Primary sources (verify here, in this order)

1. Bundled Next.js docs: `node_modules/next/dist/docs/` — first stop for Next 16 behavior and deprecations.
2. Official docs: `https://nextjs.org/docs` and `https://react.dev`.
3. FSD methodology: `https://feature-sliced.design/` (Russian docs available).
4. Vercel conventions as reference: Next.js templates/learn material and the Vercel blog — reference only; this repo's FSD layering wins on any conflict.

Not for `apps/admin`: a different stack (react-admin 5 + MUI 7, no FSD, no App Router) — see `apps/admin/AGENTS.md`.
