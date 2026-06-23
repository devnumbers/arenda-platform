# Design: `apps/frontend/AGENTS.md`

## Goal

Provide a concise, authoritative rules document for the Next.js frontend (`apps/frontend`) so that agents and developers follow the same clean-code, typed, layered approach already established by `apps/backend/AGENTS.md`.

## Context

- `apps/frontend` runs Next.js `16.2.9` with React `19.2.4`, TypeScript `^5`, and `eslint-config-next`.
- `tsconfig.json` already enables `strict: true`.
- `next.config.ts` enables the React Compiler.
- The existing `apps/frontend/AGENTS.md` is only a short Next.js warning.
- Root `AGENTS.md` and `CONTEXT.md` already define repository-level workflow and domain glossary.

## Decisions

1. **Depth**: brief rule summary, like `apps/backend/AGENTS.md`, without code examples.
2. **Architecture style**: Feature-Sliced Design (FSD) — `app`, `pages` (only with ADR), `widgets`, `features`, `entities`, `shared`.
3. **Relationship to existing files**: rewrite `apps/frontend/AGENTS.md` completely; preserve the Next.js warning as a short note.

## Proposed Structure

```
# apps/frontend/AGENTS.md

## Scope
Rules for the Next.js frontend in `apps/frontend`. Also follow the root `AGENTS.md`, `CONTEXT.md`, relevant product docs, and ADRs.

## Stack & References
- Next.js 16.2.9, React 19.2.4, TypeScript ^5.
- React Compiler enabled in `next.config.ts`.
- Official Next.js docs (`nextjs.org/docs`) take precedence over training data.
- For non-obvious third-party behavior, use Context7 for current docs.

## Required Skills
- For all frontend work: `$next-best-practices`, `$vercel-react-best-practices`.
- For component composition / design patterns: `$vercel-composition-patterns`.
- For view transitions: `$vercel-react-view-transitions`.
- For TypeScript: `$typescript`.
- For current library docs: use Context7 (`mcp:context7`) before relying on APIs that are not obvious.

## TypeScript & Linting
- Keep `strict: true` and the strictest practical compiler options.
- Use `eslint-config-next` (`core-web-vitals` + `typescript` presets).
- No `any` without a comment/ADR; prefer `unknown` + narrowing.
- Explicit return types for public module boundaries.
- Prefer `readonly` / `ReadonlyArray` / `as const` for immutable data.

## Architecture (FSD)
- `app/` — Next.js App Router pages, layouts, loading/error boundaries, route handlers.
- `pages/` — only if an explicit ADR adds the Pages Router; default is App Router.
- `widgets/` — self-contained page blocks composed of features/entities/ui.
- `features/` — user scenarios and use cases (e.g., "create operation", "pay subscription").
- `entities/` — domain models mapped from the backend (owner, property, lease, operation, subscription).
- `shared/` — reusable infrastructure: UI kit, API client, config, helpers, types, hooks not tied to a feature.
- Direction of dependencies: `app/widgets` → `features` → `entities` → `shared`. No imports upward or sideways between slices.
- Keep UI dumb; business logic lives in `features/`/`entities/`. Server calls live in `shared/api` or route handlers.

## API & Data Flow
- Use Next.js server-side data fetching (Server Components + route handlers) by default.
- Browser-side state only when interactivity requires it.
- Map backend DTOs to entity models at the API boundary; do not leak generated DTOs into widgets/features.
- Reuse backend types from OpenAPI where possible; keep frontend entity types explicit and minimal.

## Components & State
- Server Components by default; `'use client'` only for interactivity, browser APIs, or hooks that require a client boundary.
- Keep components small, explicit, and focused on one responsibility.
- Avoid prop drilling; use composition and, when necessary, feature-scoped context.
- No global state for local UI state. Use URL state for shareable page state.
- Prefer explicit event handlers and reducers over implicit side effects.

## Testing & Quality Gates
- Add tests for features and entities that contain business logic.
- Run before claiming work complete:
  ```bash
  npm run lint
  npm run build
  ```
- Keep tests deterministic; mock network and browser APIs at `shared/api` boundaries.

## Commands
```bash
npm install          # from apps/frontend
npm run dev
npm run build
npm run lint
```
```

## Implementation Notes

- Replace the current `apps/frontend/AGENTS.md` with the document above.
- Add a brief entry to `CHANGELOG.md` under the current date.
- Verify with `npm run lint` and `npm run build` in `apps/frontend`.
