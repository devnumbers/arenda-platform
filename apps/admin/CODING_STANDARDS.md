# apps/admin/CODING_STANDARDS.md

How the react-admin back-office SPA is built and reviewed. Read before implementing or reviewing an admin change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- The four ESLint gates (`no-explicit-any`, URL literals, `generated/` imports, raw `fetch` boundaries) are enforced by `eslint.config.mjs`; review does not re-report what lint blocks.
- Invariants, stack versions, and commands: `AGENTS.md` (same directory). Decisions: `docs/adr/`.
- The backend contract (`openapi.yaml`, audit action registry, sort whitelists) is the source of truth this app mirrors — see the sync-test rules below.

## Resource file anatomy

- One `.tsx` per resource at `src/`, colocating List/Show views and their components. Pattern: `src/leases.tsx` — export the shared columns (`LeaseDatagrid`) for reuse by both the resource's `List` and other resources' embedded tabs (`ReferenceManyField` in `Show`).
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

- Choice catalogs (`roleChoices`, `leaseStatusChoices`, …) mirror `openapi.yaml` and the audit registry, with Russian labels. **Every catalog ships a sync contract test** (in `src/lib/*.test.ts`): enum values from the backend source asserted to be covered exactly once with non-empty labels — drift fails the suite, not production.
- Money displays only through `MoneyField` / `formatKopecks` (`Intl` ru-RU); never ad-hoc `/100`.
- Link and Reference wrappers (`UserLinkField`, `UserReferenceField`, …) avoid extra requests; the documented exception is `ReferenceField` in list views, where RA batches `getMany`.
- Names through `fullName`/`asPersonName`; enum chips through `ChoiceChipField`.

## i18n (codified actual practice)

Resource names and field labels are localized via `ra-i18n-polyglot` with `ra-language-russian`; custom strings go into `customMessages` in `src/i18n/index.ts` (shallow-merged over the Russian pack; polyglot plurals with `||||`; dotted source keys allowed). Hardcoded Russian in filters, dialogs, and choice labels is acceptable — do not introduce a second i18n mechanism for them.

## Testing

Unit tests are pure-logic only — vitest in a node environment, colocated `*.test.ts` in `src/lib/`. Two signature patterns:

- **Sync-with-backend contract tests** — a `const` array of enum values copied from `openapi.yaml`/backend domain files, asserted to be covered exactly once with non-empty Russian labels (see `tariffs.test.ts`).
- **Behavior tests** — `it.each` over inputs: formatting edges (`-1` → «Безлимит»), unknown-enum pass-through, fallbacks (`#id`).

Components and `dataProvider` mapping are not unit-tested; that is the standard, not a gap.

## Review rubric — smells ESLint does not catch

Judgement calls for the Standards axis, not violations. Read each as *what it is* → *how to fix*.

- **Resource without `recordRepresentation`** — references and tabs render raw ids. → add a representation helper in `App.tsx`.
- **Sortable column outside the whitelist** — the header lies (dataProvider drops the sort). → `sortable={false}` or extend `sortableFieldsByResource` with a comment citing the backend.
- **Catalog without a contract test** — a new choice list with no sync test against the backend source. → write the `it.each` coverage test in `src/lib`.
- **Fetch-shaped need in a component** — logic that wants a new endpoint call. → custom `dataProvider` method; `fetch` stays in the sanctioned boundary files.
- **Ad-hoc `sx` duplicating `fields.tsx`** — money, names, chips, links hand-rolled inline. → reuse the field/helper.
- **Write path added quietly** — a mutation outside `tariffs` without the read-only decision recorded. → flag it; read-only is the default.
- **Missing Russian label** — a catalog entry with an empty or English label. → fill the label; UI language is Russian.
