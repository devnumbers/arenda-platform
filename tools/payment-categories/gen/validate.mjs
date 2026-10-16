// Shared validation logic: validate catalog.json against catalog.schema.json via ajv.
// Used by both validate-source.mjs and generate.mjs so they stay in sync.
//
// Exits the process (code 1) on I/O or parse errors when `exitOnError` is true.
// Returns { valid, errors, summary } when exitOnError is false.

import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const here = dirname(fileURLToPath(import.meta.url));
// gen/ is a subdir of the catalog dir.
const catalogDir = join(here, '..');
export const CATALOG_PATH = join(catalogDir, 'catalog.json');
export const SCHEMA_PATH = join(catalogDir, 'catalog.schema.json');
// Monorepo root is the parent of tools/ — needed for the icon-asset existence check.
export const REPO_ROOT = join(catalogDir, '..', '..');
export const ICON_ASSETS_DIR = join(
  REPO_ROOT,
  'apps/frontend/shared/assets/icons',
);

export function readJson(path, exitOnError = true) {
  let raw;
  try {
    raw = readFileSync(path, 'utf8');
  } catch (err) {
    if (!exitOnError) throw err;
    console.error(`Cannot read ${path}: ${err.message}`);
    process.exit(1);
  }
  try {
    return JSON.parse(raw);
  } catch (err) {
    if (!exitOnError) throw err;
    console.error(`Cannot parse JSON in ${path}: ${err.message}`);
    process.exit(1);
  }
}

// Compile + validate. Returns { valid, errors, summary, catalog }.
export function validateCatalog(catalog, schema) {
  const ajv = new Ajv2020({ allErrors: true, strict: true });
  addFormats(ajv);
  const validate = ajv.compile(schema);
  const valid = validate(catalog);
  let errors = valid ? [] : validate.errors ?? [];

  // Semantic checks not expressible in JSON Schema. Run them regardless of the
  // ajv result so dup-slug / missing-asset errors surface even when the shape
  // otherwise validates.
  errors = errors.concat(checkUniqueSlugs(catalog));
  errors = errors.concat(checkIconAssets(catalog));

  const allValid = valid && errors.length === 0;
  const cats = catalog.categories ?? [];
  return {
    valid: allValid,
    errors,
    summary: `catalog.json: OK (${cats.length} категорий)`,
  };
}

// Returns ajv-style errors for duplicate category slugs. JSON Schema's
// uniqueItems compares whole objects, not a single property, so uniqueness of
// `slug` across the array is checked here.
function checkUniqueSlugs(catalog) {
  const errs = [];
  const seen = new Map();
  for (const [i, cat] of (catalog.categories ?? []).entries()) {
    const slug = cat?.slug;
    if (slug === undefined) continue;
    if (seen.has(slug)) {
      errs.push({
        instancePath: `/categories/${i}`,
        keyword: 'uniqueSlugs',
        message: `duplicate category slug ${JSON.stringify(slug)} (first seen at index ${seen.get(slug)})`,
        params: { duplicateSlug: slug },
      });
    } else {
      seen.set(slug, i);
    }
  }
  return errs;
}

// Returns ajv-style errors for catalog icons that have no SVG asset in
// apps/frontend/shared/assets/icons/. The catalog is the map between category
// and icon asset; a slug pointing at a missing file would break the UI silently.
function checkIconAssets(catalog) {
  const errs = [];
  const cats = catalog.categories ?? [];
  const entries = cats.map((c) => ({ icon: c?.icon, at: `categories/${c?.slug ?? '?'}` }));
  if (catalog.userCategoryDefault?.icon !== undefined) {
    entries.push({ icon: catalog.userCategoryDefault.icon, at: 'userCategoryDefault' });
  }
  for (const { icon, at } of entries) {
    if (icon === undefined) continue;
    if (!existsSync(join(ICON_ASSETS_DIR, `${icon}.svg`))) {
      errs.push({
        instancePath: `/${at}/icon`,
        keyword: 'iconAssetExists',
        message: `icon asset apps/frontend/shared/assets/icons/${icon}.svg not found (export it from Figma and add to index.ts)`,
        params: { missingIcon: icon },
      });
    }
  }
  return errs;
}
