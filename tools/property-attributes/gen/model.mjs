// Derived, canonical data extracted from catalog.json. Every emitter (Go/TS)
// consumes this, so field ordering, enum ordering and rule grouping live in ONE
// place and are applied identically across all generated stacks.

// The canonical type ordering mirrors the Go PropertyType constants and the TS
// PropertyType union order. This is the order used to declare field maps.
export const TYPE_ORDER = [
  'apartment',
  'room',
  'apartments',
  'studio',
  'house',
  'office',
  'commercial',
  'warehouse',
  'garage',
  'parking',
  'land',
];

// Which catalog type is the "primary" that an alias mirrors.
// apartments -> apartment, commercial -> office.
// (matches Go: commercialFields = officeFields; TS: catalog.apartments = apartmentFields)
export const TYPE_ALIASES = {
  apartments: 'apartment',
  commercial: 'office',
};

export function aliasOf(typeKey) {
  return TYPE_ALIASES[typeKey] ?? typeKey;
}

// Fields for a (non-alias) type, preserving JSON insertion order (the UI order).
// Each entry: { key, def } where def is the raw catalog field object plus a
// normalised `kind`/params view.
export function fieldsFor(catalog, typeKey) {
  const real = aliasOf(typeKey);
  const rawFields = catalog.types[real].fields;
  return Object.entries(rawFields).map(([key, def]) => ({ key, def }));
}

// Enum option values for an enum field, in array order.
// Matches the existing hand-written slices in attributes.go (e.g. rooms studio,1,2,...)
// and the TS options arrays.
//
// catalog.json stores enum options as an ARRAY of { key, label } objects (not an
// object map) because V8's JSON.parse reorders integer-like object keys ascending,
// which would break the canonical UI order (e.g. studio must come first for
// apartment.rooms). Arrays preserve order.
export function enumOptionKeys(fieldDef) {
  return fieldDef.options.map((o) => o.key);
}

// All distinct field keys across the whole catalog, in stable order:
// the order of first appearance when iterating types in TYPE_ORDER then fields
// in insertion order. This becomes the AttrKey union / fieldLabels map order.
export function allFieldKeys(catalog) {
  const seen = new Set();
  const out = [];
  for (const t of TYPE_ORDER) {
    const real = aliasOf(t);
    for (const key of Object.keys(catalog.types[real].fields)) {
      if (!seen.has(key)) {
        seen.add(key);
        out.push(key);
      }
    }
  }
  return out;
}

// Build a single merged enumLabels map per field key (the union of all options
// seen for that key across types). Mirrors the hand-written labels.ts/enumLabels
// where a field like `bathroom` aggregates options from apartment+house.
//
// catalog.json stores enum options as an ARRAY of { key, label } objects; we walk
// it in order and keep first-seen label wins for each option key.
export function mergedEnumOptions(catalog) {
  const map = new Map(); // fieldKey -> Map(optionKey -> label)
  for (const t of TYPE_ORDER) {
    const real = aliasOf(t);
    for (const [fkey, fdef] of Object.entries(catalog.types[real].fields)) {
      if (fdef.kind !== 'enum') continue;
      if (!map.has(fkey)) map.set(fkey, new Map());
      const opts = map.get(fkey);
      for (const { key: optKey, label: optLabel } of fdef.options) {
        if (!opts.has(optKey)) opts.set(optKey, optLabel);
      }
    }
  }
  return map;
}

// Group rules by field (the LHS of the `lte` comparison). Each rule becomes a
// cross-field check. Returns [{ field, lte, types: [...] }] preserving catalog order.
export function rulesGrouped(catalog) {
  return catalog.rules.map((r) => ({ field: r.field, lte: r.lte, types: r.types }));
}
