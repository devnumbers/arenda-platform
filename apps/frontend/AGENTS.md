# apps/frontend/AGENTS.md

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing any code. Heed deprecation notices.

## Scope

Rules for the Next.js frontend in `apps/frontend`. Also follow the root `AGENTS.md`, the relevant per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`), relevant product docs, and ADRs.

## Stack & Skills

- Versions live in `package.json` (single source); React Compiler is enabled in `next.config.ts`. Official Next.js docs (`https://nextjs.org/docs`) take precedence over training data; for non-obvious third-party behavior use `context7`.
- Skills: `frontend` for all frontend work (this repo's FSD architecture, slice boundaries, data-flow rules), `typescript` for type design.
- Before shipping a ticket that touched screens, forms, flows, or widgets, run `/ui-walkthrough` — live black-box acceptance in the visible browser against the seeded e2e stack (P0+P1 green gates the commit; see the skill). To watch already-written e2e specs play in visible windows, use the same skill's headed-run mode (`make frontend-e2e-headed`).

## Design Conventions

Before building or changing any screen, surface, or design-layer component, read `DESIGN.md` (same directory): breakpoints and widths, header/page anatomy, which surface to choose (route / fullscreen overlay / modal / picker menu), pickers and wheels, list states, icons, motion/hover/focus, date and money formatting, Figma-first workflow, and how owner design decisions get recorded. The live catalog of design-layer components is the `/ui-kit` route.

New UI (payments design layer onward, ADR 0050) is shadcn/ui over Radix primitives styled with Tailwind utilities on the tokens (`shared/styles/tokens.css`; Tailwind v4 wired in `app/globals.css`) — do not use HeroUI in new code.

Icons: use only the canonical set in `shared/assets/icons` (owner-approved SVGs from Figma — Bold/R 24×24 plus the S 16×16 style; mapping in `shared/assets/icons/README.md`). Never draw, inline, or generate icons yourself — a missing icon means exporting its node from Figma or asking the owner (DESIGN.md §10). Render small sizes with the same file, never create size-duplicate files.

Browser automation and UI verification go through the playwright MCP (`docs/agents/mcp.md`); check desktop and mobile layouts, visible interaction states, loading/error states, and that text does not overlap or overflow.

## TypeScript & Linting

- Keep `strict: true` and the strictest practical compiler options in `tsconfig.json`; use `eslint-config-next` (`core-web-vitals` + `typescript` presets) in `eslint.config.mjs`.
- The react-hooks v7 compiler rules, the zero-suppression counter, and the security contour are lint errors today, not review calls — rule lists, levels, and the canonical spellings live in `CODING_STANDARDS.md` (React Compiler, Quality bar, Security contour).
- Use explicit return types for functions exported from `shared/`, `entities/`, `features/`, and `widgets/`. Prefer `readonly`, `ReadonlyArray`, and `as const` for immutable data.

## Coding Standards

Before implementing or reviewing frontend code, read `CODING_STANDARDS.md` (same directory): FSD slice anatomy, the DTO/command mapping patterns, react-query conventions, forms, styling and React Compiler rules, navigation flow rules, the quality bar, testing patterns, and the review rubric used by the Standards axis of `/code-review`.

## Architecture (FSD)

- `app/` — Next.js App Router pages, layouts, loading/error boundaries, and route handlers. `pages/` only if an explicit ADR adds the Pages Router.
- `widgets/` — self-contained page blocks composed of features, entities, and shared UI. `features/` — user scenarios and use cases ("create property", "pay subscription").
- `entities/` — domain models mapped from the backend; the live list of slices is the `entities/` directory (anatomy in `CODING_STANDARDS.md`).
- `shared/` — reusable infrastructure: UI kit, API client, config, helpers, types, hooks not tied to a feature. Cross-entity model types (`shared/model/`) and the react-query key registry (`shared/api/query-keys.ts`) live here because cross-slice imports are banned above `shared`.
- Dependency direction is inward only: `app/widgets` → `features` → `entities` → `shared`; no imports upward or sideways between slices (enforced by `boundaries/dependencies` in `eslint.config.mjs`). Every slice is consumed only through its `index.ts` (`fileInternalPath: "!index.ts"` policy) — keep the index minimal. `shared/` modules are entry points themselves and are imported directly.
- Keep UI dumb; business logic lives in `features/` and `entities/`. Server calls live in `shared/api` or Next.js route handlers.
- Before adding components, hooks, helpers, entity types, feature state, or API wrappers, search existing FSD slices and call sites to avoid duplicate patterns.

## API & Data Flow

- Server-side data fetching (Server Components and route handlers) by default; browser-side state only when interactivity requires it. Do not use global state for local UI state; use URL state for shareable page state.
- Map backend DTOs to entity models at the API boundary; do not leak generated DTOs into widgets or features. `shared/api/generated.ts` is imported only inside `shared/api` — everywhere else import DTO types from `shared/api/dto`; `widgets/**` and `app/**` must not import `@/shared/api/dto` at all (enforced by `no-restricted-imports`). Mapping and wire-serialization patterns: `CODING_STANDARDS.md`.
- Money arrives as integer kopecks (`number`, safe below 2^53); every conversion goes through `shared/lib/format-money.ts` — raw `/100`/`*100` and `.toFixed` outside the formatting modules are lint errors (DESIGN.md §9, quality bar in `CODING_STANDARDS.md`).
- Reuse backend types from OpenAPI where possible; keep frontend entity types explicit and minimal. Do not edit generated files by hand: update the backend OpenAPI contract and run `cd apps/frontend && npm run generate:api` (`make frontend-api-check` guards freshness), `make attributes-gen` / `make categories-gen` for the `tools/*/catalog.json` artifacts.

## Quality Gates

Before claiming frontend work complete: `npm run lint`, `npm run build` (from `apps/frontend`), and `make frontend-test`. Tickets touching screens, forms, flows, or widgets also gate on `/ui-walkthrough` with **P0 + P1 green** before the commit (P2/P3 findings report as Issues without blocking).

E2E seeds and fixtures anchor dates to the start of the **UTC** day — `todayAt` in `e2e/fixtures.ts` is the canon, never a bare local `new Date()`. The owner's calendar is MSK, the Playwright browser is UTC; local wall-clock dates have shipped three TZ-flake families (#796, #805, the midnight-window seed of #704/#838) — each paid for with a red suite and a seed fix.
