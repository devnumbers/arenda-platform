package domain

import (
	"fmt"
	"slices"
	"sort"
	"time"
	"unicode/utf8"
)

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

// attrKind classifies a catalog field's value type.
type attrKind int

const (
	kindNumber attrKind = iota
	kindInteger
	kindString
	kindEnum
)

// fieldDef internally describes each catalog field.
type fieldDef struct {
	kind     attrKind
	minFloat float64  // for kindNumber
	maxFloat float64  // for kindNumber
	minInt   int      // for kindInteger
	maxInt   int      // for kindInteger
	maxLen   int      // for kindString
	enumVals []string // for kindEnum (stored values, e.g. "studio")
}

// maxYearBuilt returns the upper bound (inclusive) for the year_built field:
// the current year + 5.
func maxYearBuilt() int {
	return time.Now().UTC().Year() + 5
}

var (
	apartmentFields = map[string]fieldDef{
		"rooms": {
			kind:     kindEnum,
			enumVals: []string{"studio", "1", "2", "3", "4", "5", "6", "7_plus"},
		},
		"area_total":     {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"area_living":    {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"area_kitchen":   {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"floor":          {kind: kindInteger, minInt: -3, maxInt: 200},
		"floors_total":   {kind: kindInteger, minInt: 1, maxInt: 200},
		"bathroom":       {kind: kindEnum, enumVals: []string{"combined", "separate", "multiple"}},
		"balcony":        {kind: kindEnum, enumVals: []string{"none", "balcony", "loggia", "balcony_and_loggia"}},
		"renovation":     {kind: kindEnum, enumVals: []string{"cosmetic", "euro", "design", "required"}},
		"year_built":     {kind: kindInteger, minInt: 1800, maxInt: maxYearBuilt()},
		"ceiling_height": {kind: kindNumber, minFloat: 2.0, maxFloat: 10.0},
		"parking_type":   {kind: kindEnum, enumVals: []string{"closed", "underground", "open"}},
	}

	roomFields = map[string]fieldDef{
		"rooms": {
			kind:     kindEnum,
			enumVals: []string{"2", "3", "4", "5", "6", "7_plus"},
		},
		"area_total":   {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"area_kitchen": {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"floor":        {kind: kindInteger, minInt: -3, maxInt: 200},
		"floors_total": {kind: kindInteger, minInt: 1, maxInt: 200},
		"bathroom":     {kind: kindEnum, enumVals: []string{"combined", "separate", "multiple"}},
		"balcony":      {kind: kindEnum, enumVals: []string{"none", "balcony", "loggia", "balcony_and_loggia"}},
		"renovation":   {kind: kindEnum, enumVals: []string{"cosmetic", "euro", "design", "required"}},
		"year_built":   {kind: kindInteger, minInt: 1800, maxInt: maxYearBuilt()},
	}

	houseFields = map[string]fieldDef{
		"land_area":    {kind: kindNumber, minFloat: 0.01, maxFloat: 1000000.0},
		"land_type":    {kind: kindEnum, enumVals: []string{"izhs", "garden", "farm"}},
		"area_total":   {kind: kindNumber, minFloat: 1.0, maxFloat: 100000.0},
		"floors_total": {kind: kindInteger, minInt: 1, maxInt: 200},
		"rooms":        {kind: kindEnum, enumVals: []string{"1", "2", "3", "4", "5", "6", "7_plus"}},
		"house_type":   {kind: kindEnum, enumVals: []string{"detached", "part", "townhouse", "duplex"}},
		"material": {
			kind: kindEnum,
			enumVals: []string{
				"brick", "monolithic", "panel", "brick_monolithic",
				"block", "wooden", "reinforced_concrete",
			},
		},
		"bathroom":   {kind: kindEnum, enumVals: []string{"indoor", "outdoor", "none"}},
		"shower":     {kind: kindEnum, enumVals: []string{"indoor", "outdoor", "none"}},
		"year_built": {kind: kindInteger, minInt: 1800, maxInt: maxYearBuilt()},
	}
)

// fieldsForType returns the catalog field map for the given property type.
// apartment and apartments share the same map; room and house have their own.
// Returns nil for unsupported types (not yet in the catalog).
func fieldsForType(t PropertyType) map[string]fieldDef {
	switch t {
	case PropertyTypeApartment, PropertyTypeApartments:
		return apartmentFields
	case PropertyTypeRoom:
		return roomFields
	case PropertyTypeHouse:
		return houseFields
	default:
		// Unsupported types (commercial, office, warehouse, garage, parking,
		// land) are added in a follow-up; nil means "no catalog yet".
		return nil
	}
}

// CatalogKeys returns the set of catalog attribute keys for the given property
// type. For unsupported types (not yet in the catalog) returns nil.
func CatalogKeys(propType PropertyType) []string {
	fields := fieldsForType(propType)
	if fields == nil {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FilterByType returns a copy of attrs containing only keys that belong to the
// catalog of the given property type. Unknown keys are silently dropped. This
// implements the "current type determines the meaning of stored keys" read
// semantics: when the type changes, keys of the old type become inert.
func (a Attributes) FilterByType(propType PropertyType) Attributes {
	fields := fieldsForType(propType)
	out := make(Attributes, len(a))
	for k, v := range a {
		if _, ok := fields[k]; ok {
			out[k] = v
		}
	}
	return out
}

// ValidateAttributes validates the given attributes against the catalog for the
// given property type. Only keys belonging to the catalog of the current type
// are accepted; unknown keys and null values are rejected. Numeric ranges,
// enum membership, and cross-field rules are checked. Only the four residential
// types (apartment, apartments, room, house) are supported here; all other
// types yield a valid result for an empty set (they are added in a follow-up).
func ValidateAttributes(propType PropertyType, attrs Attributes) ValidationResult {
	var res ValidationResult
	fields := fieldsForType(propType)

	// Unsupported type: every key is unknown. Empty attrs => valid (vacuous).
	if fields == nil {
		if len(attrs) == 0 {
			return res
		}
		keys := make([]string, 0, len(attrs))
		for k := range attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			res.Errors = append(res.Errors, AttributeValidationError{
				Field:  k,
				Reason: fmt.Sprintf("unknown attribute for property type %s", propType),
			})
		}
		return res
	}

	// Deterministic ordering: sorted slice of catalog keys, then unknown keys.
	catKeys := make([]string, 0, len(fields))
	for k := range fields {
		catKeys = append(catKeys, k)
	}
	sort.Strings(catKeys)

	// Validate present catalog fields first.
	for _, k := range catKeys {
		v, present := attrs[k]
		if !present {
			continue
		}
		if v == nil {
			res.Errors = append(res.Errors, AttributeValidationError{
				Field:  k,
				Reason: "attribute value must not be null",
			})
			continue
		}
		def := fields[k]
		if reason := validateField(def, v); reason != "" {
			res.Errors = append(res.Errors, AttributeValidationError{
				Field:  k,
				Reason: reason,
			})
		}
	}

	// Report unknown keys (present in attrs but not in the catalog) in sorted order.
	unknownKeys := make([]string, 0)
	for k := range attrs {
		if _, ok := fields[k]; !ok {
			unknownKeys = append(unknownKeys, k)
		}
	}
	sort.Strings(unknownKeys)
	for _, k := range unknownKeys {
		res.Errors = append(res.Errors, AttributeValidationError{
			Field:  k,
			Reason: fmt.Sprintf("unknown attribute for property type %s", propType),
		})
	}

	// Cross-field validations (only when BOTH sides are present).
	res.Errors = append(res.Errors, crossFieldErrors(propType, fields, attrs)...)

	return res
}

// validateField checks a single present field against its definition and returns
// a non-empty reason string on failure, or "" when valid.
func validateField(def fieldDef, v any) string {
	switch def.kind {
	case kindNumber:
		f, ok := v.(float64)
		if !ok {
			return "expected a number"
		}
		if f < def.minFloat || f > def.maxFloat {
			return fmt.Sprintf("must be between %g and %g", def.minFloat, def.maxFloat)
		}
	case kindInteger:
		f, ok := v.(float64)
		if !ok {
			return "expected an integer"
		}
		if f != float64(int64(f)) {
			return "expected an integer"
		}
		i := int(f)
		if i < def.minInt || i > def.maxInt {
			return fmt.Sprintf("must be between %d and %d", def.minInt, def.maxInt)
		}
	case kindString:
		s, ok := v.(string)
		if !ok {
			return "expected a string"
		}
		if utf8.RuneCountInString(s) > def.maxLen {
			return fmt.Sprintf("must be at most %d characters", def.maxLen)
		}
	case kindEnum:
		s, ok := v.(string)
		if !ok {
			return "expected a string"
		}
		if !slices.Contains(def.enumVals, s) {
			return fmt.Sprintf("must be one of %v", def.enumVals)
		}
	}
	return ""
}

// crossFieldErrors returns errors for cross-field rules, evaluated only when
// BOTH sides of a comparison are present and individually valid.
func crossFieldErrors(propType PropertyType, fields map[string]fieldDef, attrs Attributes) []AttributeValidationError {
	var errs []AttributeValidationError

	numVal := func(key string) (float64, bool) {
		def, inCatalog := fields[key]
		if !inCatalog {
			return 0, false
		}
		v, present := attrs[key]
		if !present || v == nil {
			return 0, false
		}
		if def.kind != kindNumber && def.kind != kindInteger {
			return 0, false
		}
		f, ok := v.(float64)
		if !ok {
			return 0, false
		}
		// Only consider the value valid if it passes its own range/type check,
		// so a cross-field error isn't produced on top of a field-level error.
		if reason := validateField(def, v); reason != "" {
			return 0, false
		}
		return f, true
	}

	switch propType {
	case PropertyTypeApartment, PropertyTypeApartments:
		if f, ok := numVal("floor"); ok {
			if ft, ok2 := numVal("floors_total"); ok2 && f > ft {
				errs = append(errs, AttributeValidationError{
					Field:  "floor",
					Reason: "floor must not exceed floors_total",
				})
			}
		}
		if al, ok := numVal("area_living"); ok {
			if at, ok2 := numVal("area_total"); ok2 && al > at {
				errs = append(errs, AttributeValidationError{
					Field:  "area_living",
					Reason: "area_living must not exceed area_total",
				})
			}
		}
		if ak, ok := numVal("area_kitchen"); ok {
			if at, ok2 := numVal("area_total"); ok2 && ak > at {
				errs = append(errs, AttributeValidationError{
					Field:  "area_kitchen",
					Reason: "area_kitchen must not exceed area_total",
				})
			}
		}
	case PropertyTypeRoom:
		if f, ok := numVal("floor"); ok {
			if ft, ok2 := numVal("floors_total"); ok2 && f > ft {
				errs = append(errs, AttributeValidationError{
					Field:  "floor",
					Reason: "floor must not exceed floors_total",
				})
			}
		}
		if ak, ok := numVal("area_kitchen"); ok {
			if at, ok2 := numVal("area_total"); ok2 && ak > at {
				errs = append(errs, AttributeValidationError{
					Field:  "area_kitchen",
					Reason: "area_kitchen must not exceed area_total",
				})
			}
		}
	default:
		// Unsupported types have no cross-field rules yet.
	}

	return errs
}
