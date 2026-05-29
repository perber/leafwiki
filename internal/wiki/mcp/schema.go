package mcp

import (
	"context"
	"sort"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

type toolActor struct {
	ID   string
	User *coreauth.User
}

func addTypedTool[In, Out any](server *sdkmcp.Server, descriptor ToolDescriptor, handler func(context.Context, In) (Out, error)) {
	addRequestTypedTool(server, descriptor, func(ctx context.Context, _ *sdkmcp.CallToolRequest, in In) (Out, error) {
		return handler(ctx, in)
	})
}

func addRequestTypedTool[In, Out any](server *sdkmcp.Server, descriptor ToolDescriptor, handler func(context.Context, *sdkmcp.CallToolRequest, In) (Out, error)) {
	tool := &sdkmcp.Tool{
		Name:         descriptor.Name,
		Description:  descriptor.Description,
		OutputSchema: toolOutputSchema(descriptor.Name),
	}
	if inputSchema := toolInputSchema(descriptor.Name); inputSchema != nil {
		tool.InputSchema = inputSchema
	}

	sdkmcp.AddTool[In, Out](server, tool, func(ctx context.Context, req *sdkmcp.CallToolRequest, in In) (*sdkmcp.CallToolResult, Out, error) {
		var zero Out
		out, err := handler(ctx, req, in)
		if err != nil {
			return nil, zero, mcpToolError(err)
		}
		return nil, out, nil
	})
}

func addActorTool[In, Out any](routes *Routes, server *sdkmcp.Server, descriptor ToolDescriptor, handler func(context.Context, toolActor, In) (Out, error)) {
	addRequestTypedTool(server, descriptor, func(ctx context.Context, req *sdkmcp.CallToolRequest, in In) (Out, error) {
		var zero Out
		user, err := routes.actorForRequest(req)
		if err != nil {
			return zero, err
		}
		return handler(ctx, toolActor{ID: user.ID, User: user}, in)
	})
}

func addEditorTool[In, Out any](routes *Routes, server *sdkmcp.Server, descriptor ToolDescriptor, handler func(context.Context, toolActor, In) (Out, error)) {
	addRequestTypedTool(server, descriptor, func(ctx context.Context, req *sdkmcp.CallToolRequest, in In) (Out, error) {
		var zero Out
		user, err := routes.editorActorForRequest(req)
		if err != nil {
			return zero, err
		}
		return handler(ctx, toolActor{ID: user.ID, User: user}, in)
	})
}

func toolInputSchema(name string) *jsonschema.Schema {
	switch name {
	case ToolCreatePage:
		return objectSchema(map[string]*jsonschema.Schema{
			"parentId": nullableStringSchema(),
			"title":    stringSchema(),
			"slug":     stringSchema(),
			"kind":     nullableStringSchema(),
		}, []string{"title", "slug"})
	case ToolEnsurePage:
		return objectSchema(map[string]*jsonschema.Schema{
			"path":  stringSchema(),
			"title": stringSchema(),
			"kind":  nullableStringSchema(),
		}, []string{"path", "title"})
	case ToolConvertPage:
		return objectSchema(map[string]*jsonschema.Schema{
			"id":         stringSchema(),
			"version":    stringSchema(),
			"targetKind": stringSchema(),
		}, []string{"id", "version", "targetKind"})
	case ToolGetPage, ToolResolvePermalink, ToolGetLinkStatus, ToolListAssets, ToolGetLatestRevision:
		return pageIDSchema()
	case ToolListRevisions:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"cursor": stringSchema(),
			"limit":  integerSchema(),
		}, nil)
	case ToolGetRevision, ToolRestoreRevision:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"revisionId": stringSchema(),
		}, []string{"revisionId"})
	case ToolCompareRevisions:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"baseRevisionId":   stringSchema(),
			"targetRevisionId": stringSchema(),
		}, []string{"baseRevisionId", "targetRevisionId"})
	case ToolGetRevisionAsset:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"revisionId": stringSchema(),
			"assetName":  stringSchema(),
		}, []string{"revisionId", "assetName"})
	case ToolPreviewRefactor:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"kind":     stringSchema(),
			"title":    stringSchema(),
			"slug":     stringSchema(),
			"content":  nullableStringSchema(),
			"parentId": nullableStringSchema(),
		}, []string{"kind"})
	case ToolApplyRefactor:
		return pageIDSchemaWith(map[string]*jsonschema.Schema{
			"version":      stringSchema(),
			"kind":         stringSchema(),
			"title":        stringSchema(),
			"slug":         stringSchema(),
			"content":      nullableStringSchema(),
			"parentId":     nullableStringSchema(),
			"rewriteLinks": booleanSchema(),
		}, []string{"version", "kind"})
	default:
		return nil
	}
}

func pageIDSchema() *jsonschema.Schema {
	return pageIDSchemaWith(nil, nil)
}

func pageIDSchemaWith(extra map[string]*jsonschema.Schema, required []string) *jsonschema.Schema {
	props := map[string]*jsonschema.Schema{
		"id":     stringSchema(),
		"pageId": stringSchema(),
	}
	for name, schema := range extra {
		props[name] = schema
	}
	schema := objectSchema(props, required)
	schema.AnyOf = requiredAlternatives("id", "pageId")
	return schema
}

