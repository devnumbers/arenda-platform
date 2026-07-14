# T-Kassa OpenAPI spec (vendored)

This package vendors the official T-Kassa (T-Business acquiring) OpenAPI
specification and the Go types generated from it. See ADR 0016
(`docs/adr/0016-tkassa-openapi-mapping.md`) for why the adapter is spec-derived.

## Upstream

- URL: <https://developer.tbank.ru/schemas/eacq/openapi.yaml>
- Vendored version: 1.21 (verify: `grep '^  version:' openapi.yaml`)
- `openapi.yaml` is byte-identical to upstream — never edit it by hand.
  Verify with:

  ```bash
  curl -fsSL https://developer.tbank.ru/schemas/eacq/openapi.yaml | diff - openapi.yaml
  ```

## Why the spec is patched

`openapi.patched.yaml` is produced from `openapi.yaml` by `./patch.sh` and is
the input to code generation. Two upstream bugs are fixed:

1. **`Common.additionalProperties` is nested inside `properties:`** — it is
   declared as a regular property named `additionalProperties` instead of a
   schema-level keyword, so oapi-codegen emits a bogus
   `AdditionalProperties *string` field. After the fix `spec.Common` carries a
   real `map[string]string` additional-properties map (used for `DATA` in
   `Init`).
2. **`Amount` is `type: number` in the `Init` and `Cancel` request schemas** —
   oapi-codegen maps that to `float32`, which loses integer precision above
   ~16.7M kopecks. The patch changes only those two `Amount` properties to
   `type: integer` + `format: int64`. The backend stores money as `BIGINT`
   kopecks (ADR 0008), so request amounts must be `int64`.

`patch.sh` fails loudly when an expected upstream pattern is missing, so a
re-vendored spec that already contains the fix (or changed shape) cannot be
patched silently.

## Re-vendor procedure

Requires `python3` on the PATH — `patch.sh` applies the spec fixes with an
embedded Python script.

From this directory:

```bash
# 1. Download the new upstream spec.
curl -fsSL https://developer.tbank.ru/schemas/eacq/openapi.yaml -o openapi.yaml
# 2. Apply the local fixes (fails if a pattern no longer matches).
./patch.sh
# 3. Regenerate the Go types.
go generate ./...
# 4. Run the billing tests.
cd ../../../../.. && go test ./internal/billing/...
```

If step 2 fails because upstream fixed one of the bugs, remove the
corresponding fix from `patch.sh` and regenerate.

## Files

- `openapi.yaml` — pristine upstream spec (do not edit).
- `patch.sh` — idempotent patch script (`openapi.yaml` → `openapi.patched.yaml`).
- `openapi.patched.yaml` — patched spec, input to code generation (committed).
- `generate.go` — `//go:generate` directive (oapi-codegen v2.7.1, types only).
- `spec.gen.go` — generated types (do not edit by hand).
