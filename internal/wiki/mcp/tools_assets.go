package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	httpinternal "github.com/perber/wiki/internal/http"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
)

func (r *Routes) registerAssetTools(server *sdkmcp.Server, opts httpinternal.RouterOptions) {
	addEditorTool[uploadAssetInput, uploadAssetOutput](r, server, toolUploadAsset, func(ctx context.Context, actor toolActor, in uploadAssetInput) (uploadAssetOutput, error) {
		if base64DecodedSize(in.ContentBase64) > opts.MaxAssetUploadSizeBytes {
			return uploadAssetOutput{}, wikiassets.NewAssetFileTooLargeError()
		}
		content, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil {
			return uploadAssetOutput{}, wikiassets.NewAssetInvalidPayloadError(err)
		}
		file := &memoryMultipartFile{Reader: bytes.NewReader(content)}
		out, err := r.uploadAsset.Execute(ctx, wikiassets.UploadAssetInput{
			UserID:   actor.ID,
			PageID:   strings.TrimSpace(in.PageID),
			File:     file,
			Filename: in.Filename,
			MaxBytes: opts.MaxAssetUploadSizeBytes,
		})
		if err != nil {
			return uploadAssetOutput{}, err
		}
		return uploadAssetOutput{File: out.URL}, nil
	})

	addTypedTool[assetInput, assetOutput](server, toolGetAsset, func(ctx context.Context, in assetInput) (assetOutput, error) {
		out, err := r.getAsset.Execute(ctx, wikiassets.GetAssetInput{
			PageID:   strings.TrimSpace(in.PageID),
			Filename: strings.TrimSpace(in.Filename),
		})
		if err != nil {
			return assetOutput{}, err
		}
		return assetOutput{
			Filename:      out.Filename,
			MimeType:      out.MIMEType,
			ContentBase64: base64.StdEncoding.EncodeToString(out.Content),
		}, nil
	})

	addTypedTool[pageIDInput, listAssetsOutput](server, toolListAssets, func(ctx context.Context, in pageIDInput) (listAssetsOutput, error) {
		out, err := r.getAssets.Execute(ctx, wikiassets.ListAssetsInput{PageID: strings.TrimSpace(firstNonEmpty(in.PageID, in.ID))})
		if err != nil {
			return listAssetsOutput{}, err
		}
		return listAssetsOutput{Files: out.Files}, nil
	})

	addEditorTool[renameAssetInput, renameAssetOutput](r, server, toolRenameAsset, func(ctx context.Context, actor toolActor, in renameAssetInput) (renameAssetOutput, error) {
		out, err := r.renameAsset.Execute(ctx, wikiassets.RenameAssetInput{
			UserID:      actor.ID,
			PageID:      strings.TrimSpace(in.PageID),
			OldFilename: in.OldFilename,
			NewFilename: in.NewFilename,
		})
		if err != nil {
			return renameAssetOutput{}, err
		}
		return renameAssetOutput{URL: out.URL}, nil
	})

	addEditorTool[deleteAssetInput, messageOutput](r, server, toolDeleteAsset, func(ctx context.Context, actor toolActor, in deleteAssetInput) (messageOutput, error) {
		if err := r.deleteAsset.Execute(ctx, wikiassets.DeleteAssetInput{
			UserID:   actor.ID,
			PageID:   strings.TrimSpace(in.PageID),
			Filename: in.Filename,
		}); err != nil {
			return messageOutput{}, err
		}
		return messageOutput{Message: "asset deleted"}, nil
	})
}
