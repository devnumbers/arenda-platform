#!/bin/sh
# patch.sh — apply local fixes to the vendored T-Kassa OpenAPI spec.
#
# Reads openapi.yaml (byte-identical to the upstream spec) and writes
# openapi.patched.yaml with exactly one fix. The script is idempotent: it
# always regenerates openapi.patched.yaml from openapi.yaml, and it fails
# loudly when an expected upstream pattern is missing (e.g. after a re-vendor
# where upstream fixed the bug), so the patch set never silently drifts.
#
# History: until spec 1.21 a second fix patched `Amount` in the Init and
# Cancel request schemas from `type: number` to `integer/int64`; upstream
# fixed both Amount declarations in 1.27, so that patch was removed.
#
# See README.md for the re-vendor procedure and ADR 0016 for the rationale.
set -eu

cd "$(dirname "$0")"

python3 - <<'PY'
import sys

SRC = "openapi.yaml"
DST = "openapi.patched.yaml"

with open(SRC, encoding="utf-8") as f:
    text = f.read()

# --- Fix 1: Common.additionalProperties is wrongly nested inside properties:.
# Upstream declares it as a regular property named "additionalProperties"
# instead of a schema-level keyword, so oapi-codegen emits a bogus
# `AdditionalProperties *string` field instead of a free-form map. Move it to
# schema level (sibling of properties:).
common_bad = """    Common:
      type: object
      properties:
        additionalProperties:
          type: string
"""
common_good = """    Common:
      type: object
      additionalProperties:
        type: string
      properties:
"""
if common_bad not in text:
    sys.exit("patch.sh: Common schema pattern not found — upstream may have "
             "changed; review this patch")
if text.count(common_bad) != 1:
    sys.exit("patch.sh: Common schema pattern is not unique")
text = text.replace(common_bad, common_good)

# --- Post-condition assertions on the patched output.
assert common_good in text, "patched Common schema missing"
assert common_bad not in text, "buggy Common schema still present"

with open(DST, "w", encoding="utf-8") as f:
    f.write(text)

print(f"patch.sh: wrote {DST} (Common additionalProperties fixed)")
PY
