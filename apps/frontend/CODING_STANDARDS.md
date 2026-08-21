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
- Persisted form drafts (survive a refresh mid-flow) go through the shared draft store — `shared/lib/hooks/useDraftStore` (`useSyncExternalStore` with `getServerSnapshot`, so the draft loads after hydration with no setState-in-effect). The slice's `use-*-draft.ts` wrapper owns only the storage key, the default, `validate`, and the terminal-step predicate; never hand-roll the sessionStorage load/persist/clear cycle.

## Components, styling, and React Compiler

- Styling is CSS Modules + design tokens (`shared/styles/tokens.css`). Tailwind utilities are HeroUI's engine, not ours — no utility classes in app components.
- `shared/ui/` is the app's own kit: folder-per-component (`Button.tsx` + `Button.module.css` + `index.ts`), ~23 wrappers. Wrap, don't bypass; `/ui-kit` is the gallery route.
- HeroUI v3 is provider-less: import `@heroui/react` components directly. `@heroui/styles` appears exactly once — the `@import` in `app/globals.css` — and never in TSX.
- **React Compiler is on** (`next.config.ts`). Manual `useMemo`/`useCallback`/`memo` is not the default: write plain code and let the compiler memoize. Reach for manual memoization only where the compiler provably can't help (values escaping to non-React code) and justify it with a comment.
- The memoization, derived-state, and effect-synchronizer smells are enforced by the tool, not the review rubric: the react-hooks v7 compiler rules already run through `eslint-config-next` 16.3.1 (the plugin's `recommended` preset is spread whole) — `purity`, `set-state-in-effect`, `set-state-in-render`, `use-memo`, `immutability`, `refs`, `preserve-manual-memoization`, `static-components`, `globals`, `error-boundaries`, `gating` at error; `rules-of-hooks` at error; `incompatible-library`/`unsupported-syntax` at warn; `exhaustive-deps` at warn until bar wave A flips it. Fix them at lint time.

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

## Quality bar — accepted waves (map #326, spec #378)

The quality bar is decided (bar [#330](https://github.com/devnumbers/arenda-platform/issues/330), spec [#378](https://github.com/devnumbers/arenda-platform/issues/378)) and lands ticket by ticket — the patterns below are the standard now; write to them without waiting for the flips. Flip conventions: wave A lands flip-with-fix (config flips together with its code fixes); waves B/C land fix-then-flip per rule family — a family's advisory counter drops to zero, then the rule flips to error, and this section plus `docs/agents/tooling.md` update in the same change. Already active today, ahead of the waves: the react-hooks v7 compiler rules (see the React Compiler section above).

### Wave A — configuration

Enforced by ESLint, `tsconfig.json`, and `next.config.ts` once landed:

- `react-hooks/exhaustive-deps` at error; `switch-exhaustiveness-check`; `consistent-type-imports`.
- Dangerous browser APIs banned via restricted rules — `dangerouslySetInnerHTML`, `innerHTML`, `insertAdjacentHTML`, `document.write`/`writeln`, `eval`, `new Function` — an XSS-class regression becomes mechanically impossible.
- Ambient `*.svg` module declarations in the shared layer — the one real typing gap behind icon-import noise, not strict-lint noise.
- tsconfig: `verbatimModuleSyntax`, `noImplicitOverride`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`; `reactStrictMode: true` explicit in `next.config.ts`.

### Wave B — type-checked core, rule by rule

A type-checked ESLint block (project service) with named rules, not a preset:

- `no-floating-promises` — `void`-prefix on fire-and-forget calls like `invalidateQueries`; real `catch` paths in handlers.
- `no-misused-promises` — async handlers in void signatures (`onPress`/`onClick`) get catch-wrappers.
- `no-unnecessary-type-assertion`, `no-non-null-assertion`, `no-base-to-string`, `require-await`, `no-deprecated`.
- `no-confusing-void-expression` (`ignoreArrowShorthand`) and `restrict-template-expressions` (`allowNumber`) — React-friendly options tuned in the config, not by suppressions.
- jsx-a11y `recommended` — keyboard support on custom controls, valid anchors; the OTP input's `no-autofocus` is solved with programmatic focus, not a rule exception.
- tsconfig `noUncheckedIndexedAccess`.

### Wave C

- The money gate, two steps: (1) code prep — kopecks input/output helpers in `shared/lib/format-money.ts` (no currency symbol, no digit grouping — semantics differ from the display helper) plus an explicit percent helper for the percent-homonym call sites; (2) selectors banning `/100` and `*100` arithmetic and `.toFixed` outside the formatting-modules allowlist (the money module and the area formatter). After step 1 the selectors hit zero false positives by construction.
- `no-unnecessary-condition`; `prefer-nullish-coalescing`.

### Zero tolerance for suppressions

`eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` — zero in manual code: fix the code, never suppress; a blocking CI counter guards the zero (accepted gate, `docs/agents/tooling.md`). The 15 existing frontend suppressions are being fixed in code during waves A/B. Generated code and build artifacts are outside the counter's scope.

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **`key={index}`** on a list that can reorder. → key by stable entity id.
- **Naked data surface** — a new query render path with no loading state (inline `isLoading` + skeleton/`FinanceLoading`) and no error path. → both states ship with the feature.
- **Mutation without invalidation** — success leaves the cache stale. → `invalidateQueries` via the key registry in `onSuccess`.
- **Hand-rolled formatting** — `/100` money math or ad-hoc date strings. → `formatMoneyKopecks`, `@react-aria/i18n` (ru-RU) helpers. (Becomes the money-gate selectors in wave C.)
- **Floating promise in a handler** — an async event handler with no error path. → `catch` → toast via `shared/lib/toast` / `ApiError`. (Becomes the `no-floating-promises` gate in wave B.)
- **Synchronous `searchParams`** — reading the Next 16 promise directly. → `await` it in the server page and pass parsed initial props into the client widget.
