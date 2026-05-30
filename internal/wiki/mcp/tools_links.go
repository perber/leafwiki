package mcp

import (
	"context"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikilinks "github.com/perber/wiki/internal/wiki/links"
)

func (r *Routes) registerLinkTools(server *sdkmcp.Server) {
	addTypedTool[pageIDInput, linkStatusOutput](server, toolGetLinkStatus, func(ctx context.Context, in pageIDInput) (linkStatusOutput, error) {
		out, err := r.linkStatus.Execute(ctx, wikilinks.GetLinkStatusInput{PageID: strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))})
		if err != nil {
			return linkStatusOutput{}, err
		}
		return linkStatusOutput{Status: out.Status}, nil
	})
}
