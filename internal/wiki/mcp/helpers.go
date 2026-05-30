package mcp

import (
	"encoding/base64"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

const publicEditorID = "public-editor"

func (r *Routes) apiPage(page *tree.Page, depth int) *dto.Page {
	var apiPage *dto.Page
	if depth == 0 {
		apiPage = dto.ToAPIPage(page, r.userResolver)
	} else {
		apiPage = dto.ToAPIPageWithDepth(page, r.userResolver, depth)
	}
	wikipages.EnrichPageMetadata(apiPage, r.treeService.ReadPageRaw)
	return apiPage
}

func publicEditor() *coreauth.User {
	return &coreauth.User{
		ID:       publicEditorID,
		Username: publicEditorID,
		Role:     coreauth.RoleEditor,
	}
}

func (r *Routes) actorForRequest(req *sdkmcp.CallToolRequest) (*coreauth.User, error) {
	if req == nil {
		return r.actorForMissingTokenInfo()
	}
	extra := req.GetExtra()
	if extra == nil {
		return r.actorForMissingTokenInfo()
	}
	tokenInfo := extra.TokenInfo
	if tokenInfo == nil {
		return r.actorForMissingTokenInfo()
	}
	if r.userService == nil {
		return nil, fmt.Errorf("authenticated MCP user service is unavailable")
	}
	user, err := r.userService.GetUserByID(tokenInfo.UserID)
	if err != nil {
		return nil, fmt.Errorf("authenticated MCP user not found")
	}
	return user, nil
}

func (r *Routes) actorForMissingTokenInfo() (*coreauth.User, error) {
	if r.authDisabled {
		return publicEditor(), nil
	}
	return nil, fmt.Errorf("authenticated MCP token info missing")
}

func (r *Routes) editorActorForRequest(req *sdkmcp.CallToolRequest) (*coreauth.User, error) {
	user, err := r.actorForRequest(req)
	if err != nil {
		return nil, err
	}
	if user.Role != coreauth.RoleEditor && user.Role != coreauth.RoleAdmin {
		return nil, fmt.Errorf("editor or admin role required")
	}
	return user, nil
}

func mcpToolError(err error) error {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		return fmt.Errorf("%s: %s", loc.Code, loc.Message)
	}
	if detail, _, ok := wikipages.PageErrorDetailForError(err); ok {
		return fmt.Errorf("%s: %s", detail.Code, detail.Message)
	}
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func base64DecodedSize(encoded string) int64 {
	trimmed := strings.TrimSpace(encoded)
	size := base64.StdEncoding.DecodedLen(len(trimmed))
	switch {
	case strings.HasSuffix(trimmed, "=="):
		size -= 2
	case strings.HasSuffix(trimmed, "="):
		size--
	}
	if size < 0 {
		return 0
	}
	return int64(size)
}
