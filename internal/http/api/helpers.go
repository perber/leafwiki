package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	verrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

func respondWithError(c *gin.Context, err error) {
	var vErr *verrors.ValidationErrors
	if errors.As(err, &vErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation_error",
			"fields": vErr.Errors,
		})
		return
	}

	switch {
	case errors.Is(err, tree.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
	case errors.Is(err, tree.ErrParentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent page not found"})
	case errors.Is(err, tree.ErrPageHasChildren):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page has children, use recursive delete"})
	case errors.Is(err, tree.ErrTreeNotLoaded):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tree not loaded"})
	case errors.Is(err, tree.ErrPageAlreadyExists):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page already exists"})
	case errors.Is(err, tree.ErrMovePageCircularReference):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Move would create a circular reference"})
	case errors.Is(err, tree.ErrPageCannotBeMovedToItself):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page cannot be moved to itself"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func ToAPIPage(p *tree.Page, userResolver *auth.UserResolver) *Page {
	return &Page{
		Node:    ToAPINode(p.PageNode, "", userResolver),
		Content: p.Content,
		Path:    buildPathFromNode(p.PageNode),
	}
}

func buildPathFromNode(node *tree.PageNode) string {
	var parts []string
	current := node
	for current != nil && current.Slug != "root" {
		parts = append([]string{current.Slug}, parts...)
		current = current.Parent
	}
	return strings.Join(parts, "/")
}

func ToAPINode(node *tree.PageNode, parentPath string, userResolver *auth.UserResolver) *Node {
	path := node.Slug

	if node.Slug == "root" {
		path = ""
	}

	if node.Slug != "root" && parentPath != "" {
		path = parentPath + "/" + node.Slug
	}

	var creator, lastAuthor *auth.UserLabel
	if userResolver != nil {
		creator, _ = userResolver.ResolveUserLabel(node.Metadata.CreatorID)
		lastAuthor, _ = userResolver.ResolveUserLabel(node.Metadata.LastAuthorID)
	}

	apiNode := &Node{
		ID:       node.ID,
		Title:    node.Title,
		Slug:     node.Slug,
		Path:     path,
		Position: node.Position,
		Metadata: NodeMetadata{
			CreatedAt:    node.Metadata.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    node.Metadata.UpdatedAt.Format(time.RFC3339),
			CreatorID:    node.Metadata.CreatorID,
			LastAuthorID: node.Metadata.LastAuthorID,
			Creator:      creator,
			LastAuthor:   lastAuthor,
		},
	}

	for _, child := range node.Children {
		apiNode.Children = append(apiNode.Children, ToAPINode(child, path, userResolver))
	}

	return apiNode
}