func toolOutputSchema(name string) *jsonschema.Schema {
	switch name {
	case ToolGetCurrentUser:
		return outputSchema(map[string]*jsonschema.Schema{"user": objectValueSchema()})
	case ToolGetConfig:
		return outputSchema(map[string]*jsonschema.Schema{
			"authDisabled":            booleanSchema(),
			"basePath":                stringSchema(),
			"publicAccess":            booleanSchema(),
			"hideLinkMetadataSection": booleanSchema(),
			"maxAssetUploadSizeBytes": integerSchema(),
			"enableRevision":          booleanSchema(),
			"enableLinkRefactor":      booleanSchema(),
			"httpRemoteUserEnabled":   booleanSchema(),
			"httpRemoteUserLogoutUrl": stringSchema(),
		})
	case ToolGetTree:
		return outputSchema(map[string]*jsonschema.Schema{"tree": objectValueSchema()})
	case ToolGetPage, ToolGetPageByPath:
		return outputSchema(map[string]*jsonschema.Schema{
			"page":       objectValueSchema(),
			"linkStatus": objectValueSchema(),
		})
	case ToolCreatePage, ToolUpdatePage, ToolEnsurePage, ToolCopyPage, ToolRestoreRevision, ToolApplyRefactor:
		return outputSchema(map[string]*jsonschema.Schema{"page": objectValueSchema()})
	case ToolLookupPath:
		return outputSchema(map[string]*jsonschema.Schema{"lookup": objectValueSchema()})
	case ToolResolvePermalink:
		return outputSchema(map[string]*jsonschema.Schema{"target": objectValueSchema()})
	case ToolSuggestSlug:
		return outputSchema(map[string]*jsonschema.Schema{"slug": stringSchema()})
	case ToolDeletePage, ToolMovePage, ToolSortPages, ToolConvertPage, ToolDeleteAsset:
		return outputSchema(map[string]*jsonschema.Schema{"message": stringSchema()})
	case ToolSearchPages:
		return outputSchema(map[string]*jsonschema.Schema{
			"count":     integerSchema(),
			"items":     arrayValueSchema(),
			"limit":     integerSchema(),
			"offset":    integerSchema(),
			"tagFacets": arrayValueSchema(),
			"hasMore":   booleanSchema(),
		})
	case ToolGetSearchStatus:
		return outputSchema(map[string]*jsonschema.Schema{"status": nullableObjectSchema()})
	case ToolListTags:
		return outputSchema(map[string]*jsonschema.Schema{"tags": arrayValueSchema()})
	case ToolGetPagesByTags, ToolGetPagesByProperty:
		return outputSchema(map[string]*jsonschema.Schema{"pages": arrayValueSchema()})
	case ToolListPropertyKeys:
		return outputSchema(map[string]*jsonschema.Schema{"keys": arrayValueSchema()})
	case ToolGetLinkStatus:
		return outputSchema(map[string]*jsonschema.Schema{"status": objectValueSchema()})
	case ToolUploadAsset:
		return outputSchema(map[string]*jsonschema.Schema{"file": stringSchema()})
	case ToolGetAsset, ToolGetRevisionAsset:
		return outputSchema(map[string]*jsonschema.Schema{
			"filename":      stringSchema(),
			"mimeType":      stringSchema(),
			"contentBase64": stringSchema(),
		})
	case ToolListAssets:
		return outputSchema(map[string]*jsonschema.Schema{"files": arrayValueSchema()})
	case ToolRenameAsset:
		return outputSchema(map[string]*jsonschema.Schema{"url": stringSchema()})
	case ToolListRevisions:
		return outputSchema(map[string]*jsonschema.Schema{
			"revisions":  arrayValueSchema(),
			"nextCursor": stringSchema(),
		})
	case ToolGetLatestRevision:
		return outputSchema(map[string]*jsonschema.Schema{"revision": objectValueSchema()})
	case ToolGetRevision:
		return outputSchema(map[string]*jsonschema.Schema{
			"revision": objectValueSchema(),
			"content":  stringSchema(),
			"assets":   arrayValueSchema(),
		})
	case ToolCompareRevisions:
		return outputSchema(map[string]*jsonschema.Schema{
			"base":           objectValueSchema(),
			"target":         objectValueSchema(),
			"contentChanged": booleanSchema(),
			"assetChanges":   arrayValueSchema(),
		})
	case ToolPreviewRefactor:
		return outputSchema(map[string]*jsonschema.Schema{
			"kind":          stringSchema(),
			"pageId":        stringSchema(),
			"oldPath":       stringSchema(),
			"newPath":       stringSchema(),
			"affectedPages": arrayValueSchema(),
			"counts":        objectValueSchema(),
			"warnings":      arrayValueSchema(),
		})
	default:
		return &jsonschema.Schema{Type: "object"}
	}
}

func outputSchema(properties map[string]*jsonschema.Schema) *jsonschema.Schema {
	required := make([]string, 0, len(properties))
	for name := range properties {
		required = append(required, name)
	}
	sort.Strings(required)
	return objectSchema(properties, required)
}

func objectSchema(properties map[string]*jsonschema.Schema, required []string) *jsonschema.Schema {
	order := make([]string, 0, len(properties))
	for name := range properties {
		order = append(order, name)
	}
	sort.Strings(order)
	return &jsonschema.Schema{
		Type:          "object",
		Properties:    properties,
		Required:      append([]string{}, required...),
		PropertyOrder: order,
	}
}

func requiredAlternatives(names ...string) []*jsonschema.Schema {
	out := make([]*jsonschema.Schema, 0, len(names))
	for _, name := range names {
		out = append(out, &jsonschema.Schema{Required: []string{name}})
	}
	return out
}

func stringSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "string"}
}

func nullableStringSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Types: []string{"string", "null"}}
}

func integerSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "integer"}
}

func booleanSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "boolean"}
}

func objectValueSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "object"}
}

func nullableObjectSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Types: []string{"object", "null"}}
}

func arrayValueSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "array"}
}
