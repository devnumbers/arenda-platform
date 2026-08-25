#!/usr/bin/env node
// generate.mjs — single-entry artifact generator for the payment-categories catalog.
//
// Reads catalog.json, validates it against catalog.schema.json (reusing the same
// ajv logic as validate-source.mjs, incl. the icon-asset existence check), and
// emits generated artifacts for two stacks WITHOUT touching any existing
// hand-written file (the admin app is out of scope for the payments effort):
//
//   Go:
//     apps/backend/internal/payments/domain/zz_categories.gen.go
//
//   Frontend TS:
//     apps/frontend/features/payment-categories/lib/generated/categories.ts
//
// This generator is idempotent: running it twice produces byte-identical files
// (the catalog is static data — no time-dependent literals).
//
// Usage:
//   node generate.mjs            # from tools/payment-categories/
//   npm run generate             # via package.json script
//   make categories-gen          # from the monorepo root

import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import { CATALOG_PATH, SCHEMA_PATH, readJson, validateCatalog } from './gen/validate.mjs';
import { gofmt } from './gen/gofmt.mjs';
import { emitGo } from './gen/emit-go.mjs';
import { emitFrontendTs } from './gen/emit-frontend-ts.mjs';

const here = dirname(fileURLToPath(import.meta.url));
// generate.mjs lives directly in tools/payment-categories/.
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
    () => writeArtifact(
      join(repoRoot, 'apps/backend/internal/payments/domain/zz_categories.gen.go'),
      gofmt(emitGo(catalog)),
    ),
    // Frontend TS.
    () => writeArtifact(
      join(repoRoot, 'apps/frontend/features/payment-categories/lib/generated/categories.ts'),
      emitFrontendTs(catalog),
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
