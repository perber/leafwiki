package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikisearch "github.com/perber/wiki/internal/wiki/search"
)

func (r *Routes) registerSearchTools(server *sdkmcp.Server) {
	addTypedTool[searchPagesInput, searchPagesOutput](server, toolSearchPages, func(ctx context.Context, in searchPagesInput) (searchPagesOutput, error) {
		if err := wikisearch.ValidateSearchRequest(in.Query, in.Tags); err != nil {
			return searchPagesOutput{}, err
		}
		limit := in.Limit
		if limit == 0 {
			limit = 20
		}
		out, err := r.search.Execute(ctx, wikisearch.SearchInput{
			Query:  in.Query,
			Tags:   in.Tags,
			Offset: in.Offset,
			Limit:  limit,
		})
		if err != nil {
			return searchPagesOutput{}, err
		}
		result := out.Result
		return searchPagesOutput{
			Count:     result.Count,
			Items:     result.Items,
			Limit:     result.Limit,
			Offset:    result.Offset,
			TagFacets: result.TagFacets,
			HasMore:   result.Offset+len(result.Items) < result.Count,
		}, nil
	})

	addTypedTool[emptyInput, searchStatusOutput](server, toolGetSearchStatus, func(ctx context.Context, _ emptyInput) (searchStatusOutput, error) {
		out := r.searchStatus.Execute(ctx)
		return searchStatusOutput{Status: out.Status}, nil
	})
}
