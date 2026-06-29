package domain

import (
	"encoding/json"
	"testing"
)

func TestOptional_UnmarshalJSON(t *testing.T) {
	t.Run("absent field stays unset", func(t *testing.T) {
		var payload struct {
			Name Optional[string] `json:"name"`
		}
		if err := json.Unmarshal([]byte(`{}`), &payload); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Name.Set {
			t.Fatalf("expected Set=false for absent field, got true")
		}
		if payload.Name.Value != "" {
			t.Fatalf("expected zero value, got %q", payload.Name.Value)
		}
	})

	t.Run("explicit null sets Set true", func(t *testing.T) {
		var payload struct {
			Name Optional[string] `json:"name"`
		}
		if err := json.Unmarshal([]byte(`{"name":null}`), &payload); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !payload.Name.Set {
			t.Fatalf("expected Set=true for null field, got false")
		}
		if payload.Name.Value != "" {
			t.Fatalf("expected zero value, got %q", payload.Name.Value)
		}
	})

	t.Run("present value sets Set true and stores value", func(t *testing.T) {
		var payload struct {
			Age Optional[int] `json:"age"`
		}
		if err := json.Unmarshal([]byte(`{"age":42}`), &payload); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !payload.Age.Set {
			t.Fatalf("expected Set=true for present field, got false")
		}
		if payload.Age.Value != 42 {
			t.Fatalf("expected value 42, got %d", payload.Age.Value)
		}
	})
}
