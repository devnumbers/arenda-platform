// Thin re-export of the generated labels. The hand-written label maps were
// replaced by the generated artifact in lib/generated/labels.ts (produced from
// tools/property-attributes/catalog.json by generate.mjs). This file keeps the
// historical import path (lib/labels) stable for consumers.
export * from './generated/labels';
