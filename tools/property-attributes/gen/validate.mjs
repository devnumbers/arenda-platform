// Shared validation logic: validate catalog.json against catalog.schema.json via ajv.
// Used by both validate-source.mjs and generate.mjs so they stay in sync.
//
// Exits the process (code 1) on I/O or parse errors when `exitOnError` is true.
// Returns { valid, errors, summary } when exitOnError is false.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const here = dirname(fileURLToPath(import.meta.url));
// gen/ is a subdir of the catalog dir.
const catalogDir = join(here, '..');
export const CATALOG_PATH = join(catalogDir, 'catalog.json');
export const SCHEMA_PATH = join(catalogDir, 'catalog.schema.json');

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

  // Semantic checks not expressible in JSON Schema without ajv-keywords:
  // enum option `key` values must be unique within a single field's options
  // array. (JSON Schema's uniqueItems compares whole objects, not a single
  // property.) Run this regardless of the ajv result so we surface dup-key
  // errors even when the shape otherwise validates.
  const dupErrors = checkEnumOptionKeyUniqueness(catalog);
  errors = errors.concat(dupErrors);

  const allValid = valid && dupErrors.length === 0;
  const typeCount = Object.keys(catalog.types).length;
  let fieldCount = 0;
  for (const type of Object.values(catalog.types)) {
    fieldCount += Object.keys(type.fields).length;
  }
  return {
    valid: allValid,
    errors,
    summary: `catalog.json: OK (${typeCount} типов, ${fieldCount} полей)`,
  };
}

// Returns an array of ajv-style error objects for enum fields whose options
// array contains duplicate `key` values. Empty array when all keys are unique.
function checkEnumOptionKeyUniqueness(catalog) {
  const errs = [];
  for (const [typeKey, typeDef] of Object.entries(catalog.types ?? {})) {
    for (const [fieldKey, fdef] of Object.entries(typeDef.fields ?? {})) {
      if (fdef.kind !== 'enum' || !Array.isArray(fdef.options)) continue;
      const seen = new Set();
      for (const opt of fdef.options) {
        const k = opt?.key;
        if (k === undefined) continue;
        if (seen.has(k)) {
          errs.push({
            instancePath: `/types/${typeKey}/fields/${fieldKey}/options`,
            schemaPath: '#/$defs/fieldDef',
            keyword: 'uniqueOptionKeys',
            message: `duplicate enum option key ${JSON.stringify(k)} in field ${fieldKey} of type ${typeKey}`,
            params: { duplicateOptionKey: k, field: fieldKey, type: typeKey },
          });
        } else {
          seen.add(k);
        }
      }
    }
  }
  return errs;
}
