package tree

import (
	"errors"
	"os"

	"github.com/perber/wiki/internal/core/treemigration"
)

type migrationNodeAdapter struct {
	node *PageNode
}

func (n *migrationNodeAdapter) ID() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.ID
}

func (n *migrationNodeAdapter) Title() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.Title
}

func (n *migrationNodeAdapter) Slug() string {
	if n == nil || n.node == nil {
		return ""
	}
	return n.node.Slug
}

func (n *migrationNodeAdapter) Kind() string {
	if n == nil || n.node == nil {
		return ""
	}
	return string(n.node.Kind)
}

func (n *migrationNodeAdapter) SetKind(kind string) {
	if n == nil || n.node == nil {
		return
	}
	n.node.Kind = NodeKind(kind)
}

func (n *migrationNodeAdapter) Metadata() treemigration.Metadata {
	if n == nil || n.node == nil {
		return treemigration.Metadata{}
	}
	return treemigration.Metadata{
		CreatedAt:    n.node.Metadata.CreatedAt,
		UpdatedAt:    n.node.Metadata.UpdatedAt,
		CreatorID:    n.node.Metadata.CreatorID,
		LastAuthorID: n.node.Metadata.LastAuthorID,
	}
}

func (n *migrationNodeAdapter) SetMetadata(metadata treemigration.Metadata) {
	if n == nil || n.node == nil {
		return
	}
	n.node.Metadata = PageMetadata{
		CreatedAt:    metadata.CreatedAt,
		UpdatedAt:    metadata.UpdatedAt,
		CreatorID:    metadata.CreatorID,
		LastAuthorID: metadata.LastAuthorID,
	}
}

func (n *migrationNodeAdapter) Children() []treemigration.Node {
	if n == nil || n.node == nil {
		return nil
	}

	children := make([]treemigration.Node, 0, len(n.node.Children))
	for _, child := range n.node.Children {
		children = append(children, &migrationNodeAdapter{node: child})
	}
	return children
}

type migrationStoreAdapter struct {
	store *NodeStore
}

func (a *migrationStoreAdapter) ResolveNode(node treemigration.Node) (*treemigration.ResolvedNode, error) {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return nil, err
	}
	resolved, err := a.store.resolveNode(entry)
	if err != nil {
		return nil, err
	}
	return &treemigration.ResolvedNode{
		Kind:       string(resolved.Kind),
		DirPath:    resolved.DirPath,
		FilePath:   resolved.FilePath,
		HasContent: resolved.HasContent,
	}, nil
}

func (a *migrationStoreAdapter) ContentPathForRead(node treemigration.Node) (string, error) {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return "", err
	}
	return a.store.contentPathForNodeRead(entry)
}

func (a *migrationStoreAdapter) ContentPathForWrite(node treemigration.Node) (string, error) {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return "", err
	}
	return a.store.contentPathForNodeWrite(entry)
}

func (a *migrationStoreAdapter) EnsureSectionIndex(node treemigration.Node) (string, error) {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return "", err
	}
	return a.store.ensureSectionIndex(entry)
}

func (a *migrationStoreAdapter) SaveChildOrder(node treemigration.Node) error {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return err
	}
	return a.store.SaveChildOrder(entry)
}

func (a *migrationStoreAdapter) ReadPageRaw(node treemigration.Node) (string, error) {
	entry, err := unwrapMigrationNode(node)
	if err != nil {
		return "", err
	}
	return a.store.ReadPageRaw(entry)
}

func unwrapMigrationNode(node treemigration.Node) (*PageNode, error) {
	adapted, ok := node.(*migrationNodeAdapter)
	if !ok || adapted == nil || adapted.node == nil {
		return nil, errors.New("invalid migration node")
	}
	return adapted.node, nil
}

func (t *TreeService) migrationDependencies() treemigration.Dependencies {
	var root treemigration.Node
	if t.tree != nil {
		root = &migrationNodeAdapter{node: t.tree}
	}

	return treemigration.Dependencies{
		Root:                 root,
		Store:                &migrationStoreAdapter{store: t.store},
		Log:                  t.log,
		CurrentSchemaVersion: CurrentSchemaVersion,
		SaveTree:             t.saveTreeLocked,
		SaveSchema: func(version int) error {
			return saveSchema(t.storageDir, version)
		},
		IsMissingContentErr: func(err error) bool {
			return errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrFileNotFound)
		},
	}
}
