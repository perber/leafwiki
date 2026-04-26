package treemigration

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
)

const (
	NodeKindPage    = "page"
	NodeKindSection = "section"
)

type Metadata struct {
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatorID    string
	LastAuthorID string
}

type ResolvedNode struct {
	Kind       string
	DirPath    string
	FilePath   string
	HasContent bool
}

type Node interface {
	ID() string
	Title() string
	Slug() string
	Kind() string
	SetKind(kind string)
	Metadata() Metadata
	SetMetadata(metadata Metadata)
	Children() []Node
}

type Store interface {
	ResolveNode(node Node) (*ResolvedNode, error)
	ContentPathForRead(node Node) (string, error)
	ContentPathForWrite(node Node) (string, error)
	EnsureSectionIndex(node Node) (string, error)
	ReadPageRaw(node Node) (string, error)
	SaveChildOrder(node Node) error
}

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Dependencies struct {
	Root                 Node
	Store                Store
	Log                  Logger
	CurrentSchemaVersion int
	SaveTree             func() error
	SaveSchema           func(version int) error
	IsMissingContentErr  func(err error) bool
}

func Run(fromVersion int, deps Dependencies) error {
	if err := validateDependencies(fromVersion, deps); err != nil {
		return err
	}

	for version := fromVersion; version < deps.CurrentSchemaVersion; version++ {
		migration, err := migrationForVersion(version)
		if err != nil {
			return err
		}

		if err := migration(deps); err != nil {
			deps.Log.Error("Error migrating schema version", "fromVersion", version, "toVersion", version+1, "error", err)
			return err
		}

		if err := deps.SaveTree(); err != nil {
			deps.Log.Error("Error saving tree after migration", "version", version+1, "error", err)
			return err
		}

		if err := deps.SaveSchema(version + 1); err != nil {
			deps.Log.Error("Error saving schema", "version", version+1, "error", err)
			return err
		}
	}

	return nil
}

func validateDependencies(fromVersion int, deps Dependencies) error {
	if fromVersion < 0 {
		return fmt.Errorf("invalid schema version: %d", fromVersion)
	}
	if deps.Root == nil {
		return errors.New("tree not loaded")
	}
	if deps.Store == nil {
		return errors.New("migration store is required")
	}
	if deps.Log == nil {
		return errors.New("migration logger is required")
	}
	if deps.SaveTree == nil {
		return errors.New("save tree callback is required")
	}
	if deps.SaveSchema == nil {
		return errors.New("save schema callback is required")
	}
	if deps.CurrentSchemaVersion < fromVersion {
		return fmt.Errorf("current schema version %d is older than stored version %d", deps.CurrentSchemaVersion, fromVersion)
	}

	for version := fromVersion; version < deps.CurrentSchemaVersion; version++ {
		if _, err := migrationForVersion(version); err != nil {
			return err
		}
	}

	return nil
}

func migrationForVersion(version int) (func(Dependencies) error, error) {
	switch version {
	case 0:
		return migrateToV1, nil
	case 1:
		return migrateToV2, nil
	case 2:
		return migrateToV3, nil
	case 3:
		return migrateToV4, nil
	case 4:
		return migrateToV5, nil
	default:
		return nil, fmt.Errorf("unsupported schema migration version: %d", version)
	}
}

func migrateToV1(deps Dependencies) error {
	return backfillMetadata(deps, deps.Root)
}

func backfillMetadata(deps Dependencies, node Node) error {
	if node == nil {
		return nil
	}

	if !node.Metadata().CreatedAt.IsZero() {
		return nil
	}

	resolved, err := deps.Store.ResolveNode(node)
	if err != nil {
		deps.Log.Error("Could not resolve node for metadata backfill", "nodeID", node.ID(), "error", err)
		return nil
	}

	statPath := resolved.FilePath
	if resolved.Kind == NodeKindSection && !resolved.HasContent {
		statPath = resolved.DirPath
	}

	createdAt := time.Now().UTC()
	updatedAt := time.Now().UTC()

	if statPath != "" {
		info, err := os.Stat(statPath)
		if err == nil {
			createdAt = info.ModTime().UTC()
			updatedAt = info.ModTime().UTC()
		} else if !os.IsNotExist(err) {
			deps.Log.Error("Could not stat node for metadata", "nodeID", node.ID(), "path", statPath, "error", err)
		}
	}

	previous := node.Metadata()
	node.SetMetadata(Metadata{
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		CreatorID:    previous.CreatorID,
		LastAuthorID: previous.LastAuthorID,
	})

	for _, child := range node.Children() {
		if err := backfillMetadata(deps, child); err != nil {
			return err
		}
	}

	return nil
}

func migrateToV5(deps Dependencies) error {
	return backfillChildOrder(deps, deps.Root)
}

func backfillChildOrder(deps Dependencies, node Node) error {
	if node == nil {
		return nil
	}

	children := node.Children()
	if len(children) > 0 {
		if node.ID() != "root" && node.Kind() != NodeKindSection {
			// Legacy snapshots could have a page node with children when a folder and an
			// .md file shared the same name. Coerce to section so SaveChildOrder can write
			// the .order.json and preserve the legacy child ordering. ReconstructTreeFromFS
			// will derive the correct kind from the filesystem after migration completes.
			deps.Log.Warn("coercing non-section node with children to section for child order backfill — likely caused by a folder and .md file sharing the same name in a previous version",
				"nodeID", node.ID(), "slug", node.Slug(), "kind", node.Kind())
			node.SetKind(NodeKindSection)
		}
		if err := deps.Store.SaveChildOrder(node); err != nil {
			return fmt.Errorf("persist child order for node %s: %w", node.ID(), err)
		}
	}

	for _, child := range children {
		if err := backfillChildOrder(deps, child); err != nil {
			return err
		}
	}

	return nil
}

