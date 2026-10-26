package dto

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestErrorResponse_Marshal_PreservesWireShape(t *testing.T) {
	detail := ErrorDetail{Code: "page_not_found", Message: "Page not found", Template: "page not found"}
	withArgs := detail
	withArgs.Args = []string{"abc"}
	emptyArgs := detail
	emptyArgs.Args = []string{}

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"nil args are omitted", ErrorResponse{Error: detail}, `{"error":{"code":"page_not_found","message":"Page not found","template":"page not found"}}`},
		{"empty args are omitted", ErrorResponse{Error: emptyArgs}, `{"error":{"code":"page_not_found","message":"Page not found","template":"page not found"}}`},
		{"populated args", ErrorResponse{Error: withArgs}, `{"error":{"code":"page_not_found","message":"Page not found","template":"page not found","args":["abc"]}}`},
		{"legacy flat error", LegacyError{Error: "Invalid or expired API key"}, `{"error":"Invalid or expired API key"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var gotDecoded, wantDecoded any
			if err := json.Unmarshal(got, &gotDecoded); err != nil {
				t.Fatalf("decode got %s: %v", got, err)
			}
			if err := json.Unmarshal([]byte(tt.want), &wantDecoded); err != nil {
				t.Fatalf("decode want: %v", err)
			}
			// Decoded comparison: object key order is not part of the contract, and
			// it also proves there are no extra fields (e.g. a localized "fields").
			if !reflect.DeepEqual(gotDecoded, wantDecoded) {
				t.Fatalf("wire shape = %s, want %s", got, tt.want)
			}
		})
	}
}
