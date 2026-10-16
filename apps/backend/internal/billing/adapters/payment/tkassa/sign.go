package tkassa

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// sign computes the T-Kassa request/webhook token.
// It excludes the keys Token, DATA, Data and Receipt, adds the password, sorts
// keys lexicographically and concatenates the stringified values before
// SHA-256 hashing. The algorithm matches the official token page verbatim:
// only root-object parameters participate (nested objects and arrays are
// excluded), values are concatenated without separators, and the digest is
// hex-lowercase.
func sign(data map[string]any, password string) string {
	keys := make([]string, 0, len(data)+1)
	for k := range data {
		if k == fieldToken || k == fieldDATA || k == fieldData || k == fieldReceipt {
			continue
		}
		keys = append(keys, k)
	}
	keys = append(keys, "Password")
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		if k == "Password" {
			sb.WriteString(password)
			continue
		}
		if shouldSkipValue(data[k]) {
			continue
		}
		sb.WriteString(stringifyValue(data[k]))
	}

	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}

// shouldSkipValue reports whether a value contributes no bytes to the signed
// concatenation: nil, empty strings and nested structures (which never
// participate in the token anyway).
func shouldSkipValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case map[string]any, []any:
		return true
	}
	return false
}

// stringifyValue renders a value the way the token concatenation expects it:
// numbers keep their wire representation (json.Number preserves the original
// digits), booleans are true/false.
func stringifyValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// Every integer width renders as its plain decimal literal.
		return fmt.Sprint(val)
	case float64:
		return stringifyFloat(val)
	case bool:
		return strconv.FormatBool(val)
	case json.Number:
		return val.String()
	case map[string]any, []any:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// stringifyFloat renders a float the way the token concatenation expects it:
// integral values collapse to their integer form, the rest keep the shortest
// decimal representation.
func stringifyFloat(val float64) string {
	if val == math.Trunc(val) {
		return strconv.FormatInt(int64(val), 10)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// verifyToken checks the token of an incoming webhook payload. The comparison
// is constant-time (hmac.Equal) so timing does not leak the expected digest.
func verifyToken(payload []byte, password string) error {
	data, err := unmarshalWebhook(payload)
	if err != nil {
		return err
	}

	expected := sign(data, password)
	actual, ok := data[fieldToken].(string)

	if !ok || !hmac.Equal([]byte(expected), []byte(actual)) {
		return errors.New("tkassa: invalid webhook token")
	}
	return nil
}

func unmarshalWebhook(payload []byte) (map[string]any, error) {
	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("tkassa: invalid webhook payload: %w", err)
	}
	return data, nil
}
