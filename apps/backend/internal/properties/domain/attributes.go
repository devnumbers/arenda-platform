package domain

// Attributes is a set of per-type property characteristics.
type Attributes map[string]any

// AttributeValidationError is a single validation failure bound to a field.
type AttributeValidationError struct {
	Field  string
	Reason string
}

// ValidationResult aggregates field-level and cross-field validation errors.
type ValidationResult struct {
	Errors []AttributeValidationError
}

// Valid returns true when there are no validation errors.
func (r ValidationResult) Valid() bool { return len(r.Errors) == 0 }

// The catalog field maps, type classifications (fieldDef/attrKind), validation
// functions (ValidateAttributes, validateField, crossFieldErrors), and helpers
// (CatalogKeys, FilterByType, fieldsForType, maxYearBuilt) live in the
// generated artifact zz_catalog.gen.go, produced from
// tools/property-attributes/catalog.json by generate.mjs. Do not edit those
// here; regenerate instead.
