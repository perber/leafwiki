package mcp

import (
	"bytes"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
)

type memoryMultipartFile struct {
	*bytes.Reader
}

func (f *memoryMultipartFile) Close() error {
	return nil
}

type emptyInput struct{}

type configOutput struct {
	PublicAccess            bool   `json:"publicAccess"`
	HideLinkMetadataSection bool   `json:"hideLinkMetadataSection"`
	AuthDisabled            bool   `json:"authDisabled"`
	BasePath                string `json:"basePath"`
	MaxAssetUploadSizeBytes int64  `json:"maxAssetUploadSizeBytes"`
	EnableRevision          bool   `json:"enableRevision"`
	EnableLinkRefactor      bool   `json:"enableLinkRefactor"`
	HTTPRemoteUserEnabled   bool   `json:"httpRemoteUserEnabled"`
	HTTPRemoteUserLogoutURL string `json:"httpRemoteUserLogoutUrl"`
}

type currentUserOutput struct {
	User *coreauth.PublicUser `json:"user"`
}

type getTreeInput struct {
	Depth *int `json:"depth,omitempty"`
}

type treeOutput struct {
	Tree *dto.Node `json:"tree"`
}

type pageIDInput struct {
	ID     string `json:"id,omitempty"`
	PageID string `json:"pageId,omitempty"`
}

type pathInput struct {
	Path string `json:"path"`
}

type lookupPathOutput struct {
	Lookup *tree.PathLookup `json:"lookup"`
}

type resolvePermalinkOutput struct {
	Target *tree.PermalinkTarget `json:"target"`
}

type suggestSlugInput struct {
	ParentID  string `json:"parentId,omitempty"`
	CurrentID string `json:"currentId,omitempty"`
	Title     string `json:"title"`
}

type suggestSlugOutput struct {
	Slug string `json:"slug"`
}

type createPageInput struct {
	ParentID *string `json:"parentId,omitempty"`
	Title    string  `json:"title"`
	Slug     string  `json:"slug"`
	Kind     *string `json:"kind,omitempty"`
}

type updatePageInput struct {
	ID         string            `json:"id"`
	Version    string            `json:"version"`
	Title      string            `json:"title"`
	Slug       string            `json:"slug"`
	Content    *string           `json:"content,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

type pageOutput struct {
	Page       *dto.Page `json:"page"`
	LinkStatus any       `json:"linkStatus,omitempty"`
}

type deletePageInput struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Recursive bool   `json:"recursive,omitempty"`
}

type movePageInput struct {
	ID       string  `json:"id"`
	Version  string  `json:"version"`
	ParentID *string `json:"parentId,omitempty"`
}

type sortPagesInput struct {
	ParentID   string   `json:"parentId"`
	OrderedIDs []string `json:"orderedIds"`
}

type ensurePageInput struct {
	Path  string  `json:"path"`
	Title string  `json:"title"`
	Kind  *string `json:"kind,omitempty"`
}

type convertPageInput struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	TargetKind string `json:"targetKind"`
}

type copyPageInput struct {
	ID             string  `json:"id"`
	TargetParentID *string `json:"targetParentId,omitempty"`
	Title          string  `json:"title"`
	Slug           string  `json:"slug"`
}

type messageOutput struct {
	Message string `json:"message"`
}

type searchPagesInput struct {
	Query  string   `json:"q,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Offset int      `json:"offset,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

type searchPagesOutput struct {
	Count     int  `json:"count"`
	Items     any  `json:"items"`
	Limit     int  `json:"limit"`
	Offset    int  `json:"offset"`
	TagFacets any  `json:"tagFacets"`
	HasMore   bool `json:"hasMore"`
}

type searchStatusOutput struct {
	Status any `json:"status"`
}

type listTagsInput struct {
	Query    string   `json:"q,omitempty"`
	Selected []string `json:"selected,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

type listTagsOutput struct {
	Tags any `json:"tags"`
}

type pagesByTagsInput struct {
	Tags []string `json:"tags"`
}

type pagesOutput struct {
	Pages any `json:"pages"`
}

type listPropertyKeysInput struct {
	Query string `json:"q,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type propertyKeysOutput struct {
	Keys any `json:"keys"`
}

type pagesByPropertyInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type linkStatusOutput struct {
	Status any `json:"status"`
}

type uploadAssetInput struct {
	PageID        string `json:"pageId"`
	Filename      string `json:"filename"`
	ContentBase64 string `json:"contentBase64"`
}

type uploadAssetOutput struct {
	File string `json:"file"`
}

type assetInput struct {
	PageID   string `json:"pageId"`
	Filename string `json:"filename"`
}

type assetOutput struct {
	Filename      string `json:"filename"`
	MimeType      string `json:"mimeType"`
	ContentBase64 string `json:"contentBase64"`
}

type listAssetsOutput struct {
	Files []string `json:"files"`
}

type renameAssetInput struct {
	PageID      string `json:"pageId"`
	OldFilename string `json:"oldFilename"`
	NewFilename string `json:"newFilename"`
}

type renameAssetOutput struct {
	URL string `json:"url"`
}

type deleteAssetInput struct {
	PageID   string `json:"pageId"`
	Filename string `json:"filename"`
}

type listRevisionsInput struct {
	ID     string `json:"id,omitempty"`
	PageID string `json:"pageId,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  *int   `json:"limit,omitempty"`
}

type listRevisionsOutput struct {
	Revisions  any    `json:"revisions"`
	NextCursor string `json:"nextCursor"`
}

type revisionIDInput struct {
	ID         string `json:"id,omitempty"`
	PageID     string `json:"pageId,omitempty"`
	RevisionID string `json:"revisionId,omitempty"`
}

type revisionOutput struct {
	Revision any `json:"revision"`
}

type compareRevisionsInput struct {
	ID               string `json:"id,omitempty"`
	PageID           string `json:"pageId,omitempty"`
	BaseRevisionID   string `json:"baseRevisionId"`
	TargetRevisionID string `json:"targetRevisionId"`
}

type revisionAssetInput struct {
	ID         string `json:"id,omitempty"`
	PageID     string `json:"pageId,omitempty"`
	RevisionID string `json:"revisionId"`
	AssetName  string `json:"assetName"`
}

type previewRefactorInput struct {
	ID       string  `json:"id,omitempty"`
	PageID   string  `json:"pageId,omitempty"`
	Kind     string  `json:"kind"`
	Title    string  `json:"title,omitempty"`
	Slug     string  `json:"slug,omitempty"`
	Content  *string `json:"content,omitempty"`
	ParentID *string `json:"parentId,omitempty"`
}

type applyRefactorInput struct {
	ID           string  `json:"id,omitempty"`
	PageID       string  `json:"pageId,omitempty"`
	Version      string  `json:"version,omitempty"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title,omitempty"`
	Slug         string  `json:"slug,omitempty"`
	Content      *string `json:"content,omitempty"`
	ParentID     *string `json:"parentId,omitempty"`
	RewriteLinks bool    `json:"rewriteLinks,omitempty"`
}
