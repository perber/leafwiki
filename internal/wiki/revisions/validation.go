package revisions

import (
	"strings"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func ValidateRevisionLookupInput(pageID, revisionID string) (string, string, error) {
	pageID = strings.TrimSpace(pageID)
	revisionID = strings.TrimSpace(revisionID)
	if pageID == "" {
		return "", "", sharederrors.NewLocalizedError(ErrCodeRevisionInvalidPageID, "Page ID is required", "page id is required", nil)
	}
	if revisionID == "" {
		return "", "", sharederrors.NewLocalizedError(ErrCodeRevisionInvalidRevisionID, "Revision ID is required", "revision id is required", nil)
	}
	return pageID, revisionID, nil
}

func ValidateRevisionCompareInput(pageID, baseRevisionID, targetRevisionID string) (string, string, string, error) {
	pageID = strings.TrimSpace(pageID)
	baseRevisionID = strings.TrimSpace(baseRevisionID)
	targetRevisionID = strings.TrimSpace(targetRevisionID)
	if pageID == "" {
		return "", "", "", sharederrors.NewLocalizedError(ErrCodeRevisionInvalidPageID, "Page ID is required", "page id is required", nil)
	}
	if baseRevisionID == "" || targetRevisionID == "" {
		return "", "", "", sharederrors.NewLocalizedError(ErrCodeRevisionCompareInvalidRequest, "Revision compare request is invalid", "revision compare request for page %s is invalid", nil, pageID)
	}
	return pageID, baseRevisionID, targetRevisionID, nil
}

func ValidateRevisionAssetInput(pageID, revisionID, assetName string) (string, string, string, error) {
	pageID, revisionID, err := ValidateRevisionLookupInput(pageID, revisionID)
	if err != nil {
		return "", "", "", err
	}
	assetName = strings.TrimSpace(strings.TrimPrefix(assetName, "/"))
	if assetName == "" {
		return "", "", "", sharederrors.NewLocalizedError(ErrCodeRevisionPreviewAssetInvalidName, "Revision asset name is invalid", "revision asset name for page %s revision %s is invalid", nil, pageID, revisionID)
	}
	return pageID, revisionID, assetName, nil
}
