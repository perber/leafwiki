package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	httpinternal "github.com/perber/wiki/internal/http"
)

func (r *Routes) registerConfigTools(server *sdkmcp.Server, opts httpinternal.RouterOptions) {
	addTypedTool[emptyInput, currentUserOutput](server, toolGetCurrentUser, func(context.Context, emptyInput) (currentUserOutput, error) {
		return currentUserOutput{User: publicEditor().ToPublicUser()}, nil
	})

	addTypedTool[emptyInput, configOutput](server, toolGetConfig, func(context.Context, emptyInput) (configOutput, error) {
		return configOutput{
			PublicAccess:            opts.PublicAccess,
			HideLinkMetadataSection: opts.HideLinkMetadataSection,
			AuthDisabled:            opts.AuthDisabled,
			BasePath:                opts.BasePath,
			MaxAssetUploadSizeBytes: opts.MaxAssetUploadSizeBytes,
			EnableRevision:          opts.EnableRevision,
			EnableLinkRefactor:      opts.EnableLinkRefactor,
			HTTPRemoteUserEnabled:   opts.HTTPRemoteUser.Enabled,
			HTTPRemoteUserLogoutURL: opts.HTTPRemoteUser.LogoutURL,
		}, nil
	})
}
