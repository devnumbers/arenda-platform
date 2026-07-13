#!/bin/sh
# patch.sh — apply local fixes to the vendored T-Kassa OpenAPI spec.
#
# Reads openapi.yaml (byte-identical to the upstream spec) and writes
# openapi.patched.yaml with exactly two fixes. The script is idempotent: it
# always regenerates openapi.patched.yaml from openapi.yaml, and it fails
# loudly when an expected upstream pattern is missing (e.g. after a re-vendor
# where upstream fixed the bug), so the patch set never silently drifts.
#
# See README.md for the re-vendor procedure and ADR 0016 for the rationale.
set -eu

cd "$(dirname "$0")"

python3 - <<'PY'
import re
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


# --- Fix 2: Amount precision in the Init and Cancel request schemas.
# Upstream declares Amount as `type: number`, which oapi-codegen maps to
# float32 — that loses integer precision above ~16.7M kopecks. Money is
# BIGINT kopecks everywhere in the backend, so Amount must be int64.
# Only the `Amount` property of the Init and Cancel schemas is changed; the
# same-named properties elsewhere (responses, receipt items) keep the
# upstream type.
def patch_amount(schema, text):
    m = re.search(rf"^    {schema}:$", text, re.MULTILINE)
    if m is None:
        sys.exit(f"patch.sh: {schema} schema not found")
    start = m.start()
    nxt = re.search(r"^    [A-Z][A-Za-z0-9]*:$", text[start + 1:], re.MULTILINE)
    end = start + 1 + nxt.start() if nxt else len(text)
    block = text[start:end]

    prop = re.search(r"^(        Amount:\n(?:          .*\n)*)", block, re.MULTILINE)
    if prop is None:
        sys.exit(f"patch.sh: Amount property not found in {schema} schema")
    prop_text = prop.group(1)
    if "\n          type: number\n" not in "\n" + prop_text:
        sys.exit(f"patch.sh: {schema}.Amount is not `type: number` — "
                 "upstream may have fixed it; review this patch")
    fixed = prop_text.replace("          type: number\n",
                              "          type: integer\n          format: int64\n", 1)
    return text[:start] + block.replace(prop_text, fixed, 1) + text[end:]


text = patch_amount("Init", text)
text = patch_amount("Cancel", text)

# --- Post-condition assertions on the patched output.
assert common_good in text, "patched Common schema missing"
assert common_bad not in text, "buggy Common schema still present"
for schema in ("Init", "Cancel"):
    m = re.search(rf"^    {schema}:$", text, re.MULTILINE)
    start = m.start()
    nxt = re.search(r"^    [A-Z][A-Za-z0-9]*:$", text[start + 1:], re.MULTILINE)
    end = start + 1 + nxt.start() if nxt else len(text)
    block = text[start:end]
    prop = re.search(r"^(        Amount:\n(?:          .*\n)*)", block, re.MULTILINE)
    assert prop is not None, f"{schema}.Amount missing after patch"
    assert "type: integer" in prop.group(1), f"{schema}.Amount not integer"
    assert "format: int64" in prop.group(1), f"{schema}.Amount not int64"

with open(DST, "w", encoding="utf-8") as f:
    f.write(text)

print(f"patch.sh: wrote {DST} (Common additionalProperties, Init.Amount and "
      "Cancel.Amount fixed)")
PY
