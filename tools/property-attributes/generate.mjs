#!/usr/bin/env node
// generate.mjs — single-entry artifact generator for the property-attributes catalog.
//
// Reads catalog.json, validates it against catalog.schema.json (reusing the same
// ajv logic as validate-source.mjs), and emits generated artifacts for three
// stacks WITHOUT touching any existing hand-written file:
//
//   Go:
//     apps/backend/internal/properties/domain/zz_catalog.gen.go
//
//   Frontend TS:
//     apps/frontend/features/property-attributes/lib/generated/{attr-keys,catalog,labels,validate}.ts
//
//   Admin TS:
//     apps/admin/src/lib/generated/{types,attr-keys,catalog,labels,format}.ts
//
// Integration (rewiring app imports / deleting hand-written duplicates) is a
// separate migration step (1.4b). This generator is idempotent: running it twice
// produces byte-identical files (modulo the year_built max bound, which recomputes
// current_year+5 on each run — so re-running across a year boundary will change
// that one literal; within the same year the output is stable).
//
// Usage:
//   node generate.mjs            # from tools/property-attributes/
//   npm run generate             # via package.json script

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { CATALOG_PATH, SCHEMA_PATH, readJson, validateCatalog } from './gen/validate.mjs';
import { gofmt } from './gen/gofmt.mjs';
import { emitGo } from './gen/emit-go.mjs';
import {
  emitAttrKeys as emitFeAttrKeys,
  emitCatalog as emitFeCatalog,
  emitLabels as emitFeLabels,
  emitValidate as emitFeValidate,
} from './gen/emit-frontend-ts.mjs';
import {
  emitTypes as emitAdminTypes,
  emitAttrKeys as emitAdminAttrKeys,
  emitCatalog as emitAdminCatalog,
  emitLabels as emitAdminLabels,
  emitFormat as emitAdminFormat,
} from './gen/emit-admin-ts.mjs';

const here = dirname(fileURLToPath(import.meta.url));
// generate.mjs lives directly in tools/property-attributes/.
const catalogDir = here;
// Monorepo root is the parent of tools/.
const repoRoot = join(catalogDir, '..', '..');

function rel(p) {
  return p.startsWith(repoRoot + '/') ? p.slice(repoRoot.length + 1) : p;
}

function writeArtifact(absPath, content) {
  mkdirSync(dirname(absPath), { recursive: true });
  writeFileSync(absPath, content, 'utf8');
  return { path: rel(absPath), bytes: Buffer.byteLength(content, 'utf8') };
}

function main() {
  const schema = readJson(SCHEMA_PATH);
  const catalog = readJson(CATALOG_PATH);

  const result = validateCatalog(catalog, schema);
  if (!result.valid) {
    console.error('catalog.json: INVALID — refusing to generate.');
    for (const err of result.errors) {
      const path = err.instancePath || '(root)';
      const prop = err.params.missingProperty ? `/${err.params.missingProperty}` : '';
      const extra = err.params.additionalProperty ? ` (additionalProperty: ${err.params.additionalProperty})` : '';
      console.error(`  ${path}${prop}${extra} — ${err.message}`);
    }
    process.exit(1);
  }
  console.log(result.summary);

  const targets = [
    // Go (optionally piped through gofmt for canonical formatting).
    //
    // The artifact is emitted straight as zz_catalog.gen.go, a complete,
    // self-contained module in package `domain` that owns the catalog symbols
    // (fieldDef, attrKind, the field maps, ValidateAttributes, …). The
    // hand-written attributes.go keeps only the domain types
    // (Attributes/AttributeValidationError/ValidationResult), so there is no
    // symbol collision. Earlier the artifact used a `.go.txt` suffix to keep it
    // out of the compiler while hand-written duplicates still existed; migration
    // step 1.4b removed those duplicates, so the file now ships as `.go`.
    () => writeArtifact(
      join(repoRoot, 'apps/backend/internal/properties/domain/zz_catalog.gen.go'),
      gofmt(emitGo(catalog)),
    ),
    // Frontend TS
    () => writeArtifact(
      join(repoRoot, 'apps/frontend/features/property-attributes/lib/generated/attr-keys.ts'),
      emitFeAttrKeys(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/frontend/features/property-attributes/lib/generated/catalog.ts'),
      emitFeCatalog(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/frontend/features/property-attributes/lib/generated/labels.ts'),
      emitFeLabels(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/frontend/features/property-attributes/lib/generated/validate.ts'),
      emitFeValidate(catalog),
    ),
    // Admin TS
    () => writeArtifact(
      join(repoRoot, 'apps/admin/src/lib/generated/types.ts'),
      emitAdminTypes(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/admin/src/lib/generated/attr-keys.ts'),
      emitAdminAttrKeys(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/admin/src/lib/generated/catalog.ts'),
      emitAdminCatalog(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/admin/src/lib/generated/labels.ts'),
      emitAdminLabels(catalog),
    ),
    () => writeArtifact(
      join(repoRoot, 'apps/admin/src/lib/generated/format.ts'),
      emitAdminFormat(catalog),
    ),
  ];

  const written = [];
  for (const t of targets) written.push(t());

  console.log('\nGenerated artifacts:');
  for (const w of written) {
    console.log(`  ${w.path}  (${w.bytes} bytes)`);
  }
  console.log(`\nDone: ${written.length} files.`);
}

main();
