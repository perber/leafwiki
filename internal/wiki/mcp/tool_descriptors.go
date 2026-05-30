package mcp

// ToolDescriptor is the single source for an MCP tool's protocol name and
// user-facing description.
type ToolDescriptor struct {
	Name        string
	Description string
}

const (
	ToolGetConfig          = "get_config"
	ToolGetCurrentUser     = "get_current_user"
	ToolGetTree            = "get_tree"
	ToolGetPage            = "get_page"
	ToolGetPageByPath      = "get_page_by_path"
	ToolLookupPath         = "lookup_path"
	ToolResolvePermalink   = "resolve_permalink"
	ToolSuggestSlug        = "suggest_slug"
	ToolCreatePage         = "create_page"
	ToolUpdatePage         = "update_page"
	ToolDeletePage         = "delete_page"
	ToolMovePage           = "move_page"
	ToolSortPages          = "sort_pages"
	ToolEnsurePage         = "ensure_page"
	ToolConvertPage        = "convert_page"
	ToolCopyPage           = "copy_page"
	ToolSearchPages        = "search_pages"
	ToolGetSearchStatus    = "get_search_status"
	ToolListTags           = "list_tags"
	ToolGetPagesByTags     = "get_pages_by_tags"
	ToolListPropertyKeys   = "list_property_keys"
	ToolGetPagesByProperty = "get_pages_by_property"
	ToolGetLinkStatus      = "get_link_status"
	ToolUploadAsset        = "upload_asset"
	ToolGetAsset           = "get_asset"
	ToolListAssets         = "list_assets"
	ToolRenameAsset        = "rename_asset"
	ToolDeleteAsset        = "delete_asset"
	ToolListRevisions      = "list_revisions"
	ToolGetLatestRevision  = "get_latest_revision"
	ToolGetRevision        = "get_revision"
	ToolCompareRevisions   = "compare_revisions"
	ToolGetRevisionAsset   = "get_revision_asset"
	ToolRestoreRevision    = "restore_revision"
	ToolPreviewRefactor    = "preview_page_refactor"
	ToolApplyRefactor      = "apply_page_refactor"
)

