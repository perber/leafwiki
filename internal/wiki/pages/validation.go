package pages

import (
	"strings"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

func ValidatePageRoutePath(routePath string) (string, error) {
	routePath = strings.TrimSpace(routePath)
	if routePath == "" {
		return "", sharederrors.NewLocalizedError(ErrCodePageMissingPath, "Missing path", "missing path", nil)
	}
	if strings.Contains(routePath, `\`) {
		return "", sharederrors.NewLocalizedError(ErrCodePageInvalidPath, "Invalid path", "invalid path %s", nil, routePath)
	}
	for _, segment := range strings.Split(routePath, "/") {
		if segment == "" || segment == "." || segment == ".." || strings.TrimSpace(segment) != segment {
			return "", sharederrors.NewLocalizedError(ErrCodePageInvalidPath, "Invalid path", "invalid path %s", nil, routePath)
		}
	}
	return routePath, nil
}

func ValidatePageKind(kind *string) (tree.NodeKind, error) {
	if kind == nil {
		return tree.NodeKindPage, nil
	}
	switch *kind {
	case string(tree.NodeKindPage), string(tree.NodeKindSection):
		return tree.NodeKind(*kind), nil
	default:
		return "", sharederrors.NewLocalizedError(ErrCodePageInvalidKind, "Invalid kind", "invalid kind", nil)
	}
}

func ValidatePageKindString(kind string) (tree.NodeKind, error) {
	return ValidatePageKind(&kind)
}

func ValidateRefactorKind(kind string) (string, error) {
	switch kind {
	case RefactorKindRename, RefactorKindMove:
		return kind, nil
	default:
		return "", sharederrors.NewLocalizedError(ErrCodePageInvalidRefactorKind, "Invalid refactor kind", "invalid refactor kind", nil)
	}
}

func ValidateMoveParentID(parentID string) (string, error) {
	if parentID == "" || parentID == "root" {
		return parentID, nil
	}
	if strings.TrimSpace(parentID) != parentID {
		return "", sharederrors.NewLocalizedError(ErrCodePageInvalidParentID, "Invalid parentId", "invalid parent id", nil)
	}
	return parentID, nil
}

func ValidateOptionalParentID(parentID *string) (*string, error) {
	if parentID == nil {
		return nil, nil
	}
	validated, err := ValidateMoveParentID(*parentID)
	if err != nil {
		return nil, err
	}
	return &validated, nil
}
