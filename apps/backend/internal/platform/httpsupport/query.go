package httpsupport

import (
	"context"
	"net/http"
)

// listAdminItems runs a paginated admin list use case and maps the resulting
// views to API items. On error it delegates to handleError and reports ok as
// false.
func listAdminItems[V, I any](w http.ResponseWriter, r *http.Request, list func(ctx context.Context) ([]V, int64, error), handleError func(w http.ResponseWriter, r *http.Request, err error), toItem func(V) I) (items []I, total int, ok bool) {
	views, totalCount, err := list(r.Context())
	if err != nil {
		handleError(w, r, err)
		return nil, 0, false
	}

	items = make([]I, 0, len(views))
	for _, v := range views {
		items = append(items, toItem(v))
	}

	return items, int(totalCount), true
}

// adminListEnvelope mirrors the generated per-entity {items, total} list
// response bodies. The JSON tags must stay identical to the oapi-codegen
// output (no omitempty), keeping the wire format byte-for-byte the same;
// query_test.go pins this against the generated types.
type adminListEnvelope[I any] struct {
	Items []I `json:"items"`
	Total int `json:"total"`
}

// RespondAdminList runs a paginated admin list use case, maps the resulting
// views to API items, and writes the {items, total} response body. On error
// the response is delegated to handleError.
func RespondAdminList[V, I any](w http.ResponseWriter, r *http.Request, list func(ctx context.Context) ([]V, int64, error), handleError func(w http.ResponseWriter, r *http.Request, err error), toItem func(V) I) {
	items, total, ok := listAdminItems(w, r, list, handleError, toItem)
	if !ok {
		return
	}

	WriteJSON(r.Context(), w, http.StatusOK, adminListEnvelope[I]{Items: items, Total: total})
}

// OptInt copies an optional integer query parameter into dst when set.
func OptInt(dst, src *int) {
	if src != nil {
		*dst = *src
	}
}

// OptString copies an optional string-ish query parameter into dst when set.
func OptString[T ~string](dst *string, src *T) {
	if src != nil {
		*dst = string(*src)
	}
}