var (
	toolGetConfig          = ToolDescriptor{Name: ToolGetConfig, Description: "Return local MCP-visible LeafWiki configuration"}
	toolGetCurrentUser     = ToolDescriptor{Name: ToolGetCurrentUser, Description: "Return the effective MCP user"}
	toolGetTree            = ToolDescriptor{Name: ToolGetTree, Description: "Return the wiki page tree"}
	toolGetPage            = ToolDescriptor{Name: ToolGetPage, Description: "Return a page by ID with link status context"}
	toolGetPageByPath      = ToolDescriptor{Name: ToolGetPageByPath, Description: "Return a page by route path with link status context"}
	toolLookupPath         = ToolDescriptor{Name: ToolLookupPath, Description: "Resolve a route path into existing and missing path segments"}
	toolResolvePermalink   = ToolDescriptor{Name: ToolResolvePermalink, Description: "Resolve a stable page ID to its current route path"}
	toolSuggestSlug        = ToolDescriptor{Name: ToolSuggestSlug, Description: "Suggest a unique child slug for a title"}
	toolCreatePage         = ToolDescriptor{Name: ToolCreatePage, Description: "Create a wiki page or section"}
	toolUpdatePage         = ToolDescriptor{Name: ToolUpdatePage, Description: "Update page title, slug, content, tags, and properties"}
	toolDeletePage         = ToolDescriptor{Name: ToolDeletePage, Description: "Delete a page"}
	toolMovePage           = ToolDescriptor{Name: ToolMovePage, Description: "Move a page to a new parent"}
	toolSortPages          = ToolDescriptor{Name: ToolSortPages, Description: "Sort a parent's child pages"}
	toolEnsurePage         = ToolDescriptor{Name: ToolEnsurePage, Description: "Ensure a page exists at a route path"}
	toolConvertPage        = ToolDescriptor{Name: ToolConvertPage, Description: "Convert a page between page and section kinds"}
	toolCopyPage           = ToolDescriptor{Name: ToolCopyPage, Description: "Copy a page and its assets"}
	toolSearchPages        = ToolDescriptor{Name: ToolSearchPages, Description: "Search pages using LeafWiki offset and limit pagination"}
	toolGetSearchStatus    = ToolDescriptor{Name: ToolGetSearchStatus, Description: "Return the search indexing status"}
	toolListTags           = ToolDescriptor{Name: ToolListTags, Description: "List tag counts"}
	toolGetPagesByTags     = ToolDescriptor{Name: ToolGetPagesByTags, Description: "List pages matching all tags"}
	toolListPropertyKeys   = ToolDescriptor{Name: ToolListPropertyKeys, Description: "List property key counts"}
	toolGetPagesByProperty = ToolDescriptor{Name: ToolGetPagesByProperty, Description: "List pages with a property value"}
	toolGetLinkStatus      = ToolDescriptor{Name: ToolGetLinkStatus, Description: "Return link status for a page"}
	toolUploadAsset        = ToolDescriptor{Name: ToolUploadAsset, Description: "Upload an asset from base64 content"}
	toolGetAsset           = ToolDescriptor{Name: ToolGetAsset, Description: "Read an asset as base64 content"}
	toolListAssets         = ToolDescriptor{Name: ToolListAssets, Description: "List page assets"}
	toolRenameAsset        = ToolDescriptor{Name: ToolRenameAsset, Description: "Rename a page asset"}
	toolDeleteAsset        = ToolDescriptor{Name: ToolDeleteAsset, Description: "Delete a page asset"}
	toolListRevisions      = ToolDescriptor{Name: ToolListRevisions, Description: "List page revisions"}
	toolGetLatestRevision  = ToolDescriptor{Name: ToolGetLatestRevision, Description: "Get the latest page revision"}
	toolGetRevision        = ToolDescriptor{Name: ToolGetRevision, Description: "Get a page revision snapshot"}
	toolCompareRevisions   = ToolDescriptor{Name: ToolCompareRevisions, Description: "Compare two page revisions"}
	toolGetRevisionAsset   = ToolDescriptor{Name: ToolGetRevisionAsset, Description: "Read a revision asset as base64 content"}
	toolRestoreRevision    = ToolDescriptor{Name: ToolRestoreRevision, Description: "Restore a page revision"}
	toolPreviewRefactor    = ToolDescriptor{Name: ToolPreviewRefactor, Description: "Preview a page rename or move refactor"}
	toolApplyRefactor      = ToolDescriptor{Name: ToolApplyRefactor, Description: "Apply a page rename or move refactor"}
)

var baseToolDescriptors = []ToolDescriptor{
	toolGetConfig, toolGetCurrentUser, toolGetTree, toolGetPage, toolGetPageByPath,
	toolLookupPath, toolResolvePermalink, toolSuggestSlug, toolCreatePage,
	toolUpdatePage, toolDeletePage, toolMovePage, toolSortPages, toolEnsurePage,
	toolConvertPage, toolCopyPage, toolSearchPages, toolGetSearchStatus,
	toolListTags, toolGetPagesByTags, toolListPropertyKeys, toolGetPagesByProperty,
	toolGetLinkStatus, toolUploadAsset, toolGetAsset, toolListAssets, toolRenameAsset,
	toolDeleteAsset,
}

var revisionToolDescriptors = []ToolDescriptor{
	toolListRevisions, toolGetLatestRevision, toolGetRevision, toolCompareRevisions,
	toolGetRevisionAsset, toolRestoreRevision,
}

var linkRefactorToolDescriptors = []ToolDescriptor{
	toolPreviewRefactor, toolApplyRefactor,
}

func BaseToolNames() []string {
	return toolNames(baseToolDescriptors)
}

func RevisionToolNames() []string {
	return toolNames(revisionToolDescriptors)
}

func LinkRefactorToolNames() []string {
	return toolNames(linkRefactorToolDescriptors)
}

func toolNames(descriptors []ToolDescriptor) []string {
	names := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		names = append(names, descriptor.Name)
	}
	return names
}
