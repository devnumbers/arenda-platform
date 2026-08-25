#!/usr/bin/env node
// Validates catalog.json against catalog.schema.json using ajv (JSON Schema draft 2020-12),
// plus semantic checks (unique slugs, icon-asset existence). Exits 0 on success with a
// summary line, 1 on any validation or I/O error.

import { CATALOG_PATH, SCHEMA_PATH, readJson, validateCatalog } from './gen/validate.mjs';

const schema = readJson(SCHEMA_PATH);
const catalog = readJson(CATALOG_PATH);

const result = validateCatalog(catalog, schema);

if (result.valid) {
  console.log(result.summary);
  process.exit(0);
}

console.error('catalog.json: INVALID');
for (const err of result.errors) {
  const path = err.instancePath || '(root)';
  const prop = err.params?.missingProperty ? `/${err.params.missingProperty}` : '';
  const extra = err.params?.additionalProperty
    ? ` (additionalProperty: ${err.params.additionalProperty})`
    : '';
  console.error(`  ${path}${prop}${extra} — ${err.message}`);
}
process.exit(1);
