# apps/admin/CODING_STANDARDS.md

How the react-admin back-office SPA is built and reviewed. Read before implementing or reviewing an admin change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- The wave-2C ESLint gates (`no-explicit-any`, URL literals, `generated/` imports, raw `fetch` boundaries) and the quality-bar gates (the type-checked preset, react-hooks, money selectors) are enforced by `eslint.config.mjs`; review does not re-report what lint blocks.
- Invariants, stack versions, and commands: `AGENTS.md` (same directory). Decisions: `docs/adr/`.
- The backend contract (`openapi.yaml`, audit action registry, sort whitelists) is the source of truth this app mirrors — see the sync-test rules below.

## Resource file anatomy

- One `.tsx` per resource at `src/`, colocating List/Show views and their components. Pattern: `src/properties.tsx` — colocated `PropertyList`/`PropertyShow` plus shared blocks (`PropertyAttributesBlock`) reused by other views.
- `List` uses `rowClick="show"` and `bulkActionButtons={false}` (the app is read-only — see below). Columns outside the backend sort whitelist get `sortable={false}`.
- Every `<Resource>` in `App.tsx` gets an MUI icon and an explicit `recordRepresentation` (a string field or a helper function with JSDoc) — without one, RA renders bare ids in references.
- Cross-resource panels (e.g. subscription panels in `UserShow`) are exported components from a colocated module, not routed resources.

## Read-only by design

The admin is a read-only console over the backend: every resource except `tariffs` is List + Show only, and `dataProvider` throws 405 for `delete`/`updateMany`. Write access for `tariffs` goes through Dialog buttons (`EditTariffButton`/`CreateTariffButton`) with `useUpdate`/`useCreate` — not routed Create/Edit pages. Extending write access to another resource is a deliberate product decision that must be reflected in `dataProvider` (URL mapping, editable fields, tests) — never a quiet default.

## dataProvider conventions

All backend calls go through `src/dataProvider.ts` (or `authProvider.ts`) — the design pull, not just the ESLint gate:

- URL mapping lives in the `listUrl`/`oneUrl` switches; unknown resources throw.
- Sorting: `sortableFieldsByResource` mirrors the backend sort whitelists (comments cite them); out-of-whitelist sorts are silently dropped to avoid backend 400s — keep the map in sync when the backend whitelist changes.
- Pagination via `limit`/`offset`; resources without pagination are listed in `unpaginatedResources`.
- Response parsing accepts both `X-Total-Count` and envelope shapes; `getOne` unwraps per-resource envelope keys in its switch.
- Errors: non-OK responses are parsed as RFC7807 `ProblemDetails` and thrown as react-admin `HttpError` with the body — keep that shape for new endpoints.
- New admin operations become **custom methods** on the `AdminDataProvider` interface (`refundPayment`, `getStats`, …), implemented once in `dataProvider.ts`; components call the method, never `fetch`.

## fields.tsx — choices and display

- Choice catalogs (`roleChoices`, `propertyStatusChoices`, …) mirror `openapi.yaml` and the audit registry, with Russian labels. **Every catalog ships a sync contract test** (in `src/lib/*.test.ts`): enum values from the backend source asserted to be covered exactly once with non-empty labels — drift fails the suite, not production.
- Money displays only through `MoneyField` / `formatKopecks` (`Intl` ru-RU); never ad-hoc `/100` — enforced by the money selectors (raw `/100`/`*100` and `.toFixed` banned outside `fields.tsx`).
- Link and Reference wrappers (`UserLinkField`, `UserReferenceField`, …) avoid extra requests; the documented exception is `ReferenceField` in list views, where RA batches `getMany`.
- Names through `fullName`/`asPersonName`; enum chips through `ChoiceChipField`.

## i18n (codified actual practice)

Resource names and field labels are localized via `ra-i18n-polyglot` with `ra-language-russian`; custom strings go into `customMessages` in `src/i18n/index.ts` (shallow-merged over the Russian pack; polyglot plurals with `||||`; dotted source keys allowed). Hardcoded Russian in filters, dialogs, and choice labels is acceptable — do not introduce a second i18n mechanism for them.

## Testing

Unit tests are pure-logic only — vitest in a node environment, colocated `*.test.ts` in `src/lib/`. Two signature patterns:

