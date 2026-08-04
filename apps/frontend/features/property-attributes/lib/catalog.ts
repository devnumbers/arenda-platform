// Thin re-export of the generated catalog. The hand-written catalog definition
// was replaced by the generated artifact in lib/generated/catalog.ts (produced
// from tools/property-attributes/catalog.json by generate.mjs). This file keeps
// the historical import path (lib/catalog) stable for consumers.
export * from './generated/catalog';
