// Thin re-export of the generated validation logic. The hand-written validators
// were replaced by the generated artifact in lib/generated/validate.ts (produced
// from tools/property-attributes/catalog.json by generate.mjs). This file keeps
// the historical import path (lib/validate) stable for consumers.
export * from './generated/validate';
