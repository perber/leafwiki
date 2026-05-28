package mcp

import (
	"context"
	"encoding/base64"
	"os"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
)

func (r *Routes) registerRevisionTools(server *sdkmcp.Server) {
	addTypedTool[listRevisionsInput, listRevisionsOutput](server, toolListRevisions, func(ctx context.Context, in listRevisionsInput) (listRevisionsOutput, error) {
		pageID := strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))
		limit, err := wikirevisions.NormalizeRevisionListLimit(in.Limit, pageID)
		if err != nil {
			return listRevisionsOutput{}, err
		}
		out, err := r.listRevs.Execute(ctx, wikirevisions.ListRevisionsInput{
			PageID: pageID,
			Cursor: strings.TrimSpace(in.Cursor),
			Limit:  limit,
		})
		if err != nil {
			return listRevisionsOutput{}, err
		}
		revisions := make([]*wikirevisions.RevisionResponse, 0, len(out.Revisions))
		for _, rev := range out.Revisions {
			revisions = append(revisions, wikirevisions.ToRevisionResponse(rev, r.userResolver))
		}
		return listRevisionsOutput{Revisions: revisions, NextCursor: out.NextCursor}, nil
	})

	addTypedTool[pageIDInput, revisionOutput](server, toolGetLatestRevision, func(ctx context.Context, in pageIDInput) (revisionOutput, error) {
		pageID := strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))
		out, err := r.getLatestRev.Execute(ctx, wikirevisions.GetLatestRevisionInput{PageID: pageID})
		if err != nil {
			return revisionOutput{}, err
		}
		if out.Revision == nil {
			return revisionOutput{}, wikirevisions.NewRevisionNotFoundError("Revision not found", "revision for page %s not found", pageID)
		}
		return revisionOutput{Revision: wikirevisions.ToRevisionResponse(out.Revision, r.userResolver)}, nil
	})

	addTypedTool[revisionIDInput, *wikirevisions.RevisionSnapshotResponse](server, toolGetRevision, func(ctx context.Context, in revisionIDInput) (*wikirevisions.RevisionSnapshotResponse, error) {
		pageID, revisionID, err := wikirevisions.ValidateRevisionLookupInput(firstNonEmpty(in.PageID, in.ID), in.RevisionID)
		if err != nil {
			return nil, err
		}
		out, err := r.getRev.Execute(ctx, wikirevisions.GetRevisionInput{
			PageID:     pageID,
			RevisionID: revisionID,
		})
		if err != nil {
			return nil, err
		}
		if out.Snapshot == nil {
			return nil, wikirevisions.NewRevisionNotFoundError("Revision not found", "revision %s for page %s not found", revisionID, pageID)
		}
		return wikirevisions.ToSnapshotResponse(out.Snapshot, r.userResolver), nil
	})

	addTypedTool[compareRevisionsInput, *wikirevisions.RevisionComparisonResponse](server, toolCompareRevisions, func(ctx context.Context, in compareRevisionsInput) (*wikirevisions.RevisionComparisonResponse, error) {
		pageID, baseRevisionID, targetRevisionID, err := wikirevisions.ValidateRevisionCompareInput(firstNonEmpty(in.PageID, in.ID), in.BaseRevisionID, in.TargetRevisionID)
		if err != nil {
			return nil, err
		}
		out, err := r.compareRevs.Execute(ctx, wikirevisions.CompareRevisionsInput{
			PageID:           pageID,
			BaseRevisionID:   baseRevisionID,
			TargetRevisionID: targetRevisionID,
		})
		if err != nil {
			return nil, err
		}
		if out.Comparison == nil {
			return nil, wikirevisions.NewRevisionNotFoundError("Revision not found", "revision compare resource for page %s not found", pageID)
		}
		return wikirevisions.ToComparisonResponse(out.Comparison, r.userResolver), nil
	})

	addTypedTool[revisionAssetInput, assetOutput](server, toolGetRevisionAsset, func(ctx context.Context, in revisionAssetInput) (assetOutput, error) {
		pageID, revisionID, assetName, err := wikirevisions.ValidateRevisionAssetInput(firstNonEmpty(in.PageID, in.ID), in.RevisionID, in.AssetName)
		if err != nil {
			return assetOutput{}, err
		}
		out, err := r.getRevAsset.Execute(ctx, wikirevisions.GetRevisionAssetInput{
			PageID:     pageID,
			RevisionID: revisionID,
			AssetName:  assetName,
		})
		if err != nil {
			return assetOutput{}, err
		}
		if out.Asset == nil {
			return assetOutput{}, wikirevisions.NewRevisionNotFoundError("Revision asset not found", "revision asset %s for page %s revision %s not found", assetName, pageID, revisionID)
		}
		data, err := os.ReadFile(out.Asset.Path)
		if err != nil {
			return assetOutput{}, wikirevisions.NewRevisionAssetBlobUnavailableError(assetName, pageID, revisionID, err)
		}
		mimeType := wikirevisions.DetectRevisionAssetMIMEType(out.Asset.Asset.Name, out.Asset.Asset.MIMEType)
		return assetOutput{
			Filename:      out.Asset.Asset.Name,
			MimeType:      mimeType,
			ContentBase64: base64.StdEncoding.EncodeToString(data),
		}, nil
	})

	addTypedTool[revisionIDInput, pageOutput](server, toolRestoreRevision, func(ctx context.Context, in revisionIDInput) (pageOutput, error) {
		out, err := r.restoreRev.Execute(ctx, wikirevisions.RestoreRevisionInput{
			UserID:     publicEditorID,
			PageID:     strings.TrimSpace(firstNonEmpty(in.PageID, in.ID)),
			RevisionID: strings.TrimSpace(in.RevisionID),
		})
		if err != nil {
			return pageOutput{}, err
		}
		return pageOutput{Page: r.apiPage(out.Page, 0)}, nil
	})
}
