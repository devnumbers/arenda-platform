#!/usr/bin/env node
// Validates catalog.json against catalog.schema.json using ajv (JSON Schema draft 2020-12).
// Exits 0 on success with a summary line, 1 on any validation or I/O error.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const __dirname = dirname(fileURLToPath(import.meta.url));
const catalogPath = join(__dirname, 'catalog.json');
const schemaPath = join(__dirname, 'catalog.schema.json');

function readJson(path) {
  let raw;
  try {
    raw = readFileSync(path, 'utf8');
  } catch (err) {
    console.error(`Cannot read ${path}: ${err.message}`);
    process.exit(1);
  }
  try {
    return JSON.parse(raw);
  } catch (err) {
    console.error(`Cannot parse JSON in ${path}: ${err.message}`);
    process.exit(1);
  }
}

const schema = readJson(schemaPath);
const catalog = readJson(catalogPath);

const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);

const validate = ajv.compile(schema);

const ajvValid = validate(catalog);

// Semantic check not expressible in JSON Schema without ajv-keywords:
// enum option `key` values must be unique within a single field's options
// array. (JSON Schema's uniqueItems compares whole objects, not a single
// property.)
const dupErrors = checkEnumOptionKeyUniqueness(catalog);

if (ajvValid && dupErrors.length === 0) {
  const typeCount = Object.keys(catalog.types).length;
  let fieldCount = 0;
  for (const type of Object.values(catalog.types)) {
    fieldCount += Object.keys(type.fields).length;
  }
  console.log(`catalog.json: OK (${typeCount} типов, ${fieldCount} полей)`);
  process.exit(0);
} else {
  console.error('catalog.json: INVALID');
  for (const err of validate.errors ?? []) {
    const path = err.instancePath || '(root)';
    const prop = err.params.missingProperty ? `/${err.params.missingProperty}` : '';
    const extra = err.params.additionalProperty ? ` (additionalProperty: ${err.params.additionalProperty})` : '';
    console.error(`  ${path}${prop}${extra} — ${err.message}`);
  }
  for (const err of dupErrors) {
    console.error(`  ${err.instancePath} — ${err.message}`);
  }
  process.exit(1);
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
            message: `duplicate enum option key ${JSON.stringify(k)} in field ${fieldKey} of type ${typeKey}`,
          });
        } else {
          seen.add(k);
        }
      }
    }
  }
  return errs;
}
