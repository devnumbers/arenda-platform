# ADR 0027: Single-Source Codegen for the Property Attributes Catalog

## Status

Accepted (2026-08-04)

## Context

The property attributes catalog (10 property types, ~20 fields each, enum
options, numeric ranges, cross-field rules, Russian display labels) was
maintained by hand in three places across the monorepo:

- **Go** — `apps/backend/internal/properties/domain/attributes.go` held the
  field definitions, `ValidateAttributes`, `validateField`, and
  `crossFieldErrors` used by the backend to validate create/update.
- **Frontend TS** — `apps/frontend/features/property-attributes/lib/{catalog,
  labels, validate, format}.ts` held the catalog shape, Russian labels,
  validation, and formatting used by the property forms.
- **Admin TS** — `apps/admin/src/lib/propertyAttributes.ts` was a hand copy of
  the frontend catalog + labels for the react-admin back office.

Every new enum option or field required editing three files in two languages by
hand, with no mechanism to catch drift. The Russian display labels lived only
in the TS stacks. The object xlsx export (ADR-companion of the wayfinder #50
spec) needed attribute characteristics rendered in Russian, but the backend had
no labels at all — that gap forced either a second hand-maintained label map in
Go or a new source of truth.

The decision was made in the wayfinder map for the property attributes catalog
and is recorded here because it fixes a code-generation contract across three
stacks that is expensive to reverse, and rejects two seemingly natural
alternatives (a shared `packages/` workspace, and codegen for labels only).

## Decision

### 1. One source: `tools/property-attributes/catalog.json`

A single normalized JSON document is the source of truth for the catalog. It is
organized by the 10 property types; each type declares its fields. A field
carries `kind` (`number` / `integer` / `string` / `enum`), `label`, and the
kind-specific constraints (`min`/`max`/`decimals`/`unit` for numbers, `maxLen`
for strings, `options` for enums), plus a `group`. Group labels and type labels
live alongside. Cross-field rules and the dynamic `year_built` ceiling are
expressed in the same file (see sections 5 and 7).

Enum options are stored as an **array** of `{ key, label }` objects, not as a
`key → label` map. This is load-bearing: V8 sorts integer-like string keys of
plain objects numerically, so `studio` would lose first place to `"1"` in the
rooms selector. An array preserves the authored UI order on every engine.

### 2. A code generator emits all three stacks from the catalog

`tools/property-attributes/generate.mjs` (Node, no framework, plus `ajv` for
schema validation) reads `catalog.json` and writes generated artifacts into
three stacks:

- **Go** — `apps/backend/internal/properties/domain/zz_catalog.gen.go`: the
  field definitions (`fieldDef`, `attrKind`), `ValidateAttributes`,
  `validateField`, `crossFieldErrors`, `CatalogKeys`, `FilterByType`, and the
  label helpers `PropertyTypeLabel`, `AttributeFieldLabel`,
  `AttributeEnumLabel`.
- **Frontend TS** — `apps/frontend/features/property-attributes/lib/generated/`
  (`attr-keys`, `catalog`, `labels`, `validate`): types `AttrKey`/`AttrField`,
  the `catalog`, label maps, `validateField`, `validateAttributes`,
  `filterByType`.
- **Admin TS** — `apps/admin/src/lib/generated/` (`types`, `attr-keys`,
  `catalog`, `labels`, `format`): self-contained types (`PropertyType`,
  `PropertyAttributes`, `AttrKey`), the `catalog`, label maps, and
  `formatAttributeValue`.

All generated files are committed and start with the banner
`// Code generated … DO NOT EDIT.`. The pre-existing hand-written modules
were reduced to the domain types that are not derivable from the catalog (for
Go: `Attributes`, `AttributeValidationError`, `ValidationResult` in
`attributes.go`; the TS stacks keep their entity type modules) plus re-exports
where existing import sites needed to keep compiling. The catalog, the field
maps, the label maps, and all validation logic now live only in generated code.

### 3. Placement in `tools/property-attributes/`, no Node workspaces

The generator is a standalone Node project under `tools/property-attributes/`,
not a `packages/` workspace. The repository has no Node workspaces today —
`apps/frontend` and `apps/admin` are independent npm projects — and introducing
workspaces for a single internal generator would be an infrastructure change
touching two production applications for no other benefit. The generator reads
`catalog.json` locally and writes into the apps over relative paths, mirroring
two existing patterns:

- `generate:api`, where the frontend reads `../backend/.../openapi.yaml` by
  relative path;
- `tools/screenshots/`, a separate Node project living under `tools/`.

A `packages/` shared workspace was rejected as too invasive for the value.

### 4. Codegen replaces the whole catalog and validation, not just labels

The generator emits field types, ranges, enum option sets, and cross-field
rules — not only the Russian labels. `validateField` and `crossFieldErrors`
are emitted, not hand-written. This closes the last drift surface: if only
labels were generated, the cross-field rules would still be duplicated across
the three stacks by hand.

