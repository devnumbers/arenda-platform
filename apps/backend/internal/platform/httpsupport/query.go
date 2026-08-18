package httpsupport

import (
	"context"
	"net/http"
)

// ListAdminItems runs a paginated admin list use case and maps the resulting
// views to API items. On error it delegates to handleError and reports ok as
// false. Exported because the admin and billing HTTP adapters, now in separate
// per-domain packages, both rely on the same generic list-and-map flow.
func ListAdminItems[V, I any](w http.ResponseWriter, r *http.Request, list func(ctx context.Context) ([]V, int64, error), handleError func(w http.ResponseWriter, r *http.Request, err error), toItem func(V) I) (items []I, total int, ok bool) {
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
