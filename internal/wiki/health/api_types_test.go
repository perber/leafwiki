package health

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthResponse_Marshal_PreservesWireShape(t *testing.T) {
	tests := []struct {
		name   string
		status string
		checks map[string]string
		want   string
	}{
		{"populated checks", "ok", map[string]string{"sqlite": "ok", "data_dir": "ok", "search": "indexing"}, `{"status":"ok","checks":{"sqlite":"ok","data_dir":"ok","search":"indexing"}}`},
		{"nil checks are null, not omitted", "degraded", nil, `{"status":"degraded","checks":null}`},
		{"empty checks are an object, not omitted", "degraded", map[string]string{}, `{"status":"degraded","checks":{}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(HealthResponse{Status: tt.status, Checks: tt.checks})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			legacy, err := json.Marshal(gin.H{"status": tt.status, "checks": tt.checks})
			if err != nil {
				t.Fatalf("Marshal legacy map: %v", err)
			}

			if !jsonEqual(t, got, []byte(tt.want)) {
				t.Errorf("wire shape = %s, want %s", got, tt.want)
			}
			if !jsonEqual(t, got, legacy) {
				t.Errorf("named model = %s, differs from equivalent map %s", got, legacy)
			}
		})
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("decode %s: %v", a, err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return reflect.DeepEqual(av, bv)
}
