# apps/frontend/CODING_STANDARDS.md

How the Next.js frontend is built and reviewed. Read before implementing or reviewing a frontend change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- Import boundaries and the DTO isolation gates are enforced by `eslint.config.mjs` (`boundaries/dependencies`, `no-restricted-imports`); review does not re-report what lint blocks.
- The security lint gates are enforced by `eslint.config.mjs` (see the Security contour section below).
- Invariants and commands: `AGENTS.md` (same directory). Domain language: per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`). Decisions: `docs/adr/`.
- New UI (payments design layer onward) is built on shadcn/ui over Radix primitives, styled with Tailwind utilities on top of our tokens (ADR 0050). Легаси HeroUI снесён целиком (аменд ADR 0050 2026-09-26, тикет #901) — нового HeroUI-кода не бывает, его MCP-доки сняты.

## FSD slice anatomy

Layers live at the app root (`app/`, `widgets/`, `features/`, `entities/`, `shared/`; alias `@/*`). What belongs where:

- **`features/<name>/` is thin by design**: `api/hooks.ts` (react-query hooks), optional `lib/` (pure helpers), optional `ui/` (components), `index.ts`. No `model/` in features — server state lives in react-query, local UI state in `useState`/URL.
- **`entities/<name>/`** holds the domain model mapped from the backend: `model/types.ts` (camelCase entity types + Command types), `model/mappers.ts` (DTO→entity), optional `lib/` (format helpers) and `ui/` (list/badge components).
- **`widgets/<name>/`** composes page blocks out of features, entities, and `shared/ui`.
- A slice's `index.ts` is its only import surface from other slices (enforced); keep it minimal — export only what other elements actually import.

Adding a new slice: user scenario → feature; domain model shared across scenarios → entity; page block → widget. If it fetches, its keys go into `shared/api/query-keys.ts`. Before adding any component, hook, or helper, search `shared/ui`, `shared/lib`, and existing slices (knip also guards dead exports). The one deliberate exception is `features/popups/**` (ignored in `knip.json`): the popup mechanism is spec-mandated infrastructure (ADR 0023) kept alive with no consumers until the next onboarding popup ships.

## Data flow across the DTO boundary

- `shared/api/generated.ts` is generated and imported only inside `shared/api`; the rest of the app imports DTO types from `shared/api/dto.ts` (the re-export) or, better, not at all.
- DTO→entity mapping lives in `entities/<name>/model/mappers.ts`: snake_case → camelCase, `?? null` normalization, and page-wrapper mapping (`items/limit/offset/has_more/next_offset`) for paginated endpoints. Example: `entities/user/model/mappers.ts`.
- Requests use camelCase **Command types** (`UserUpdateCommand` in `entities/user/model/types.ts`); mutations take the command and serialize inside the feature: when the endpoint is snake_case, add an explicit `toCreateWireRequest`/`toUpdateWireRequest` in the feature's `api/`, covered by a wire test.

## react-query conventions

- Hooks live in `features/<name>/api/hooks.ts`, named `useXxx`, typed like `UseMutationResult<Entity, ApiError, XxxCommand>`; API errors normalize to `ApiError` (`shared/api/errors.ts`).
- Cache keys live in the single registry `shared/api/query-keys.ts` — one `xxxKeys` factory per feature: `all: ['domain'] as const` plus parameterized keys composed from `all` (`detail(id)`, `byProperty(propertyId)`). The registry is shared because cross-feature invalidation must bypass the cross-slice import ban.
- Mutations invalidate in `onSuccess` through the registry (`void invalidateQueries({ queryKey: xxxKeys.all })` — the fire-and-forget spelling, see the `no-floating-promises` gate); the profile pattern is `setQueryData` for the optimistic value plus `invalidateQueries`; deletes use `removeQueries`, which is synchronous in v5 and stays bare.
- `QueryClient` is configured once in `shared/providers/query-provider.tsx` (`staleTime: 30_000`, `refetchOnWindowFocus: false`) — don't override per-query without a stated reason.

## Forms

No form library and no schema validator — this is deliberate, not a gap:

- Button-submit forms: controlled `useState` fields + `touched`/`submitAttempted` flags + derived validity + a derived `canSubmit`. Pattern: `widgets/properties/ui/PropertyEditForm.tsx`.
- Quiet-autosave forms (screen renders no save button; each field commits on blur/clear as a one-field diff): commit via a pure per-field patch helper and toasts for saved/error. An empty string clears the field — the backend PATCH contract treats a missing/`null` field as "leave unchanged". Pattern: `widgets/profile/ui/AccountScreen.tsx` (+ `widgets/profile/lib/profile-edit.ts`).
- Validation error strings are hardcoded Russian, inline next to the field.
- Property attributes validate through the generated validators (`features/property-attributes/lib/validate.ts` re-exports the generated catalog validators) — never hand-roll rules the catalog already encodes.
- Persisted form drafts (survive a refresh mid-flow) go through the shared draft store — `shared/lib/hooks/useDraftStore` (`useSyncExternalStore` with `getServerSnapshot`, so the draft loads after hydration with no setState-in-effect). The slice's `use-*-draft.ts` wrapper owns only the storage key, the default, `validate`, and the terminal-step predicate; never hand-roll the sessionStorage load/persist/clear cycle — web storage outside `features/auth/lib/**` and `useDraftStore` is a lint error.

## Components, styling, and React Compiler

- Styling: existing components use CSS Modules + design tokens (`shared/styles/tokens.css`); new design-layer components (ADR 0050) are shadcn/ui over Radix, styled with Tailwind utilities on the same tokens. The mix of design-layer Tailwind utilities and local CSS modules — the canon outside `design/` and in screen modules of `widgets`/`features`/`app` — is the target state as of the ADR 0050 amendment (2026-09-26, #901); no separate migration effort is recorded.
- Motion, hover, and focus conventions (the unified Apple curve, touch-safe hover, no focus rings on inputs) live in `DESIGN.md` §8 — read it before styling anything.
- `shared/ui/` is the app's own kit; its canon is `shared/ui/design/` — components on Tailwind utilities over the tokens, no CSS modules. Folder-per-component with a CSS module (`PullToRefresh.tsx` + `PullToRefresh.module.css` + `index.ts`) remains the canon outside `design/` (live-value, pull-to-refresh, toast) and in screen modules. Wrap, don't bypass; `/ui-kit` is the gallery route.
- ~~HeroUI v3 (legacy widgets only, ADR 0050)~~ — снято: пакет снесён, react- и стилевой слои в дереве отсутствуют (аменд ADR 0050 2026-09-26, тикет #901).
- **React Compiler is on** (`next.config.ts`). Manual `useMemo`/`useCallback`/`memo` is not the default: write plain code and let the compiler memoize. Reach for manual memoization only where the compiler provably can't help (values escaping to non-React code) and justify it with a comment.
- The memoization, derived-state, and effect-synchronizer smells are enforced by the tool, not the review rubric: the react-hooks v7 compiler rules run through `eslint-config-next` (the plugin's `recommended` preset is spread whole) — `purity`, `set-state-in-effect`, `set-state-in-render`, `use-memo`, `immutability`, `refs`, `preserve-manual-memoization`, `static-components`, `globals`, `error-boundaries`, `gating`, `rules-of-hooks`, `exhaustive-deps` at error; `incompatible-library`/`unsupported-syntax` at warn. Fix them at lint time.

## Navigation and browser history

- Completing or cancelling a flow that returns to its source page (saving an edit form, "Cancel", "Add later" on a wizard success step, deleting an entity) — `goBack(router, fallbackHref)` from `shared/lib/navigation`: the form pops out of history and the source page below opens; with empty history it degrades to `router.replace(fallbackHref)`.
- Completing a flow that navigates to a new page (created entity, another section) — `router.replace`: the target replaces the transient history entry.
- `router.replace` back to the source page is banned: it duplicates the source page in history and the first Back press returns to the same URL (a "dead" back). Exception (#1079, owner 02.10): the rental-create wizard's success exits («Хорошо», header cross) use `router.replace(ROUTES.property(propertyId))` — the success landing must be the property card regardless of the entry, and one real entry keeps the property-creation wizard (`/properties/new`) in history under the wizard, where `goBack` reopened that finished creation form instead of the card. The dead back and the leftover `/properties/new` under the card are the accepted price; the steps' chrome (crosses, step-1 Back) keeps `goBack`.
- Entering a flow ("Create" buttons, `handleEdit`) and ordinary content navigation — `router.push`.
- The Back button walks history (`goBack`), so completed flow pages must not remain in it — otherwise Back returns the user to an already-finished form.

## Screen shell

Every page assembles its chrome from the design-layer primitives — `<TopNav>`, `<PageContent>`, `<StickyBottomBar>` — in one fixed order and composition. The anatomy, widths, breakpoints, full-height-scroll pattern, and the surface-choice table (route / fullscreen overlay / modal / picker menu) live in `DESIGN.md` §1–4 and are mandatory reading before UI work.

Server prefetch rule (#887): a content page whose first frame reads a query serves that frame with data on a cold entry — the RSC page wraps the screen in `<Suspense fallback={<ScreenSkeleton />}>` + `ServerPrefetchBoundary` and lays out its first-frame queries through the `queryOptions` factories of the feature's dual `api/queries.ts` module with `serverApiClient`. Mechanics, scope rules, and gates live in `DESIGN.md` §15; hooks stay in `api/hooks.ts` ('use client') reading the same factories.

Column width rule (решение владельца 26.09, аудит #870): the content column is exactly the `PageContent` cap — `max-w-column` (560) — on every breakpoint. Content blocks sit **inside** the column with their own `px-6` (24px each side), so cards/pills render 512 wide on tablet/desktop and full-width-minus-24 on mobile. Negative-margin compensation (`-mx-N` + breakpoint `mx-0`, «гашу вставку кабинета») against the page wrapper is forbidden: the wrapper carries no horizontal padding since #865, so such a hack only pushes the block past the 560 cap (600px bug on the payments hubs). Inner `-mx` cancelling a container's **own** padding (a list spanning its padded card, `picker-field`) stays legitimate.

## Testing

Unit tests are pure-logic only — vitest runs in a node environment with no DOM, no testing-library, no msw. This is the standard, not a gap:

- Colocated `*.test.ts` next to the module. Priority targets: wire serializers and DTO→entity mappers (shape assertions with `toStrictEqual`), pure `lib/` and `shared/lib` modules (navigation, pwa, formatting).
- Components and API calls are not unit-tested; UI behavior is covered by Playwright e2e. Narrow exception (2026-09-30, `shared/ui/design/button.test.ts`; 02.10 — also `shared/ui/design/user-button.test.ts`): a stateless component may be called as a function and its React element tree inspected — no render, no DOM, no testing-library, same node-env canon.

## Quality bar

The gates below are active and enforced by `eslint.config.mjs`, `tsconfig.json`, and `next.config.ts` — write to them without waiting for anything. The rule history and flip mechanics are registered in `docs/agents/tooling.md`; new rule families land fix-then-flip per family (advisory counter drops to zero, then the rule flips to error) with this file and the registry updated in the same change.

### Configuration gates

- `react-hooks/exhaustive-deps` at error; `switch-exhaustiveness-check`; `consistent-type-imports` — `import type` is the only legal spelling for type-only imports.
- Dangerous browser APIs banned via restricted selectors — `dangerouslySetInnerHTML`, `innerHTML`, `insertAdjacentHTML`, `document.write`/`writeln`, `eval`, `new Function` — an XSS-class regression is mechanically impossible.
- Ambient `*.svg` module declarations in the shared layer (`shared/assets/svg.d.ts`): svg imports are typed `FC<SVGProps<SVGSVGElement>>`, not `any`. The file must stay **first** in `tsconfig.json` `include` — with duplicate wildcard `*.svg` declarations the first into the program wins, and Next's image-types fallback (via `next-env.d.ts`) types svg as `any`.
- tsconfig: `verbatimModuleSyntax`, `noImplicitOverride`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`; `reactStrictMode: true` explicit in `next.config.ts`.

### Type-checked core

A type-checked ESLint block (project service) with named rules, not a preset. Canonical spellings:

- `no-unnecessary-type-assertion`, `no-non-null-assertion` — a cast or `!` that the type system already knows is noise; a value the system doesn't know needs a guard (`def?.kind !== 'number'` early-throw), not an assertion.
- `no-base-to-string` — `String(unknown)` renders `[object Object]` at the worst moment; narrow to string with a fallback (the api client's `problemText`).
- `require-await`; `no-confusing-void-expression` (`ignoreArrowShorthand`) and `restrict-template-expressions` (`allowNumber`) — React-friendly options tuned in the config, not by suppressions. `void`-prefixing a void-returning call (App-Router `router.replace`) is the exact confusion the rule names — the prefix goes only on promises.
- `no-floating-promises` — a promise is handled or explicitly discarded, three canonical spellings: `void` on fire-and-forget `invalidateQueries` (hooks' `onSuccess`, post-mutation invalidation; `removeQueries` is synchronous in v5 and stays bare); `void` on `refetch()` in retry/conflict handlers — v5's `refetch` never rejects without `throwOnError` (query-core swallows the rejection; the error surfaces through `isError` → the error-state UI the handler serves); a real `.catch` with meaningful handling where the promise can genuinely reject (the SW updater's `serviceWorker.ready` reports via `reportClientError`) — never an empty catch.
- `no-misused-promises` — an async function never goes into a void-signature prop (`onClick`, `onSubmit`, `onRetry`, `onConfirm`, …) directly; the call site wraps it: `onClick={() => void handleSubmit()}`. The two shapes the wrappers bridge cannot reject: full-try/catch handlers (every `mutateAsync` path notifies its own error) and `refetch()`. A handler that can genuinely reject gets a real `.catch` at the wrapper instead — the wrapper must never be where an error dies silently.
- `no-deprecated` — deprecated APIs are replaced, not suppressed. Two spellings this codebase hit: form submit handlers take `SubmitEvent<T>` from `react` (React 19.2 deprecated `FormEvent`), and a deprecated Web API is replaced by the signal it actually carried (the iPadOS detector reads the `Macintosh` UA token instead of `navigator.platform`, pinned by a device-table characterization test).
- jsx-a11y `recommended` at error — the whole preset, no per-rule exceptions. `alt-text` keeps next's `img: ["Image"]` scope unioned with the preset's defaults. Custom listbox controls are keyboard-operable through `shared/ui/design/listbox-keyboard.ts`: Enter/Space select, Escape closes back onto the trigger, arrows/Home/End move roving focus with wrap-around, selection hands focus back to the trigger. Anchors carry real navigable `href`s — no `href="#"` placeholders. `autoFocus` is banned: step forms focus their field programmatically (`ref` + `useEffect` on the step, the login CodeStep's pattern).
- tsconfig `noUncheckedIndexedAccess` — index access returns `T | undefined`, handled explicitly, never with `!`. Canonical spellings: a `Record<K, string>` class map keyed by a literal union takes `styles.x ?? ''` in the initializer (missing class degrades to unstyled); an indexed element is bound and narrowed before use; regexp groups are extracted and checked; a fixed-shape array return types as a tuple (`weekDates` → `readonly [string × 7]`); iterating bytes/collections is for-of. CSS-module `styles.*` reads are `string | undefined` — compose class names with `clsx`, never by template interpolation.

### Money and condition gates

- The money gate: kopecks input/output lives in `shared/lib/format-money.ts` (`kopecksToRublesString` / `parseRublesToKopecks` — plain decimal strings, no symbol, no grouping; parse takes dot or comma, rounds to the kopeck, `undefined` for empty/invalid/negative, `{positive: true}` rejects zero) plus `ratioToPercent` for percent-homonym call sites. Selectors ban `/100` and `*100` (both operand sides) and `.toFixed` outside the two-module allowlist `MONEY_FORMATTING_ALLOWLIST` in `eslint.config.mjs` (the money module and the area formatter `features/property-attributes/lib/format.ts`).
- The allowlist block re-declares the security selectors for the two formatting modules — ignoring the files for the money selectors never drops their security coverage; extending the allowlist is a deliberate config edit registered in `docs/agents/tooling.md`, never a code-site exception.
- `no-unnecessary-condition` — a conditional the declared types already decide is a lie about the types beneath it; write the honest type or drop the check. A lookup in a total `Record<Union, T>` map is `T` — no `??` fallback and no truthiness guard before rendering; a check already made by narrowing is dead — remove it; a `?.` on a link the type says is non-nullish goes away; when the check existed only because the declared type was wider than reality, narrow the declaration instead (the tuple return of `weekDates` is the precedent).
- `prefer-nullish-coalescing` — `??` where nullish is what is meant; where `||` deliberately means "falsy means absent" (empty-string draft fields, an empty display name, ReactNode hint guards), the check is spelled explicitly (`draft.endDate?.length ? draft.endDate : undefined`, `Boolean(helperText)`, `name?.length ? name : 'Пользователь'`). Boolean OR over an optional prop is written as a destructure default (`disabled = false`); ternary `a !== undefined ? a : b` collapses to `a ?? b`.

### Zero tolerance for suppressions

`eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` — zero in manual code: fix the code, never suppress; the blocking counter `make ts-suppressions` guards the zero (`docs/agents/tooling.md`). Generated code (`shared/api/generated.ts`, `features/property-attributes/lib/generated/`) and build artifacts are outside the counter's scope.

### Security contour — lint gates and served headers

Four blocking gates in `eslint.config.mjs`, wired through the existing lint runs (pre-commit `frontend lint`, the CI `frontend` job):

- **Markdown is out of the product; its lint gates are tripwires**: the renderer was removed — react-markdown is gone from `package.json` and `MarkdownContent` from `shared/ui/`, no source renders markdown today. The `rehype-raw` import ban and the `urlTransform` prop ban remain in `eslint.config.mjs` as deliberate tripwires against a silent reintroduction; bringing the renderer back reactivates the policy (#331: `rehype-raw` stays banned, the renderer's default URL sanitizer is the policy, runtime sanitization (`rehype-sanitize`) was rejected) and the tripwires' fate is decided with it.
- **`NEXT_PUBLIC_*` reads are banned** in every static form (member, computed literal, destructuring from `process.env`); the `PUBLIC_ENV_ALLOWLIST` in the config is the deliberate exposure list — empty today, so exposing a variable to the client bundle is a config edit, never a silent code read.
- **Web storage is banned outside its three owners**: `localStorage`/`sessionStorage` (bare, `window.`, `globalThis.` forms) live only in `features/auth/lib/**` (login draft, resend cooldown), `shared/lib/hooks/useDraftStore.ts`, and `features/push-notifications/lib/push-prompt-flag.ts` (the per-browser «авто-промпт показывали» flag, спека #1028 §3 — the flag must be per-browser, not on the backend) — session tokens stay in httpOnly cookies.
- **Dangerous browser APIs are banned** (see Configuration gates).

Security headers are served by `next.config.ts` — XCTO, XFO DENY, Referrer-Policy, Permissions-Policy, `poweredByHeader: false`, and CSP step 1 (`unsafe-inline` only for script/style; dev adds `'unsafe-eval'` for React Refresh). The strict directive tail is shared with CSP step 2 as `CSP_BASE_DIRECTIVES` in `shared/lib/csp.ts` — new external origins (CDN, fonts, analytics) require a `connect-src`/`img-src`/`font-src` edit there, which moves both policies at once.

CSP step 2: with `CSP_REPORT_ONLY=true` (stage-only env), `proxy.ts` additionally serves the future strict policy — nonce + `strict-dynamic` — as `Content-Security-Policy-Report-Only`, and violations land in `/api/csp-report` → stdout → Uptrace. Without the flag the app behaves exactly as before; the blocking flip is a separate decision on the collected data (`docs/research/2026-08-23-csp-step2-report-only.md`).

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **`key={index}`** on a list that can reorder. → key by stable entity id.
- **Naked data surface** — a new query render path with no loading state (inline `isLoading` + skeleton/`FinanceLoading`) and no error path. → both states ship with the feature.
- **Mutation without invalidation** — success leaves the cache stale. → `void invalidateQueries` via the key registry in `onSuccess`.
- **Hand-rolled formatting** — `/100` money math or ad-hoc date strings. → `formatMoneyKopecks`, `@react-aria/i18n` (ru-RU) helpers.
- **Floating promise in a handler** — an async event handler with no error path. → `catch` → toast via `shared/lib/toast` / `ApiError`. (The rubric covers what the `no-floating-promises` `void` escape hatch cannot judge — whether the discard is justified — and the same judgement backs `no-misused-promises`' void-wrappers: the wrapped handler must be full try/catch or a never-rejecting `refetch`.)
- **Synchronous `searchParams`** — reading the Next 16 promise directly. → `await` it in the server page and pass parsed initial props into the client widget.
