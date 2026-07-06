package domain

import "encoding/json"

// Optional represents a JSON value that may be absent, null, or present.
// Set is true when the key was present in the JSON body (including null).
type Optional[T any] struct {
	Value T
	Set   bool
}

func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.Set {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}
