# apps/frontend/CODING_STANDARDS.md

How the Next.js frontend is built and reviewed. Read before implementing or reviewing a frontend change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- Import boundaries, the DTO isolation gates, and the `@heroui/styles` ban are enforced by `eslint.config.mjs` (`boundaries/dependencies`, `no-restricted-imports`); review does not re-report what lint blocks.
- The security lint gates — no `rehype-raw` / no `urlTransform` prop (react-markdown secure by default), `NEXT_PUBLIC_*` only via the `PUBLIC_ENV_ALLOWLIST` in the config, web storage only in `features/auth/lib/**` and `useDraftStore` — are enforced by `eslint.config.mjs` (decision #331; see the Security contour section below).
- Invariants and commands: `AGENTS.md` (same directory). Domain language: per-context `CONTEXT.md` (index in `CONTEXT-MAP.md`). Decisions: `docs/adr/`.
- New UI (payments design layer onward) is built on shadcn/ui over Radix primitives, styled with Tailwind utilities on top of our tokens (ADR 0050). Legacy HeroUI v3 widgets: verify APIs through the `heroui-react` MCP server until migrated — v3 is beta and not in model training data.

## FSD slice anatomy

Layers live at the app root (`app/`, `widgets/`, `features/`, `entities/`, `shared/`; alias `@/*`). What belongs where:

- **`features/<name>/` is thin by design**: `api/hooks.ts` (react-query hooks), optional `lib/` (pure helpers), optional `ui/` (components), `index.ts`. No `model/` in features — server state lives in react-query, local UI state in `useState`/URL.
- **`entities/<name>/`** holds the domain model mapped from the backend: `model/types.ts` (camelCase entity types + Command types), `model/mappers.ts` (DTO→entity), optional `lib/` (format helpers) and `ui/` (list/badge components).
- **`widgets/<name>/`** composes page blocks out of features, entities, and `shared/ui`.
- A slice's `index.ts` is its only import surface from other slices (enforced); keep it minimal — export only what other elements actually import.

Adding a new slice: user scenario → feature; domain model shared across scenarios → entity; page block → widget. If it fetches, its keys go into `shared/api/query-keys.ts`. Before adding any component, hook, or helper, search `shared/ui`, `shared/lib`, and existing slices (knip also guards dead exports). The one deliberate exception is `features/popups/**` (ignored in `knip.json`): the popup mechanism is spec-mandated infrastructure (ADR 0023, spec #434) kept alive with no consumers until the next onboarding popup ships.

## Data flow across the DTO boundary

- `shared/api/generated.ts` is generated and imported only inside `shared/api`; the rest of the app imports DTO types from `shared/api/dto.ts` (the re-export) or, better, not at all.
- DTO→entity mapping lives in `entities/<name>/model/mappers.ts`: snake_case → camelCase, `?? null` normalization, and page-wrapper mapping (`items/limit/offset/has_more/next_offset`) for paginated endpoints. Example: `entities/user/model/mappers.ts`.
- Requests use camelCase **Command types** (`UserUpdateCommand` in `entities/user/model/types.ts`); mutations take the command and serialize inside the feature: when the endpoint is snake_case, add an explicit `toCreateWireRequest`/`toUpdateWireRequest` in the feature's `api/`, covered by a wire test.

## react-query conventions

- Hooks live in `features/<name>/api/hooks.ts`, named `useXxx`, typed like `UseMutationResult<Entity, ApiError, XxxCommand>`; API errors normalize to `ApiError` (`shared/api/errors.ts`).
- Cache keys live in the single registry `shared/api/query-keys.ts` — one `xxxKeys` factory per feature: `all: ['domain'] as const` plus parameterized keys composed from `all` (`detail(id)`, `byProperty(propertyId)`). The registry is shared because cross-feature invalidation must bypass the cross-slice import ban.
- Mutations invalidate in `onSuccess` through the registry (`void invalidateQueries({ queryKey: xxxKeys.all })` — the fire-and-forget spelling, see the wave B `no-floating-promises` gate); the profile pattern is `setQueryData` for the optimistic value plus `invalidateQueries`; deletes use `removeQueries`, which is synchronous in v5 and stays bare.
- `QueryClient` is configured once in `shared/providers/query-provider.tsx` (`staleTime: 30_000`, `refetchOnWindowFocus: false`) — don't override per-query without a stated reason.

## Forms

No form library and no schema validator — this is deliberate, not a gap:

- Controlled `useState` fields + `touched`/`submitAttempted` flags + derived validity + a derived `canSubmit`. Pattern: `widgets/profile/ui/PersonalDataForm.tsx`.
- Validation error strings are hardcoded Russian, inline next to the field.
- Property attributes validate through the generated validators (`features/property-attributes/lib/validate.ts` re-exports the generated catalog validators) — never hand-roll rules the catalog already encodes.
- Persisted form drafts (survive a refresh mid-flow) go through the shared draft store — `shared/lib/hooks/useDraftStore` (`useSyncExternalStore` with `getServerSnapshot`, so the draft loads after hydration with no setState-in-effect). The slice's `use-*-draft.ts` wrapper owns only the storage key, the default, `validate`, and the terminal-step predicate; never hand-roll the sessionStorage load/persist/clear cycle — web storage outside `features/auth/lib/**` and `useDraftStore` is a lint error (decision #331).

## Components, styling, and React Compiler

- Styling: existing components use CSS Modules + design tokens (`shared/styles/tokens.css`); new design-layer components (ADR 0050) are shadcn/ui over Radix, styled with Tailwind utilities on the same tokens. The transitional mix is accepted until the app-wide migration.
- Touch-safe hover (decision 2026-08-26): Tailwind `hover:` utilities are already emitted inside `@media (hover: hover)` by Tailwind v4 — use them freely in the design layer. In CSS Modules, every `:hover` rule must sit inside an `@media (hover: hover)` block (all legacy rules were wrapped by codemod; new ones follow the pattern), so hover styles never fire on touch devices.
- Unified motion (decision 2026-08-26, after HIG Motion / WWDC23): one Apple curve `cubic-bezier(0.32, 0.72, 0, 1)` (`--dl-ease`) everywhere — 250ms (`--dl-duration-fast`) for colors/backgrounds/borders, 350ms (`--dl-duration-move`) for movement/transforms, 300ms for the TextField floating label. Tailwind picks the defaults up via `--default-transition-*` in `@theme` (named utilities: `ease-apple`, `duration-fast`, `duration-move`); CSS Modules spell the vars out (`transition: background-color var(--dl-duration-fast) var(--dl-ease)`). No hardcoded `Ns ease` values; `prefers-reduced-motion` is deliberately not gated (owner's call).
- Focus (decision 2026-08-26): input fields carry **no focus ring at all** — the caret is the focus affordance; do not re-add `focus-within:`/`focus-visible:` rings to field boxes. Buttons/links keep their `:focus-visible` ring: it only appears on keyboard focus and never on mouse clicks.
- `shared/ui/` is the app's own kit: folder-per-component (`Button.tsx` + `Button.module.css` + `index.ts`), ~23 wrappers. Wrap, don't bypass; `/ui-kit` is the gallery route.
- HeroUI v3 (legacy widgets only, ADR 0050): provider-less — import `@heroui/react` components directly. `@heroui/styles` appears exactly once — the `@import` in `app/globals.css` — and never in TSX.
- **React Compiler is on** (`next.config.ts`). Manual `useMemo`/`useCallback`/`memo` is not the default: write plain code and let the compiler memoize. Reach for manual memoization only where the compiler provably can't help (values escaping to non-React code) and justify it with a comment.
- The memoization, derived-state, and effect-synchronizer smells are enforced by the tool, not the review rubric: the react-hooks v7 compiler rules already run through `eslint-config-next` 16.3.1 (the plugin's `recommended` preset is spread whole) — `purity`, `set-state-in-effect`, `set-state-in-render`, `use-memo`, `immutability`, `refs`, `preserve-manual-memoization`, `static-components`, `globals`, `error-boundaries`, `gating` at error; `rules-of-hooks` at error; `incompatible-library`/`unsupported-syntax` at warn; `exhaustive-deps` at error (quality bar wave A). Fix them at lint time.

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

The quality bar is decided (bar [#330](https://github.com/devnumbers/arenda-platform/issues/330), spec [#378](https://github.com/devnumbers/arenda-platform/issues/378)) and lands ticket by ticket — the patterns below are the standard now; write to them without waiting for the flips. Flip conventions: wave A lands flip-with-fix (config flips together with its code fixes); waves B/C land fix-then-flip per rule family — a family's advisory counter drops to zero, then the rule flips to error, and this section plus `docs/agents/tooling.md` update in the same change. Already active today: wave A (ticket #390, section below), the wave B families (tickets #393–#398, section below), the wave C money gate (ticket #403, section below), wave C's `no-unnecessary-condition` (ticket #404, section below), and, ahead of the waves, the react-hooks v7 compiler rules (see the React Compiler section above).

### Wave A — configuration (in force since 2026-08-21, ticket [#390](https://github.com/devnumbers/arenda-platform/issues/390))

Enforced by `eslint.config.mjs`, `tsconfig.json`, and `next.config.ts`:

- `react-hooks/exhaustive-deps` at error; `switch-exhaustiveness-check` (the config's type-checked block — the `projectService` foundation the wave B families extend); `consistent-type-imports` — `import type` is the only legal spelling for type-only imports.
- Dangerous browser APIs banned via restricted selectors — `dangerouslySetInnerHTML`, `innerHTML`, `insertAdjacentHTML`, `document.write`/`writeln`, `eval`, `new Function` — an XSS-class regression becomes mechanically impossible.
- Ambient `*.svg` module declarations in the shared layer (`shared/assets/svg.d.ts`): svg imports are typed `FC<SVGProps<SVGSVGElement>>`, not `any`. The file must stay **first** in `tsconfig.json` `include` — with duplicate wildcard `*.svg` declarations the first into the program wins, and Next's image-types fallback (via `next-env.d.ts`) types svg as `any`.
- tsconfig: `verbatimModuleSyntax`, `noImplicitOverride`, `noUnusedLocals`, `noUnusedParameters`, `noImplicitReturns`; `reactStrictMode: true` explicit in `next.config.ts`.

### Wave B — type-checked core, rule by rule

A type-checked ESLint block (project service) with named rules, not a preset. First families in force since 2026-08-22 (tickets [#393](https://github.com/devnumbers/arenda-platform/issues/393), [#394](https://github.com/devnumbers/arenda-platform/issues/394), [#395](https://github.com/devnumbers/arenda-platform/issues/395), [#396](https://github.com/devnumbers/arenda-platform/issues/396)); jsx-a11y `recommended` in force since 2026-08-22 (ticket [#397](https://github.com/devnumbers/arenda-platform/issues/397)); tsconfig `noUncheckedIndexedAccess` in force since 2026-08-22 (ticket [#398](https://github.com/devnumbers/arenda-platform/issues/398)):

- `no-unnecessary-type-assertion`, `no-non-null-assertion` — a cast or `!` that the type system already knows is noise; a value the system doesn't know needs a guard (`def?.kind !== 'number'` early-throw), not an assertion.
- `no-base-to-string` — `String(unknown)` renders `[object Object]` at the worst moment; narrow to string with a fallback (the api client's `problemText`).
- `require-await`; `no-confusing-void-expression` (`ignoreArrowShorthand`) and `restrict-template-expressions` (`allowNumber`) — React-friendly options tuned in the config, not by suppressions. `void`-prefixing a void-returning call (App-Router `router.replace`) is the exact confusion the rule names — the prefix goes only on promises.
- `no-floating-promises` (ticket #394) — a promise is handled or explicitly discarded, three canonical spellings: `void` on fire-and-forget `invalidateQueries` (hooks' `onSuccess`, post-mutation invalidation; `removeQueries` is synchronous in v5 and stays bare — the prefix goes only on promises); `void` on `refetch()` in retry/conflict handlers — v5's `refetch` never rejects without `throwOnError` (query-core swallows the rejection; the error surfaces through `isError` → the error-state UI the handler serves, so the state machine is the error path); a real `.catch` with meaningful handling where the promise can genuinely reject (the SW updater's `serviceWorker.ready` reports via `reportClientError`) — never an empty catch.
- `no-misused-promises` (ticket #395) — an async function never goes into a void-signature prop (`onClick`, `onSubmit`, `onRetry`, `onConfirm`, …) directly; the call site wraps it: `onClick={() => void handleSubmit()}`. Honest by construction, not a style call — the two shapes the wrappers bridge cannot reject: full-try/catch handlers (every `mutateAsync` path notifies its own error) and `refetch()` (the #394 semantics above). A handler that can genuinely reject gets a real `.catch` at the wrapper instead — the wrapper must never be where an error dies silently.
- `no-deprecated` (ticket #396) — deprecated APIs are replaced, not suppressed. Two spellings this codebase hit: form submit handlers take `SubmitEvent<T>` from `react` (React 19.2 deprecated `FormEvent` — `onSubmit` is typed `SubmitEventHandler`; the swap is type-level only), and a deprecated Web API is replaced by the signal it actually carried (the iPadOS detector reads the `Macintosh` UA token — byte-identical in spoof mode — instead of `navigator.platform`, pinned by a device-table characterization test).
- jsx-a11y `recommended` at error (ticket #397) — the whole preset, no per-rule exceptions. The plugin ships with `eslint-config-next`, so the config spreads the preset's rules only (re-declaring the plugin would be a config error); `alt-text` keeps next's `img: ["Image"]` scope unioned with the preset's defaults. Custom listbox controls are keyboard-operable through the shared listbox keyboard module (`shared/ui/select/listbox-keyboard.ts`: `runListboxAction` dispatches an option row's key events, `listboxKeyAction` is its pure tested core, `moveOptionFocus`/`focusListboxEdge` move roving focus over `[role="option"]` rows): Enter/Space select, Escape closes back onto the trigger (Select's listbox-level Escape listener serves every row, footer rows included), arrows/Home/End move focus with wrap-around, and selection hands focus back to the trigger. Anchors carry real navigable `href`s (the login step's legal links point at the privacy-policy document — no `href="#"` placeholders). `autoFocus` is banned: step forms focus their field programmatically (`ref` + `useEffect` on the step, the login CodeStep's pattern), which keeps the focus point under component control instead of a mount-time attribute.
- tsconfig `noUncheckedIndexedAccess` (ticket #398) — index access returns `T | undefined`, and the undefined is handled explicitly, never with `!` (banned since the first families). Canonical spellings: a `Record<K, string>` class map keyed by a literal union stays total — the initializer takes `styles.x ?? ''`, so a missing CSS class degrades to unstyled instead of crashing; an indexed element is bound and narrowed before use (`const item = items[activeIndex]; if (item !== undefined) …` — the suggestion inputs' Enter-picks, which also closes the stale-index crash window); regexp groups are extracted and checked; a function that returns a fixed-shape array types it as a tuple (`weekDates` → `readonly [string × 7]`), moving the length invariant into the type so callers' `days[0]` needs no fallback; iterating bytes/collections is for-of (iterator access is what the flag exempts). CSS-module `styles.*` reads are `string | undefined` — compose class names with `clsx` (drops falsy), never by template interpolation, which both satisfies `restrict-template-expressions` (allowNullish stays false) and kills the literal `"undefined"` class name a typo'd lookup would interpolate.

### Wave C — money gate (in force since 2026-08-23, ticket [#403](https://github.com/devnumbers/arenda-platform/issues/403)); `no-unnecessary-condition` (in force since 2026-08-23, ticket [#404](https://github.com/devnumbers/arenda-platform/issues/404)); `prefer-nullish-coalescing` (in force since 2026-08-23, ticket [#405](https://github.com/devnumbers/arenda-platform/issues/405))

- The money gate landed two-step (fix-then-flip in one change): (1) code prep — the kopecks input/output pair in `shared/lib/format-money.ts` (`kopecksToRublesString` / `parseRublesToKopecks` — plain decimal strings for input fields, no currency symbol, no digit grouping; parse takes dot or comma, rounds to the kopeck, returns `undefined` for empty/invalid/negative input, and `{positive: true}` rejects zero for the strictly-positive amounts) plus the percent helper `ratioToPercent` for the percent-homonym call sites (progress fills, bar-chart widths); (2) selectors banning `/100` and `*100` (both operand sides) and `.toFixed` outside the two-module allowlist `MONEY_FORMATTING_ALLOWLIST` in `eslint.config.mjs` — the money module and the area formatter (`features/property-attributes/lib/format.ts`). After step 1 the selectors hit zero findings by construction; the grid's 15 money/percent code sites (lease/operation edit forms, the lease-creation wizard, the operations model, the lease card, the profit report) migrated to the helpers — sums unchanged — and the 16th finding, the area formatter's `.toFixed`, stays as the allowlist's second module by design.
- The allowlist block re-declares the security selectors for the two formatting modules (from the shared `SECURITY_RESTRICTED_SYNTAX` array): ignoring the files for the money selectors never drops their security coverage — extending the allowlist is a deliberate config edit registered in `docs/agents/tooling.md`, never a code-site exception.
- `no-unnecessary-condition` (ticket #404) — a conditional the declared types already decide is a lie about the types beneath it; write the honest type or drop the check. Canonical shapes: a lookup in a total `Record<Union, T>` map is `T` — no `??` fallback and no truthiness guard before rendering (`statusLabels[status]`, the status-icon map); a check already made by narrowing (an aliased `canSubmit`, an earlier `if` over a two-value union) is dead — remove it, don't restate it; a `?.` on a link the type says is non-nullish (`window.matchMedia`, `tariff` inside a non-null subscription object) goes away; when the check existed only because the declared type was wider than reality, narrow the declaration instead (the tuple return of `weekDates` is the wave-B precedent).
- `prefer-nullish-coalescing` (ticket #405) — `??` where nullish is what is meant; where `||` deliberately means "falsy means absent" (empty-string draft fields, an empty display name, ReactNode hint guards), the check is spelled explicitly instead (`draft.endDate?.length ? draft.endDate : undefined`, `Boolean(helperText)`, `name?.length ? name : 'Пользователь'`) — the reader sees which falsy values the site treats as absent. Boolean OR over an optional prop is written as a destructure default (`disabled = false`), which restores plain `||` over non-nullable booleans; ternary `a !== undefined ? a : b` collapses to `a ?? b`.

### Zero tolerance for suppressions

`eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` — zero in manual code: fix the code, never suppress; the blocking counter `make ts-suppressions` guards the zero since #399 (in force, `docs/agents/tooling.md`). The last 8 directives were eliminated in code, not moved: PhotoGrid turned out to be dead code and was deleted, the four prop-sync `set-state-in-effect` disables became the render-time prop adjustment from the React docs, and the push-status hook's `exhaustive-deps` disable became an explicit `useCallback`. Generated code and build artifacts are outside the counter's scope.

### Security contour — lint gates and served headers in force (decision #331, tickets #387/#388; browser-API bans — wave A #390)

Four blocking gates in `eslint.config.mjs`, wired through the existing lint runs (pre-commit `frontend lint`, the CI `frontend` job):

- **Markdown stays secure by default**: the `rehype-raw` import is banned and the `urlTransform` prop may not be passed at all — react-markdown's default URL sanitizer is the policy; runtime sanitization (`rehype-sanitize`) was rejected. Changing the markdown content source from repo files to API/DB reopens the decision.
- **`NEXT_PUBLIC_*` reads are banned** in every static form (member, computed literal, destructuring from `process.env`); the `PUBLIC_ENV_ALLOWLIST` in the config is the deliberate exposure list — empty today, so exposing a variable to the client bundle is a config edit, never a silent code read.
- **Web storage is banned outside its two owners**: `localStorage`/`sessionStorage` (bare, `window.`, `globalThis.` forms) live only in `features/auth/lib/**` (login draft, resend cooldown) and `shared/lib/hooks/useDraftStore.ts` — session tokens stay in httpOnly cookies.
- **Dangerous browser APIs are banned** (wave A): `dangerouslySetInnerHTML`, `innerHTML`, `insertAdjacentHTML`, `document.write`/`writeln`, `eval`, `new Function` — restricted selectors in the config's security block; zero usages today, the gate exists so an XSS-class regression is a lint error, not a review call.

Security headers (decision #331, ticket #388) are served by `next.config.ts` — XCTO, XFO DENY, Referrer-Policy, Permissions-Policy, `poweredByHeader: false`, and CSP step 1 (`unsafe-inline` only for script/style; dev adds `'unsafe-eval'` for React Refresh). The strict directive tail is shared with CSP step 2 as `CSP_BASE_DIRECTIVES` in `shared/lib/csp.ts` — new external origins (CDN, fonts, analytics) require a `connect-src`/`img-src`/`font-src` edit there, which moves both policies at once — same change as the registry update, never a silent code dependency.

CSP step 2 recon (ticket #406): with `CSP_REPORT_ONLY=true` (stage-only env), `proxy.ts` additionally serves the future strict policy — nonce + `strict-dynamic` — as `Content-Security-Policy-Report-Only`, and violations land in `/api/csp-report` → stdout → Uptrace. Not a dev-time concern: without the flag the app behaves exactly as before, and the blocking flip is a separate decision on the collected data (`docs/research/2026-08-23-csp-step2-report-only.md`).

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **`key={index}`** on a list that can reorder. → key by stable entity id.
- **Naked data surface** — a new query render path with no loading state (inline `isLoading` + skeleton/`FinanceLoading`) and no error path. → both states ship with the feature.
- **Mutation without invalidation** — success leaves the cache stale. → `void invalidateQueries` via the key registry in `onSuccess`.
- **Hand-rolled formatting** — `/100` money math or ad-hoc date strings. → `formatMoneyKopecks`, `@react-aria/i18n` (ru-RU) helpers. (Becomes the money-gate selectors in wave C.)
- **Floating promise in a handler** — an async event handler with no error path. → `catch` → toast via `shared/lib/toast` / `ApiError`. (The `no-floating-promises` gate of wave B; the rubric now covers what the rule's `void` escape hatch cannot judge — whether the discard is justified — and the same judgement backs `no-misused-promises`' void-wrappers: the wrapped handler must be full try/catch or a never-rejecting `refetch`.)
- **Synchronous `searchParams`** — reading the Next 16 promise directly. → `await` it in the server page and pass parsed initial props into the client widget.
