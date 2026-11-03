package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
)

// TestHTTPContextFromRequest_LiftsCallerIntoMCPContext proves the seam the
// tool handlers will read: the values the gin wrapper stores on the incoming
// request's context (routes.go) must arrive on the context mcp-go hands to
// the tools.
func TestHTTPContextFromRequest_LiftsCallerIntoMCPContext(t *testing.T) {
	user := &coreauth.User{ID: "u-1", Username: "admin", Role: coreauth.RoleAdmin}

	tests := []struct {
		name        string
		requestCtx  context.Context
		wantUser    *coreauth.User
		wantKeyAuth bool
	}{
		{
			name:        "no caller stays anonymous",
			requestCtx:  context.Background(),
			wantUser:    nil,
			wantKeyAuth: false,
		},
		{
			name:        "session caller",
			requestCtx:  context.WithValue(context.Background(), mcpUserKey, user),
			wantUser:    user,
			wantKeyAuth: false,
		},
		{
			name:        "api key caller carries the user and the key marker",
			requestCtx:  context.WithValue(context.WithValue(context.Background(), mcpUserKey, user), mcpKeyAuthKey, true),
			wantUser:    user,
			wantKeyAuth: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil).WithContext(tc.requestCtx)
			ctx := httpContextFromRequest(context.Background(), req)

			if got := userFromContext(ctx); got != tc.wantUser {
				t.Errorf("userFromContext = %v, want %v", got, tc.wantUser)
			}
			if got := keyAuthFromContext(ctx); got != tc.wantKeyAuth {
				t.Errorf("keyAuthFromContext = %v, want %v", got, tc.wantKeyAuth)
			}
		})
	}
}
