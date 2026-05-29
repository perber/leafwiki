package mcp

import (
	"context"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	wikilinks "github.com/perber/wiki/internal/wiki/links"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

func (r *Routes) registerPageTools(server *sdkmcp.Server) {
	addTypedTool[getTreeInput, treeOutput](server, toolGetTree, func(_ context.Context, in getTreeInput) (treeOutput, error) {
		root := r.treeService.GetTree()
		if root == nil {
			return treeOutput{}, nil
		}
		if in.Depth != nil {
			return treeOutput{Tree: dto.ToAPINodeWithDepth(root, "", r.userResolver, *in.Depth)}, nil
		}
		return treeOutput{Tree: dto.ToAPINode(root, "", r.userResolver)}, nil
	})

	addTypedTool[pageIDInput, pageOutput](server, toolGetPage, func(ctx context.Context, in pageIDInput) (pageOutput, error) {
		out, err := r.getPage.Execute(ctx, wikipages.GetPageInput{ID: strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))})
		if err != nil {
			return pageOutput{}, err
		}
		return r.pageOutputWithLinkStatus(ctx, out.Page, 0)
	})

	addTypedTool[pathInput, pageOutput](server, toolGetPageByPath, func(ctx context.Context, in pathInput) (pageOutput, error) {
		routePath, err := wikipages.ValidatePageRoutePath(in.Path)
		if err != nil {
			return pageOutput{}, err
		}
		out, err := r.findByPath.Execute(ctx, wikipages.FindByPathInput{RoutePath: routePath})
		if err != nil {
			return pageOutput{}, err
		}
		depth := 0
		if out.Page.Kind == tree.NodeKindSection {
			depth = 1
		}
		return r.pageOutputWithLinkStatus(ctx, out.Page, depth)
	})

	addTypedTool[pathInput, lookupPathOutput](server, toolLookupPath, func(ctx context.Context, in pathInput) (lookupPathOutput, error) {
		out, err := r.lookupPath.Execute(ctx, wikipages.LookupPagePathInput{Path: strings.TrimSpace(in.Path)})
		if err != nil {
			return lookupPathOutput{}, err
		}
		return lookupPathOutput{Lookup: out.Lookup}, nil
	})

	addTypedTool[pageIDInput, resolvePermalinkOutput](server, toolResolvePermalink, func(ctx context.Context, in pageIDInput) (resolvePermalinkOutput, error) {
		out, err := r.resolveLink.Execute(ctx, wikipages.ResolvePermalinkInput{ID: strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))})
		if err != nil {
			return resolvePermalinkOutput{}, err
		}
		return resolvePermalinkOutput{Target: out.Target}, nil
	})

	addEditorTool[suggestSlugInput, suggestSlugOutput](r, server, toolSuggestSlug, func(ctx context.Context, _ toolActor, in suggestSlugInput) (suggestSlugOutput, error) {
		title, err := wikipages.ValidateSuggestSlugTitle(in.Title)
		if err != nil {
			return suggestSlugOutput{}, err
		}
		out, err := r.suggestSlug.Execute(ctx, wikipages.SuggestSlugInput{
			ParentID:  strings.TrimSpace(in.ParentID),
			CurrentID: strings.TrimSpace(in.CurrentID),
			Title:     title,
		})
		if err != nil {
			return suggestSlugOutput{}, err
		}
		return suggestSlugOutput{Slug: out.Slug}, nil
	})

	addEditorTool[createPageInput, pageOutput](r, server, toolCreatePage, func(ctx context.Context, actor toolActor, in createPageInput) (pageOutput, error) {
		kind, err := wikipages.ValidatePageKind(in.Kind)
		if err != nil {
			return pageOutput{}, err
		}
		out, err := r.createPage.Execute(ctx, wikipages.CreatePageInput{
			UserID:   actor.ID,
			ParentID: in.ParentID,
			Title:    in.Title,
			Slug:     in.Slug,
			Kind:     &kind,
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(out.Page, 0)}, nil
	})

	addEditorTool[updatePageInput, pageOutput](r, server, toolUpdatePage, func(ctx context.Context, actor toolActor, in updatePageInput) (pageOutput, error) {
		if err := wikipages.ValidatePageMetadataInput(in.Tags, in.Properties); err != nil {
			return pageOutput{}, err
		}
		contentToSave := in.Content
		fromImport := false
		if in.Content != nil {
			combined, err := markdown.BuildMarkdownWithExtraFrontmatter(wikipages.BuildExtraFields(in.Tags, in.Properties), *in.Content)
			if err != nil {
				return pageOutput{}, err
			}
			contentToSave = &combined
			fromImport = true
		}
		kind := tree.NodeKindPage
		out, err := r.updatePage.Execute(ctx, wikipages.UpdatePageInput{
			UserID:     actor.ID,
			ID:         strings.TrimSpace(in.ID),
			Version:    strings.TrimSpace(in.Version),
			Title:      in.Title,
			Slug:       in.Slug,
			Content:    contentToSave,
			Kind:       &kind,
			FromImport: fromImport,
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(out.Page, 0)}, nil
	})

	addEditorTool[deletePageInput, messageOutput](r, server, toolDeletePage, func(ctx context.Context, actor toolActor, in deletePageInput) (messageOutput, error) {
		if err := r.deletePage.Execute(ctx, wikipages.DeletePageInput{
			UserID:    actor.ID,
			ID:        strings.TrimSpace(in.ID),
			Version:   strings.TrimSpace(in.Version),
			Recursive: in.Recursive,
		}); err != nil {
			return messageOutput{}, err
		}
		return messageOutput{Message: "Page deleted"}, nil
	})

	addEditorTool[movePageInput, messageOutput](r, server, toolMovePage, func(ctx context.Context, actor toolActor, in movePageInput) (messageOutput, error) {
		parentID := ""
		if in.ParentID != nil {
			parentID = *in.ParentID
		}
		if err := r.movePage.Execute(ctx, wikipages.MovePageInput{
			UserID:   actor.ID,
			ID:       strings.TrimSpace(in.ID),
			Version:  strings.TrimSpace(in.Version),
			ParentID: parentID,
		}); err != nil {
			return messageOutput{}, err
		}
		return messageOutput{Message: "Page moved"}, nil
	})

	addEditorTool[sortPagesInput, messageOutput](r, server, toolSortPages, func(ctx context.Context, _ toolActor, in sortPagesInput) (messageOutput, error) {
		if err := r.sortPages.Execute(ctx, wikipages.SortPagesInput{
			ParentID:   strings.TrimSpace(in.ParentID),
			OrderedIDs: in.OrderedIDs,
		}); err != nil {
			return messageOutput{}, err
		}
		return messageOutput{Message: "Pages sorted successfully"}, nil
	})

	addEditorTool[ensurePageInput, pageOutput](r, server, toolEnsurePage, func(ctx context.Context, actor toolActor, in ensurePageInput) (pageOutput, error) {
		kind, err := wikipages.ValidatePageKind(in.Kind)
		if err != nil {
			return pageOutput{}, err
		}
		out, err := r.ensurePath.Execute(ctx, wikipages.EnsurePathInput{
			UserID:      actor.ID,
			TargetPath:  strings.TrimSpace(in.Path),
			TargetTitle: in.Title,
			Kind:        &kind,
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(out.Page, 0)}, nil
	})

	addEditorTool[convertPageInput, messageOutput](r, server, toolConvertPage, func(ctx context.Context, actor toolActor, in convertPageInput) (messageOutput, error) {
		targetKind, err := wikipages.ValidateConvertTargetKind(in.TargetKind)
		if err != nil {
			return messageOutput{}, err
		}
		if err := r.convertPage.Execute(ctx, wikipages.ConvertPageInput{
			UserID:     actor.ID,
			ID:         strings.TrimSpace(in.ID),
			Version:    strings.TrimSpace(in.Version),
			TargetKind: targetKind,
		}); err != nil {
			return messageOutput{}, err
		}
		return messageOutput{Message: "Page converted"}, nil
	})

	addEditorTool[copyPageInput, pageOutput](r, server, toolCopyPage, func(ctx context.Context, actor toolActor, in copyPageInput) (pageOutput, error) {
		out, err := r.copyPage.Execute(ctx, wikipages.CopyPageInput{
			UserID:         actor.ID,
			SourcePageID:   strings.TrimSpace(in.ID),
			TargetParentID: in.TargetParentID,
			Title:          in.Title,
			Slug:           in.Slug,
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(out.Page, 0)}, nil
	})
}

func (r *Routes) pageOutputWithLinkStatus(ctx context.Context, page *tree.Page, depth int) (pageOutput, error) {
	out, err := r.linkStatus.Execute(ctx, wikilinks.GetLinkStatusInput{PageID: page.ID})
	if err != nil {
		return pageOutput{}, err
	}
	return pageOutput{Page: r.apiPage(page, depth), LinkStatus: out.Status}, nil
}
