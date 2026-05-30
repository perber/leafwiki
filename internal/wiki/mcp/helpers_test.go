package mcp

import (
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestActorForRequestRejectsMissingTokenInfoByDefault(t *testing.T) {
	t.Parallel()

	routes := &Routes{}
	for _, req := range []*sdkmcp.CallToolRequest{
		nil,
		{},
		{Extra: &sdkmcp.RequestExtra{}},
	} {
		if user, err := routes.actorForRequest(req); err == nil {
			t.Fatalf("actorForRequest(%#v) returned user %#v, want missing-token error", req, user)
		}
	}
}
