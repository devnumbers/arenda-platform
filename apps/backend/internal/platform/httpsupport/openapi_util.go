package httpsupport

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// DatePtrToOpenAPI converts a *time.Time into the generated openapi Date value
// pointer used by response DTOs. A nil input yields a nil pointer so omitted
// optional dates stay absent from the wire payload.
func DatePtrToOpenAPI(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	return &openapi_types.Date{Time: *t}
}

// PtrString converts a pointer to a constrained string type (such as a
// generated openapi enum alias) into a plain *string. A nil input yields nil so
// omitted optional fields stay absent from the application command.
func PtrString[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}