### 5. A single-operator mini-DSL `lte` for cross-field rules

The current cross-field rules all have the form *field ≤ otherField*:
`floor ≤ floors_total`, `area_living ≤ area_total`,
`area_kitchen ≤ area_total`. They are recorded in `catalog.json` under `rules`
as `{ types, field, lte: "otherField" }`. A DSL with one operator covers every
existing rule. If a rule that is not a simple `lte` ever appears, it can be
added as a hand-written hook while the simple rules stay in the DSL — the
schema and generator do not need to grow a general expression language.

### 6. JSON Schema + `ajv` validation on generation

`catalog.schema.json` (JSON Schema draft 2020-12) describes the catalog
structure. The generator validates `catalog.json` against it with `ajv` before
emitting anything, so a typo in a `kind`, a missing required field, or a stray
option key fails generation before any code is written.

### 7. Dynamic `year_built` ceiling via a marker

`year_built` is bounded above by `current_year + 5` (mirroring Go
`maxYearBuilt()` and the TS `yearBuiltMax = new Date().getFullYear() + 5`).
Because a JSON literal cannot express "now", the file stores a snapshot number
(`max: 2031` at authoring time) plus the marker `yearBuiltMaxDynamic: true`.
The generator recomputes the ceiling as `current_year + 5` on every run; the
runtime may also compute it dynamically. The marker signals that the literal is
a snapshot, not an eternal constant.

### 8. CI gate `make attributes-check`

Following the `backend-tkassa-spec-check` precedent, `make attributes-check`
regenerates the artifacts and compares content hashes (`git hash-object`)
before and after. It fails if `catalog.json` was changed but the regenerated
artifacts were not committed. Content hashes (not `git status`) are used so the
gate also holds on a dirty tree with in-flight work; on CI's clean checkout a
hash change is exactly a status change.

### 9. Contract tests as the migration proof

The existing Go tests
(`apps/backend/internal/properties/domain/attributes_test.go`, 17 tests) were
kept unchanged and green after replacing the hand-written catalog with the
generated one — that is the evidence the behavior did not drift. The TS stacks
had no attribute tests before the migration: `vitest` was introduced into both
`apps/frontend` and `apps/admin`, 104 tests were written against the current
behavior *before* the migration, and the migration was done with those tests
kept green.

## Rejected alternatives

- **`packages/` shared workspace** (publish the catalog as an internal npm
  package consumed by frontend and admin): rejected — see section 3. It would
  force workspaces onto two production apps for one internal generator, and
  adds a publish/consume lifecycle the monorepo does not otherwise have.
- **Codegen for labels only** (keep the field maps and validation hand-written
  in each stack): rejected — see section 4. It leaves the cross-field rules and
  enum option sets duplicated by hand, which is exactly the drift the catalog
  was created to remove.
- **Hand-written label map on the backend** (don't generate Go labels; copy the
  Russian strings into Go by hand for the export): rejected — it recreates the
  three-place duplication for labels specifically and breaks the "edit once"
  property.

## Consequences

- **Positive.** One source of truth: changing the catalog is editing a single
  `catalog.json` and running `make attributes-gen`. Russian labels are now
  available on the backend, which is what lets the object xlsx export render
  type and attribute names in Russian via `PropertyTypeLabel`,
  `AttributeFieldLabel`, and `AttributeEnumLabel`. Type safety is preserved:
  `AttrKey` is generated as a string-literal union. CI catches drift via
  `make attributes-check`.
- **Negative / risk.** The generator is new code; a bug in emission is a
  regression in validation. The mitigations are the contract tests (sections 8
  and 9), the CI gate, and byte-for-byte idempotent output. Adding a new field
  or enum requires understanding the codegen schema (documented in
  `tools/property-attributes/README.md` and the three `AGENTS.md` files). The
  `year_built` ceiling is a static snapshot plus a dynamic marker, so the
  number in `catalog.json` is only refreshed when the generator runs.
- **Related rules.** `apps/backend/AGENTS.md`, `apps/frontend/AGENTS.md`, and
  `apps/admin/AGENTS.md` each carry the rule: the attributes catalog is
  generated from `tools/property-attributes/catalog.json`; regenerate with
  `make attributes-gen`; `make attributes-check` fails in CI if a
  `catalog.json` change was not committed with its regenerated artifacts; do
  not hand-edit `generated/` or `zz_catalog.gen.go`.

## See also

- ADR 0001 (DDD modular monolith — the generator crosses contexts only by
  writing into the `properties` domain package and the two TS apps, it does
  not add a new context).
- Source and generator: `tools/property-attributes/` (`README.md`,
  `catalog.json`, `catalog.schema.json`, `generate.mjs`).
- Updated spec: `docs/plans/wayfinder-export-property-xlsx-spec.md`
  (the export consumes the generated Go label helpers on the new "Объект"
  sheet).
- CI gates in `Makefile`: `attributes-gen`, `attributes-check`.