- **Sync-with-backend contract tests** — a `const` array of enum values copied from `openapi.yaml`/backend domain files, asserted to be covered exactly once with non-empty Russian labels (see `tariffs.test.ts`).
- **Behavior tests** — `it.each` over inputs: formatting edges (`-1` → «Безлимит»), unknown-enum pass-through, fallbacks (`#id`).

Components and `dataProvider` mapping are not unit-tested; that is the standard, not a gap.

## Quality bar — admin track (tracks I–II in force since 2026-08-22, tickets #391/#392; map #326, spec #378)

The admin joins the frontend quality bar (bar [#330](https://github.com/devnumbers/arenda-platform/issues/330), spec [#378](https://github.com/devnumbers/arenda-platform/issues/378)) on its own minimal-config track — the whole mode adds exactly one devDependency, `eslint-plugin-react-hooks`. Both tracks are tool-enforced: track I landed the preset, track II typed the HTTP boundaries ([#392](https://github.com/devnumbers/arenda-platform/issues/392)).

- `typescript-eslint` `recommendedTypeChecked` + `prefer-nullish-coalescing` run at error level over all manual code (`src` minus `src/lib/generated/` — the only exclusion left). Since track II the HTTP boundaries are typed and under the preset: client wrappers return response JSON as `unknown`, target types and primitive narrowing happen at the edge, react-admin's `any`-typed params (`filter`, bare `GetOneParams`/`GetManyParams`) are anchored with `Record<string, unknown>` filters and param generics, and unsupported provider methods return rejected promises. Custom provider methods are typed via `useDataProvider<AdminDataProvider>()`; async handlers in void-signature props go through `void`-wrappers (the handlers catch everything themselves).
- `eslint-plugin-react-hooks` `recommended` — conditional hook calls and hook discipline checked statically, not by runtime crashes; synchronous `setState` in an effect body is an error (reset state in the triggering handler, not in the effect).
- Money selectors: raw `/100`/`*100` arithmetic and `.toFixed` banned outside `src/fields.tsx` (and outside `src/lib/generated/` — generated code is not fixed by hand), the same file-allowlist approach as frontend wave C. The selectors live in a files-scoped block that restates the URL-literal selectors — flat-config last-write-wins replaces `no-restricted-syntax` per file set, the same trap the frontend config documents.
- `src/lib/generated/` stays behind files-scoped config blocks; the generator template is not adapted to the ESLint bar. One motivated exception landed with track I: `emit-admin-ts.mjs` emits `groups[groups.length - 1]?.items.push(...)` instead of the non-null-safe indexing — tsconfig flags are program-wide and generated code cannot be files-scoped out of `tsc`, so the one-token `?.` hardening (behavior identical — the group is pushed by the guard above) is the only non-suppressing path to a green gate.
- tsconfig: `noUncheckedIndexedAccess`, `verbatimModuleSyntax` — indexed access carries `| undefined` (`??` / narrowing, not assertions), type-only imports are spelled `import type` / `type` specifiers.
- jsx-a11y stays frontend-only (rejected for the admin — MUI and react-admin carry the semantics).

Zero tolerance for suppressions: `eslint-disable*` comments, explicit `any`, and `@ts-ignore`/`@ts-expect-error` — zero in manual code (`src` outside `src/lib/generated/`): fix the code, never suppress; a blocking CI counter guards the zero (accepted gate, `docs/agents/tooling.md`). The admin has no legal suppressions today.

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **Resource without `recordRepresentation`** — references and tabs render raw ids. → add a representation helper in `App.tsx`.
- **Sortable column outside the whitelist** — the header lies (dataProvider drops the sort). → `sortable={false}` or extend `sortableFieldsByResource` with a comment citing the backend.
- **Catalog without a contract test** — a new choice list with no sync test against the backend source. → write the `it.each` coverage test in `src/lib`.
- **Fetch-shaped need in a component** — logic that wants a new endpoint call. → custom `dataProvider` method; `fetch` stays in the sanctioned boundary files.
- **Ad-hoc `sx` duplicating `fields.tsx`** — money, names, chips, links hand-rolled inline. → reuse the field/helper.
- **Write path added quietly** — a mutation outside `tariffs` without the read-only decision recorded. → flag it; read-only is the default.
- **Missing Russian label** — a catalog entry with an empty or English label. → fill the label; UI language is Russian.
