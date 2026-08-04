// Thin re-export of the generated attribute artifacts.
//
// The hand-written copy of the catalog/labels/format logic for the admin app
// was replaced by generated artifacts in lib/generated/ (produced from
// tools/property-attributes/catalog.json by generate.mjs). This file keeps the
// historical import path (./lib/propertyAttributes) stable for consumers such
// as src/properties.tsx and the propertyAttributes tests.

export * from './generated/types';
export * from './generated/attr-keys';
export * from './generated/catalog';
export * from './generated/labels';
export * from './generated/format';
