package domain

import "encoding/json"

// Optional represents a JSON value that may be absent, null, or present.
// Set is true when the key was present in the JSON body (including null).
type Optional[T any] struct {
	Value T
	Set   bool
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		var zero T
		o.Value = zero
		return nil
	}
	return json.Unmarshal(data, &o.Value)
}
