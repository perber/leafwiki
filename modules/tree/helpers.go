package tree

import "github.com/teris-io/shortid"

// GenerateUniqueID generates a unique ID for a tree entry
func GenerateUniqueID() (string, error) {
	id, err := shortid.Generate()
	if err != nil {
		return "", err
	}

	return id, nil
}

func GeneratePathFromPageNode(entry *PageNode) string {
	path := ""
	if entry.Parent != nil {
		path = GeneratePathFromPageNode(entry.Parent) + "/" + entry.Slug
	} else {
		path = entry.Slug
	}
	return path
}
