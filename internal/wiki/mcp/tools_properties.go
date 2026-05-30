package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikiproperties "github.com/perber/wiki/internal/wiki/properties"
)

func (r *Routes) registerPropertyTools(server *sdkmcp.Server) {
	addTypedTool[listPropertyKeysInput, propertyKeysOutput](server, toolListPropertyKeys, func(ctx context.Context, in listPropertyKeysInput) (propertyKeysOutput, error) {
		out, err := r.propertyKeys.Execute(ctx, wikiproperties.GetPropertyKeysInput{Filter: in.Query, Limit: in.Limit})
		if err != nil {
			return propertyKeysOutput{}, err
		}
		return propertyKeysOutput{Keys: out.Keys}, nil
	})

	addTypedTool[pagesByPropertyInput, pagesOutput](server, toolGetPagesByProperty, func(ctx context.Context, in pagesByPropertyInput) (pagesOutput, error) {
		out, err := r.pagesByProp.Execute(ctx, wikiproperties.GetPagesByPropertyInput{Key: in.Key, Value: in.Value})
		if err != nil {
			return pagesOutput{}, err
		}
		return pagesOutput{Pages: out.Pages}, nil
	})
}
