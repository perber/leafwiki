package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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

func ToAPIPage(p *tree.Page) *Page {
	return &Page{
		PageNode: p.PageNode,
		Content:  p.Content,
		Path:     buildPathFromNode(p.PageNode),
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

func ToAPINode(node *tree.PageNode, parentPath string) *Node {
	path := node.Slug

	if node.Slug == "root" {
		path = ""
	}

	if node.Slug != "root" && parentPath != "" {
		path = parentPath + "/" + node.Slug
	}

	apiNode := &Node{
		ID:       node.ID,
		Title:    node.Title,
		Slug:     node.Slug,
		Path:     path,
		Position: node.Position,
	}

	for _, child := range node.Children {
		apiNode.Children = append(apiNode.Children, ToAPINode(child, path))
	}

	return apiNode
}
