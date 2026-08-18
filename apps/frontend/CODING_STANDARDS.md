# apps/frontend/CODING_STANDARDS.md

How the Next.js frontend is built and reviewed. Read before implementing or reviewing a frontend change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- Import boundaries, the DTO isolation gates, and the `@heroui/styles` ban are enforced by `eslint.config.mjs` (`boundaries/dependencies`, `no-restricted-imports`); review does not re-report what lint blocks.
- Invariants and commands: `AGENTS.md` (same directory). Domain language: per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`). Decisions: `docs/adr/`.
- HeroUI v3 component APIs: verify through the `heroui-react` MCP server — v3 is beta and not in model training data.

## FSD slice anatomy

Layers live at the app root (`app/`, `widgets/`, `features/`, `entities/`, `shared/`; alias `@/*`). What belongs where:

- **`features/<name>/` is thin by design**: `api/hooks.ts` (react-query hooks), optional `lib/` (pure helpers), optional `ui/` (components), `index.ts`. No `model/` in features — server state lives in react-query, local UI state in `useState`/URL.
- **`entities/<name>/`** holds the domain model mapped from the backend: `model/types.ts` (camelCase entity types + Command types), `model/mappers.ts` (DTO→entity), optional `lib/` (format helpers) and `ui/` (list/badge components).
- **`widgets/<name>/`** composes page blocks out of features, entities, and `shared/ui`.
- A slice's `index.ts` is its only import surface from other slices (enforced); keep it minimal — export only what other elements actually import.

Adding a new slice: user scenario → feature; domain model shared across scenarios → entity; page block → widget. If it fetches, its keys go into `shared/api/query-keys.ts`. Before adding any component, hook, or helper, search `shared/ui`, `shared/lib`, and existing slices (knip also guards dead exports).

## Data flow across the DTO boundary

- `shared/api/generated.ts` is generated and imported only inside `shared/api`; the rest of the app imports DTO types from `shared/api/dto.ts` (the re-export) or, better, not at all.
- DTO→entity mapping lives in `entities/<name>/model/mappers.ts`: snake_case → camelCase, `?? null` normalization, page-wrapper mapping (`items/limit/offset/has_more/next_offset`). Example: `entities/operation/model/mappers.ts`.
- Requests use camelCase **Command types** (`UserUpdateCommand` in `entities/user/model/types.ts`); mutations take the command and serialize inside the feature: when the endpoint is snake_case, add an explicit `toCreateWireRequest`/`toUpdateWireRequest` in the feature's `api/` (see `features/operations`), covered by a wire test.

## react-query conventions

- Hooks live in `features/<name>/api/hooks.ts`, named `useXxx`, typed like `UseMutationResult<Entity, ApiError, XxxCommand>`; API errors normalize to `ApiError` (`shared/api/errors.ts`).
- Cache keys live in the single registry `shared/api/query-keys.ts` — one `xxxKeys` factory per feature: `all: ['domain'] as const` plus parameterized keys composed from `all` (`detail(id)`, `byProperty(propertyId)`). The registry is shared because cross-feature invalidation must bypass the cross-slice import ban.
- Mutations invalidate in `onSuccess` through the registry (`invalidateQueries({ queryKey: xxxKeys.all })`); the profile pattern is `setQueryData` for the optimistic value plus `invalidateQueries`; deletes use `removeQueries`.
- `QueryClient` is configured once in `shared/providers/query-provider.tsx` (`staleTime: 30_000`, `refetchOnWindowFocus: false`) — don't override per-query without a stated reason.

## Forms

No form library and no schema validator — this is deliberate, not a gap:

- Controlled `useState` fields + `touched`/`submitAttempted` flags + derived validity + a derived `canSubmit`. Pattern: `widgets/profile/ui/PersonalDataForm.tsx`.
- Validation error strings are hardcoded Russian, inline next to the field.
- Property attributes validate through the generated validators (`features/property-attributes/lib/validate.ts` re-exports the generated catalog validators) — never hand-roll rules the catalog already encodes.

## Components, styling, and React Compiler

- Styling is CSS Modules + design tokens (`shared/styles/tokens.css`). Tailwind utilities are HeroUI's engine, not ours — no utility classes in app components.
- `shared/ui/` is the app's own kit: folder-per-component (`Button.tsx` + `Button.module.css` + `index.ts`), ~23 wrappers. Wrap, don't bypass; `/ui-kit` is the gallery route.
- HeroUI v3 is provider-less: import `@heroui/react` components directly. `@heroui/styles` appears exactly once — the `@import` in `app/globals.css` — and never in TSX.
- **React Compiler is on** (`next.config.ts`). Manual `useMemo`/`useCallback`/`memo` is not the default: write plain code and let the compiler memoize. Reach for manual memoization only where the compiler provably can't help (values escaping to non-React code) and justify it with a comment.

## Navigation and browser history

- Completing or cancelling a flow that returns to its source page (saving an edit form, "Cancel", "Add later" on a wizard success step, deleting an entity) — `goBack(router, fallbackHref)` from `shared/lib/navigation`: the form pops out of history and the source page below opens; with empty history it degrades to `router.replace(fallbackHref)`.
- Completing a flow that navigates to a new page (created entity, another section) — `router.replace`: the target replaces the transient history entry.
- `router.replace` back to the source page is banned: it duplicates the source page in history and the first Back press returns to the same URL (a "dead" back).
- Entering a flow ("Create" buttons, `handleEdit`) and ordinary content navigation — `router.push`.
- The Back button walks history (`goBack`), so completed flow pages must not remain in it — otherwise Back returns the user to an already-finished form.

## Testing

Unit tests are pure-logic only — vitest runs in a node environment with no DOM, no testing-library, no msw. This is the standard, not a gap:

- Colocated `*.test.ts` next to the module. Priority targets: wire serializers and DTO→entity mappers (shape assertions with `toStrictEqual`), pure `lib/` and `shared/lib` modules (navigation, pwa, formatting).
- Components and API calls are not unit-tested; UI behavior is covered by Playwright e2e.

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **Manual memoization** — `useMemo`/`useCallback`/`memo` without the escaping-values justification. → delete it; React Compiler owns memoization.
- **useState for derived state** — state recomputable from props/other state, or mirroring server data. → compute in render, or move to react-query.
- **useEffect synchronizer** — an effect chain updating local state to "keep in sync". → derive during render; effects are for external systems.
- **`key={index}`** on a list that can reorder. → key by stable entity id.
- **Naked data surface** — a new query render path with no loading state (inline `isLoading` + skeleton/`FinanceLoading`) and no error path. → both states ship with the feature.
- **Mutation without invalidation** — success leaves the cache stale. → `invalidateQueries` via the key registry in `onSuccess`.
- **Hand-rolled formatting** — `/100` money math or ad-hoc date strings. → `formatMoneyKopecks`, `@react-aria/i18n` (ru-RU) helpers.
- **Floating promise in a handler** — an async event handler with no error path. → `catch` → toast via `shared/lib/toast` / `ApiError`.
- **Synchronous `searchParams`** — reading the Next 16 promise directly. → `await` it in the server page and pass parsed initial props into the client widget.
