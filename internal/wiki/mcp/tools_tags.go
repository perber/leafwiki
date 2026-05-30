package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikitags "github.com/perber/wiki/internal/wiki/tags"
)

func (r *Routes) registerTagTools(server *sdkmcp.Server) {
	addTypedTool[listTagsInput, listTagsOutput](server, toolListTags, func(ctx context.Context, in listTagsInput) (listTagsOutput, error) {
		out, err := r.getTags.Execute(ctx, wikitags.GetTagsInput{
			Filter:   in.Query,
			Selected: in.Selected,
			Limit:    in.Limit,
		})
		if err != nil {
			return listTagsOutput{}, err
		}
		return listTagsOutput{Tags: out.Tags}, nil
	})

	addTypedTool[pagesByTagsInput, pagesOutput](server, toolGetPagesByTags, func(ctx context.Context, in pagesByTagsInput) (pagesOutput, error) {
		tags, err := wikitags.ValidatePagesByTagsInput(in.Tags)
		if err != nil {
			return pagesOutput{}, err
		}
		out, err := r.pagesByTags.Execute(ctx, wikitags.GetPagesByTagsInput{Tags: tags})
		if err != nil {
			return pagesOutput{}, err
		}
		return pagesOutput{Pages: out.Pages}, nil
	})
}
