package test_utils

import (
	"encoding/json"
	"sort"
	"testing"
)

// JSONKind returns the JSON type of raw: "string", "number", "bool", "null",
// "array" or "object". It fails the test for invalid JSON.
func JSONKind(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	case []any:
		return "array"
	default:
		return "object"
	}
}

// AssertJSONObjectShape fails unless raw is a JSON object with exactly the keys
// of want, each holding the JSON type named there (see JSONKind). It pins the
// wire shape of an API response independently of the Go type that produced it.
func AssertJSONObjectShape(t *testing.T, raw json.RawMessage, want map[string]string) {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("not a JSON object: %s: %v", raw, err)
	}
	if len(obj) != len(want) {
		t.Errorf("keys = %v, want %v", sortedKeys(obj), sortedKeys(want))
	}
	for key, kind := range want {
		value, ok := obj[key]
		if !ok {
			t.Errorf("missing key %q in %s", key, raw)
			continue
		}
		if got := JSONKind(t, value); got != kind {
			t.Errorf("%s is a JSON %s, want %s", key, got, kind)
		}
	}
}

// AssertLocalizedError fails unless raw is the localized error envelope
// (dto.ErrorResponse without args) carrying the given code.
func AssertLocalizedError(t *testing.T, raw json.RawMessage, code string) {
	t.Helper()
	AssertJSONObjectShape(t, raw, map[string]string{"error": "object"})
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	AssertJSONObjectShape(t, body.Error, map[string]string{"code": "string", "message": "string", "template": "string"})
	var detail struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body.Error, &detail); err != nil {
		t.Fatalf("decode %s: %v", body.Error, err)
	}
	if detail.Code != code {
		t.Errorf("error code = %q, want %q", detail.Code, code)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
