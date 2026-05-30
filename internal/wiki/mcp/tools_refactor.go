package mcp

import (
	"context"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

func (r *Routes) registerRefactorTools(server *sdkmcp.Server) {
	addEditorTool[previewRefactorInput, *wikipages.RefactorPreview](r, server, toolPreviewRefactor, func(ctx context.Context, _ toolActor, in previewRefactorInput) (*wikipages.RefactorPreview, error) {
		out, err := r.previewRef.Execute(ctx, wikipages.RefactorPreviewInput{
			PageID:      strings.TrimSpace(firstNonEmpty(in.PageID, in.ID)),
			Kind:        in.Kind,
			Title:       in.Title,
			Slug:        in.Slug,
			Content:     in.Content,
			NewParentID: in.ParentID,
		})
		if err != nil {
			return nil, err
		}
		return out, nil
	})

	addEditorTool[applyRefactorInput, pageOutput](r, server, toolApplyRefactor, func(ctx context.Context, actor toolActor, in applyRefactorInput) (pageOutput, error) {
		page, err := r.applyRef.Execute(ctx, wikipages.RefactorApplyInput{
			UserID:       actor.ID,
			Version:      strings.TrimSpace(in.Version),
			RewriteLinks: in.RewriteLinks,
			RefactorPreviewInput: wikipages.RefactorPreviewInput{
				PageID:      strings.TrimSpace(firstNonEmpty(in.PageID, in.ID)),
				Kind:        in.Kind,
				Title:       in.Title,
				Slug:        in.Slug,
				Content:     in.Content,
				NewParentID: in.ParentID,
			},
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(page, 0)}, nil
	})
}