func migrateToV2(deps Dependencies) error {
	backfillKindFromFS(deps, deps.Root)

	for _, child := range deps.Root.Children() {
		if err := addFrontmatter(deps, child); err != nil {
			deps.Log.Error("Error adding frontmatter to child node", "nodeID", child.ID(), "error", err)
			return err
		}
	}

	return nil
}

func backfillKindFromFS(deps Dependencies, root Node) {
	if root == nil {
		return
	}

	root.SetKind(NodeKindSection)

	var walk func(node Node)
	walk = func(node Node) {
		if node == nil {
			return
		}

		if node.ID() != "root" && node.Kind() != NodeKindPage && node.Kind() != NodeKindSection {
			resolved, err := deps.Store.ResolveNode(node)
			if err == nil {
				node.SetKind(resolved.Kind)
			} else {
				if len(node.Children()) > 0 {
					node.SetKind(NodeKindSection)
				} else {
					node.SetKind(NodeKindPage)
				}
				deps.Log.Warn("could not resolve node on disk; kind backfilled by heuristic",
					"nodeID", node.ID(), "slug", node.Slug(), "err", err, "kind", node.Kind())
			}
		}

		for _, child := range node.Children() {
			walk(child)
		}
	}

	for _, child := range root.Children() {
		walk(child)
	}
}

func addFrontmatter(deps Dependencies, node Node) error {
	content, err := deps.Store.ReadPageRaw(node)
	if err != nil {
		if deps.IsMissingContentErr != nil && deps.IsMissingContentErr(err) {
			deps.Log.Warn("Page file does not exist, skipping frontmatter addition", "nodeID", node.ID())
			for _, child := range node.Children() {
				if err := addFrontmatter(deps, child); err != nil {
					deps.Log.Error("Error adding frontmatter to child node", "nodeID", child.ID(), "error", err)
					return err
				}
			}
			return nil
		}

		deps.Log.Error("Could not read page content for node", "nodeID", node.ID(), "error", err)
		return fmt.Errorf("could not read page content for node %s: %w", node.ID(), err)
	}

	filePath, err := deps.Store.ContentPathForWrite(node)
	if err != nil {
		return fmt.Errorf("could not determine content path for node %s: %w", node.ID(), err)
	}

	mdFile := markdown.NewMarkdownFile(filePath, content, markdown.Frontmatter{})
	if raw := strings.TrimSpace(content); raw != "" {
		mdFile, err = markdown.NewMarkdownFileFromRaw(filePath, content)
		if err != nil {
			deps.Log.Error("Could not parse markdown content for node", "nodeID", node.ID(), "error", err)
			return fmt.Errorf("could not parse markdown content for node %s: %w", node.ID(), err)
		}
	}

	fm := mdFile.GetFrontmatter()
	changed := false

	if strings.TrimSpace(fm.LeafWikiID) == "" {
		fm.LeafWikiID = node.ID()
		changed = true
	}
	if strings.TrimSpace(fm.LeafWikiTitle) == "" {
		fm.LeafWikiTitle = node.Title()
		changed = true
	}

	if changed {
		mdFile.SetLeafWikiFrontmatter(fm.LeafWikiID, fm.LeafWikiTitle)
		if err := mdFile.WriteToFile(); err != nil {
			deps.Log.Error("could not write updated page content", "nodeID", node.ID(), "filePath", filePath, "error", err)
			return fmt.Errorf("could not write updated page content for node %s: %w", node.ID(), err)
		}

		deps.Log.Info("frontmatter backfilled", "nodeID", node.ID(), "path", filePath)
	}

	for _, child := range node.Children() {
		if err := addFrontmatter(deps, child); err != nil {
			deps.Log.Error("Error adding frontmatter to child node", "nodeID", child.ID(), "error", err)
			return err
		}
	}

	return nil
}

func migrateToV3(deps Dependencies) error {
	return backfillMetadataFrontmatter(deps, deps.Root)
}

func backfillMetadataFrontmatter(deps Dependencies, node Node) error {
	if node == nil {
		return nil
	}

	filePath, err := deps.Store.ContentPathForRead(node)
	if err != nil {
		return fmt.Errorf("could not determine content path for node %s: %w", node.ID(), err)
	}

	if fileExists(filePath) {
		mdFile, err := markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf("could not load markdown file for node %s: %w", node.ID(), err)
		}

		metadata := node.Metadata()
		mdFile.SetLeafWikiMetadata(
			formatMetadataTime(metadata.CreatedAt),
			formatMetadataTime(metadata.UpdatedAt),
			strings.TrimSpace(metadata.CreatorID),
			strings.TrimSpace(metadata.LastAuthorID),
		)
		if err := mdFile.WriteToFile(); err != nil {
			return fmt.Errorf("could not write migrated metadata for node %s: %w", node.ID(), err)
		}
	}

	for _, child := range node.Children() {
		if err := backfillMetadataFrontmatter(deps, child); err != nil {
			return err
		}
	}

	return nil
}

func migrateToV4(deps Dependencies) error {
	return materializeSectionIndexes(deps, deps.Root)
}

func materializeSectionIndexes(deps Dependencies, node Node) error {
	if node == nil {
		return nil
	}

	if node.ID() != "root" && node.Kind() == NodeKindSection {
		if _, err := deps.Store.EnsureSectionIndex(node); err != nil {
			return fmt.Errorf("could not materialize section index for node %s: %w", node.ID(), err)
		}
	}

	for _, child := range node.Children() {
		if err := materializeSectionIndexes(deps, child); err != nil {
			return err
		}
	}

	return nil
}

func formatMetadataTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
