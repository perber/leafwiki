package tree

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/shared"
)

const legacyTreeFilename = "tree.json"
const migratedLegacyTreeFilename = "tree.json.migrated.bak"
const fsMigrationMarker = ".leafwiki_fs_migrated"

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

type NodeStore struct {
	storageDir string
	log        *slog.Logger
	slugger    *SlugService
}

type sectionOrderFile struct {
	Children []string `json:"children"`
}

func NewNodeStore(storageDir string) *NodeStore {
	return &NodeStore{
		storageDir: storageDir,
		log:        slog.Default().With("component", "NodeStore"),
		slugger:    NewSlugService(),
	}
}

func (f *NodeStore) migratedLegacyTreePath() string {
	return filepath.Join(f.storageDir, migratedLegacyTreeFilename)
}

func (f *NodeStore) legacyTreePath() string {
	return filepath.Join(f.storageDir, legacyTreeFilename)
}

func (f *NodeStore) migrationMarkerPath() string {
	return filepath.Join(f.storageDir, fsMigrationMarker)
}

func (f *NodeStore) HasLegacyTreeJSON() bool {
	return fileExists(f.legacyTreePath())
}

func (f *NodeStore) HasFSMigrationMarker() bool {
	return fileExists(f.migrationMarkerPath())
}

func (f *NodeStore) WriteFSMigrationMarker() error {
	return os.WriteFile(f.migrationMarkerPath(), []byte("ok\n"), 0o644)
}

func (f *NodeStore) ArchiveLegacyTreeJSON() error {
	src := f.legacyTreePath()
	dst := f.migratedLegacyTreePath()

	if !fileExists(src) {
		return nil
	}
	if fileExists(dst) {
		return fmt.Errorf("cannot archive legacy tree.json: destination already exists: %s", dst)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("archive legacy tree.json: %w", err)
	}

	return nil
}

func (f *NodeStore) LoadLegacyTree() (*legacyPageNode, error) {
	path := f.legacyTreePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root legacyPageNode
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("unmarshal legacy tree.json: %w", err)
	}

	return &root, nil
}

func (f *NodeStore) RootDirHasContent() bool {
	rootDir := filepath.Join(f.storageDir, "root")
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func (f *NodeStore) ReconstructTreeFromFS() (*PageNode, error) {
	root := &PageNode{
		ID:       "root",
		Slug:     "root",
		Title:    "root",
		Parent:   nil,
		Children: []*PageNode{},
		Kind:     NodeKindSection,
	}

	rootDir := filepath.Join(f.storageDir, "root")

	info, err := os.Stat(rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No on-disk content yet; return an empty root tree.
			return root, nil
		}
		return nil, fmt.Errorf("stat root dir %s: %w", rootDir, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("root path %s is not a directory", rootDir)
	}

	if err := f.reconstructTreeRecursive(rootDir, root); err != nil {
		return nil, fmt.Errorf("reconstruct tree from fs: %w", err)
	}

	f.markDuplicateIDs(root)

	return root, nil
}

func (f *NodeStore) reconstructTreeRecursive(currentPath string, parent *PageNode) error {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", currentPath, err)
	}

	// stable, deterministic ordering (case-insensitive, with case-sensitive tie-breaker)
	sort.SliceStable(entries, func(i, j int) bool {
		li := strings.ToLower(entries[i].Name())
		lj := strings.ToLower(entries[j].Name())
		if li == lj {
			return entries[i].Name() < entries[j].Name()
		}
		return li < lj
	})

	var children []*PageNode
	for _, entry := range entries {
		name := entry.Name()

		// ignore hidden files and folders (those starting with .)
		if strings.HasPrefix(name, ".") {
			continue
		}

		if strings.EqualFold(name, "order.json") {
			continue
		}

		title := name
		id := ""
		repairNeeded := false
		var issues []NodeIssue
		if entry.IsDir() {
			normalizedSlug := normalizeSlug(name)
			if err := f.slugger.IsValidSlug(normalizedSlug); err != nil {
				f.log.Error("skipping directory with invalid slug", "directory", name, "normalized", normalizedSlug, "error", err)
				continue
			}

			indexPath := filepath.Join(currentPath, name, "index.md")
			if fileExists(indexPath) {
				mdFile, err := markdown.LoadMarkdownFile(indexPath)
				if err != nil {
					f.log.Error("could not load index.md", "path", indexPath, "error", err)
					repairNeeded = true
					issues = append(issues, NodeIssue{
						Code:    NodeIssueMissingIndexMD,
						Message: "index.md exists but could not be parsed",
					})
				} else {
					title, err = mdFile.GetTitle()
					if err != nil {
						f.log.Error("could not extract title from index.md", "path", indexPath, "error", err)
					}
					if mdFile.GetFrontmatter().LeafWikiID != "" {
						id = mdFile.GetFrontmatter().LeafWikiID
					}
				}
			} else {
				repairNeeded = true
				issues = append(issues, NodeIssue{
					Code:    NodeIssueMissingIndexMD,
					Message: "section has no index.md",
				})
			}

			if strings.TrimSpace(id) == "" {
				id = syntheticNodeID(filepath.Join(currentPath, name))
				repairNeeded = true
				issues = append(issues, NodeIssue{
					Code:    NodeIssueMissingID,
					Message: "section is missing leafwiki_id in index.md frontmatter",
				})
			}

			child := &PageNode{
				ID:           id,
				Slug:         normalizedSlug,
				Title:        title,
				Parent:       parent,
				Children:     []*PageNode{},
				Kind:         NodeKindSection,
				RepairNeeded: repairNeeded,
				Issues:       issues,
			}
			children = append(children, child)

			if err := f.reconstructTreeRecursive(filepath.Join(currentPath, name), child); err != nil {
				return err
			}
			continue
		}

		// file
		ext := filepath.Ext(name)
		if !strings.EqualFold(ext, ".md") {
			continue
		}

		baseFilename := strings.TrimSuffix(name, ext)
		if strings.EqualFold(baseFilename, "index") {
			continue
		}
		normalizedSlug := normalizeSlug(baseFilename)
		if err := f.slugger.IsValidSlug(normalizedSlug); err != nil {
			f.log.Error("skipping file with invalid slug", "file", name, "normalized", normalizedSlug, "error", err)
			continue
		}

		filePath := filepath.Join(currentPath, name)

		mdFile, err := markdown.LoadMarkdownFile(filePath)
		if err != nil {
			f.log.Error("could not load markdown file", "path", filePath, "error", err)
			continue
		}
		title, err = mdFile.GetTitle()
		if err != nil {
			f.log.Error("could not extract title from file", "path", filePath, "error", err)
			continue
		}
		if mdFile.GetFrontmatter().LeafWikiID != "" {
			id = mdFile.GetFrontmatter().LeafWikiID
		} else {
			id = syntheticNodeID(filePath)
			repairNeeded = true
			issues = append(issues, NodeIssue{
				Code:    NodeIssueMissingID,
				Message: "page is missing leafwiki_id in frontmatter",
			})
		}

		child := &PageNode{
			ID:           id,
			Slug:         normalizedSlug,
			Title:        title,
			Parent:       parent,
			Children:     nil,
			Kind:         NodeKindPage,
			RepairNeeded: repairNeeded,
			Issues:       issues,
		}
		children = append(children, child)
	}

	parent.Children = f.sortChildrenForParent(currentPath, children, parent)
	return nil
}

func (f *NodeStore) MigrateLegacyTreeJSONToFS() error {
	if !f.HasLegacyTreeJSON() {
		return nil
	}
	if f.HasFSMigrationMarker() {
		return nil
	}
	if f.RootDirHasContent() {
		return fmt.Errorf("refusing legacy migration: root/ already contains content")
	}

	legacyRoot, err := f.LoadLegacyTree()
	if err != nil {
		return fmt.Errorf("load legacy tree: %w", err)
	}
	if legacyRoot == nil {
		return fmt.Errorf("legacy tree.json is nil")
	}

	rootDir := filepath.Join(f.storageDir, "root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return fmt.Errorf("ensure root dir: %w", err)
	}

	for _, child := range legacyRoot.Children {
		if err := f.migrateLegacyNode(rootDir, child); err != nil {
			return err
		}
	}

	if err := f.writeLegacyOrderFile(rootDir, legacyRoot.Children); err != nil {
		return err
	}

	if err := f.ArchiveLegacyTreeJSON(); err != nil {
		return fmt.Errorf("archive legacy tree.json after migration: %w", err)
	}

	if err := f.WriteFSMigrationMarker(); err != nil {
		return fmt.Errorf("write migration marker: %w", err)
	}

	return nil
}

func (f *NodeStore) migrateLegacyNode(parentDir string, n *legacyPageNode) error {
	if n == nil {
		return nil
	}

	slug := strings.TrimSpace(n.Slug)
	if slug == "" {
		return fmt.Errorf("legacy node %q has empty slug", n.ID)
	}

	switch n.Kind {
	case NodeKindSection:
		dirPath := filepath.Join(parentDir, slug)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			return fmt.Errorf("create section dir %s: %w", dirPath, err)
		}

		indexPath := filepath.Join(dirPath, "index.md")
		if err := f.writeLegacyNodeMarkdown(indexPath, n); err != nil {
			return err
		}

		for _, child := range n.sortedChildren() {
			if err := f.migrateLegacyNode(dirPath, child); err != nil {
				return err
			}
		}

		if err := f.writeLegacyOrderFile(dirPath, n.Children); err != nil {
			return err
		}
		return nil

	case NodeKindPage:
		if len(n.Children) > 0 {
			// Legacy invalid shape: page with children.
			// Migrate safely as section to preserve subtree.
			dirPath := filepath.Join(parentDir, slug)
			if err := os.MkdirAll(dirPath, 0o755); err != nil {
				return fmt.Errorf("create promoted section dir %s: %w", dirPath, err)
			}

			indexPath := filepath.Join(dirPath, "index.md")
			if err := f.writeLegacyNodeMarkdown(indexPath, n); err != nil {
				return err
			}

			for _, child := range n.sortedChildren() {
				if err := f.migrateLegacyNode(dirPath, child); err != nil {
					return err
				}
			}

			if err := f.writeLegacyOrderFile(dirPath, n.Children); err != nil {
				return err
			}
			return nil
		}

		filePath := filepath.Join(parentDir, slug+".md")
		return f.writeLegacyNodeMarkdown(filePath, n)

	default:
		return fmt.Errorf("legacy node %q has unknown kind %q", n.ID, n.Kind)
	}
}

func (n *legacyPageNode) sortedChildren() []*legacyPageNode {
	out := append([]*legacyPageNode(nil), n.Children...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (f *NodeStore) writeLegacyOrderFile(sectionDir string, children []*legacyPageNode) error {
	if len(children) == 0 {
		return nil
	}

	sorted := append([]*legacyPageNode(nil), children...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Position != sorted[j].Position {
			return sorted[i].Position < sorted[j].Position
		}
		return sorted[i].ID < sorted[j].ID
	})

	ids := make([]string, 0, len(sorted))
	for _, ch := range sorted {
		if ch == nil || strings.TrimSpace(ch.ID) == "" {
			continue
		}
		ids = append(ids, ch.ID)
	}

	data, err := json.MarshalIndent(sectionOrderFile{Children: ids}, "", "  ")
	if err != nil {
		return err
	}
	return shared.WriteFileAtomic(filepath.Join(sectionDir, "order.json"), data, 0o644)
}

func (f *NodeStore) writeLegacyNodeMarkdown(path string, n *legacyPageNode) error {
	body := "# " + strings.TrimSpace(n.Title) + "\n"
	fm := markdown.Frontmatter{
		LeafWikiID:    strings.TrimSpace(n.ID),
		LeafWikiTitle: strings.TrimSpace(n.Title),
	}
	if !n.Metadata.CreatedAt.IsZero() {
		fm.CreatedAt = n.Metadata.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !n.Metadata.UpdatedAt.IsZero() {
		fm.UpdatedAt = n.Metadata.UpdatedAt.UTC().Format(time.RFC3339)
	}
	fm.CreatorID = strings.TrimSpace(n.Metadata.CreatorID)
	fm.LastAuthorID = strings.TrimSpace(n.Metadata.LastAuthorID)

	mf := markdown.NewMarkdownFile(path, body, fm)
	if err := mf.WriteToFile(); err != nil {
		return fmt.Errorf("write migrated markdown %s: %w", path, err)
	}
	return nil
}

func (f *NodeStore) orderFilePathForSection(section *PageNode) (string, error) {
	if section == nil {
		return "", errors.New("section is nil")
	}
	if section.Kind != NodeKindSection {
		return "", fmt.Errorf("node %s is not a section", section.ID)
	}
	dir, err := f.dirPathForNode(section)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "order.json"), nil
}

func (f *NodeStore) readChildOrder(parentDir string) ([]string, error) {
	path := filepath.Join(parentDir, "order.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var file sectionOrderFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	return file.Children, nil
}
func (f *NodeStore) WriteChildOrder(parent *PageNode, orderedIDs []string) error {
	if parent == nil {
		return errors.New("parent is nil")
	}
	if parent.Kind != NodeKindSection {
		return fmt.Errorf("parent %s is not a section", parent.ID)
	}

	valid := make(map[string]bool, len(parent.Children))
	for _, child := range parent.Children {
		valid[child.ID] = true
	}

	seen := make(map[string]bool, len(orderedIDs))
	out := make([]string, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if !valid[id] {
			return fmt.Errorf("order contains non-child id %q", id)
		}
		if seen[id] {
			return fmt.Errorf("order contains duplicate id %q", id)
		}
		seen[id] = true
		out = append(out, id)
	}

	path, err := f.orderFilePathForSection(parent)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(sectionOrderFile{Children: out}, "", "  ")
	if err != nil {
		return err
	}
	return shared.WriteFileAtomic(path, data, 0o644)
}

func (f *NodeStore) AppendChildOrder(parent *PageNode, childID string) error {
	ids, err := f.normalizedOrderedChildren(parent)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == childID {
			return nil
		}
	}
	ids = append(ids, childID)
	return f.WriteChildOrder(parent, ids)
}

func (f *NodeStore) RemoveChildOrder(parent *PageNode, childID string) error {
	ids, err := f.normalizedOrderedChildren(parent)
	if err != nil {
		return err
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != childID {
			out = append(out, id)
		}
	}
	return f.WriteChildOrder(parent, out)
}

func (f *NodeStore) markDuplicateIDs(root *PageNode) {
	seen := map[string]*PageNode{}
	var walk func(*PageNode)
	walk = func(n *PageNode) {
		if n == nil {
			return
		}
		if n.ID != "" && n.ID != "root" {
			if prev, ok := seen[n.ID]; ok {
				n.RepairNeeded = true
				n.Issues = append(n.Issues, NodeIssue{
					Code:    NodeIssueDuplicateID,
					Message: fmt.Sprintf("duplicate leafwiki_id also used by %q", prev.CalculatePath()),
				})
				prev.RepairNeeded = true
				prev.Issues = append(prev.Issues, NodeIssue{
					Code:    NodeIssueDuplicateID,
					Message: fmt.Sprintf("duplicate leafwiki_id also used by %q", n.CalculatePath()),
				})
			} else {
				seen[n.ID] = n
			}
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(root)
}

func (f *NodeStore) currentOrderedChildren(parent *PageNode) ([]string, error) {
	if parent == nil {
		return nil, errors.New("parent is nil")
	}
	if parent.Kind != NodeKindSection {
		return nil, fmt.Errorf("parent %s is not a section", parent.ID)
	}
	dir, err := f.dirPathForNode(parent)
	if err != nil {
		return nil, err
	}
	return f.readChildOrder(dir)
}

func (f *NodeStore) sortChildrenForParent(parentDir string, children []*PageNode, parent *PageNode) []*PageNode {
	orderIDs, err := f.readChildOrder(parentDir)
	if err != nil {
		parent.RepairNeeded = true
		parent.Issues = append(parent.Issues, NodeIssue{
			Code:    NodeIssueInvalidOrder,
			Message: "order.json could not be parsed; falling back to deterministic order",
		})
		orderIDs = nil
	}

	byID := make(map[string]*PageNode, len(children))
	for _, child := range children {
		byID[child.ID] = child
	}

	var ordered []*PageNode
	seen := make(map[string]bool, len(children))
	unknownIDs := false

	for _, id := range orderIDs {
		if child, ok := byID[id]; ok {
			if !seen[id] {
				ordered = append(ordered, child)
				seen[id] = true
			}
		} else {
			unknownIDs = true
		}
	}

	if unknownIDs {
		parent.RepairNeeded = true
		parent.Issues = append(parent.Issues, NodeIssue{
			Code:    NodeIssueInvalidOrder,
			Message: "order.json references unknown child ids",
		})
	}

	var tail []*PageNode
	for _, child := range children {
		if !seen[child.ID] {
			tail = append(tail, child)
		}
	}

	sort.SliceStable(tail, func(i, j int) bool {
		si := strings.ToLower(tail[i].Slug)
		sj := strings.ToLower(tail[j].Slug)
		if si == sj {
			return tail[i].Slug < tail[j].Slug
		}
		return si < sj
	})

	return append(ordered, tail...)
}

func (f *NodeStore) normalizedOrderedChildren(parent *PageNode) ([]string, error) {
	raw, err := f.currentOrderedChildren(parent)
	if err != nil {
		return nil, err
	}

	valid := make(map[string]bool, len(parent.Children))
	for _, child := range parent.Children {
		valid[child.ID] = true
	}

	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		if !valid[id] || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}

// WriteNodeFrontmatter explicitly persists node metadata to the canonical content file.
// For sections, this may create index.md if missing.
// For pages, a missing file is treated as drift.
func (f *NodeStore) WriteNodeFrontmatter(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "WriteNodeFrontmatter", Reason: "an entry is required"}
	}
	if entry.ID == "root" {
		return nil
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return err
	}

	var mdFile *markdown.MarkdownFile

	if _, err := os.Stat(filePath); err == nil {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf("could not load markdown file %s: %w", filePath, err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if entry.Kind == NodeKindPage {
			return &DriftError{
				NodeID: entry.ID,
				Kind:   entry.Kind,
				Path:   filePath,
				Reason: "expected page file missing",
			}
		}
		mdFile = markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	} else {
		return fmt.Errorf("could not stat markdown file %s: %w", filePath, err)
	}

	mdFile.SetFrontmatterID(entry.ID)
	mdFile.SetFrontmatterTitle(entry.Title)
	mdFile.SetFrontmatterMetadata(
		formatRFC3339(entry.Metadata.CreatedAt),
		entry.Metadata.CreatorID,
		formatRFC3339(entry.Metadata.UpdatedAt),
		entry.Metadata.LastAuthorID,
	)

	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf("could not write markdown file %s: %w", filePath, err)
	}

	return nil
}

// syntheticNodeID is only a temporary projection fallback for nodes missing leafwiki_id.
// It must not be treated as stable identity across renames or moves.
func syntheticNodeID(path string) string {
	return "missing-id:" + strings.ReplaceAll(path, string(filepath.Separator), "/")
}

// CreatePage creates a new page file under the given parent entry
func (f *NodeStore) CreatePage(parentEntry *PageNode, newEntry *PageNode) error {
	if parentEntry == nil {
		return &InvalidOpError{Op: "CreatePage", Reason: "a parent entry is required"}
	}
	if newEntry == nil {
		return &InvalidOpError{Op: "CreatePage", Reason: "a new entry is required"}
	}
	if newEntry.ID == "root" {
		return &InvalidOpError{Op: "CreatePage", Reason: "cannot create root"}
	}

	// Pages can only be created under sections (Option A)
	if parentEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "CreatePage", Reason: "parent entry must be a section"}
	}
	if newEntry.Kind != NodeKindPage {
		return &InvalidOpError{Op: "CreatePage", Reason: "new entry must be a page"}
	}

	// Parent directory is determined by the tree path
	parentDir, err := f.dirPathForNode(parentEntry)
	if err != nil {
		return err
	}

	// Ensure the parent directory exists (idempotent)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("could not ensure parent directory exists: %w", err)
	}

	// Destination paths
	destBase := filepath.Join(parentDir, newEntry.Slug)
	destFile := destBase + ".md"
	destDir := destBase

	// Reject if either a file OR a directory with same slug exists
	if fileExists(destFile) || fileExists(destDir) {
		return &PageAlreadyExistsError{Path: destBase}
	}

	// Build and write file
	mf := markdown.NewMarkdownFile(destFile, "# "+newEntry.Title+"\n", markdown.Frontmatter{})
	mf.SetFrontmatterID(newEntry.ID)
	mf.SetFrontmatterTitle(newEntry.Title)
	mf.SetFrontmatterMetadata(
		formatRFC3339(newEntry.Metadata.CreatedAt),
		newEntry.Metadata.CreatorID,
		formatRFC3339(newEntry.Metadata.UpdatedAt),
		newEntry.Metadata.LastAuthorID,
	)
	if err := mf.WriteToFile(); err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}

	return nil
}

// CreateSection creates a new section (folder) under the given parent entry.
func (f *NodeStore) CreateSection(parentEntry *PageNode, newEntry *PageNode) error {
	if parentEntry == nil {
		return &InvalidOpError{Op: "CreateSection", Reason: "a parent entry is required"}
	}
	if newEntry == nil {
		return &InvalidOpError{Op: "CreateSection", Reason: "a new entry is required"}
	}
	if newEntry.ID == "root" {
		return &InvalidOpError{Op: "CreateSection", Reason: "cannot create root"}
	}
	if parentEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "CreateSection", Reason: "parent entry must be a section"}
	}
	if newEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "CreateSection", Reason: "new entry must be a section"}
	}

	parentDir, err := f.dirPathForNode(parentEntry)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("could not ensure parent directory exists: %w", err)
	}

	destBase := filepath.Join(parentDir, newEntry.Slug)
	destFile := destBase + ".md"
	destDir := destBase

	if fileExists(destFile) || fileExists(destDir) {
		return &PageAlreadyExistsError{Path: destBase}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("could not create section folder: %w", err)
	}

	indexPath := filepath.Join(destDir, "index.md")
	mf := markdown.NewMarkdownFile(indexPath, "# "+newEntry.Title+"\n", markdown.Frontmatter{})
	mf.SetFrontmatterID(newEntry.ID)
	mf.SetFrontmatterTitle(newEntry.Title)
	mf.SetFrontmatterMetadata(
		formatRFC3339(newEntry.Metadata.CreatedAt),
		newEntry.Metadata.CreatorID,
		formatRFC3339(newEntry.Metadata.UpdatedAt),
		newEntry.Metadata.LastAuthorID,
	)

	if err := mf.WriteToFile(); err != nil {
		return fmt.Errorf("could not create section index.md: %w", err)
	}

	return nil
}

// UpsertContent updates the content of a page file on disk
// It creates the file if it does not exist also for sections (index.md)
func (f *NodeStore) UpsertContent(entry *PageNode, content string) error {

	if entry == nil {
		return &InvalidOpError{Op: "UpsertContent", Reason: "an entry is required"}
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return err
	}

	var mf *markdown.MarkdownFile
	if _, err := os.Stat(filePath); err == nil {
		mf, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		mf = markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	} else {
		return err
	}

	mf.SetFrontmatterID(entry.ID)
	mf.SetFrontmatterTitle(entry.Title)
	mf.SetFrontmatterMetadata(
		formatRFC3339(entry.Metadata.CreatedAt),
		entry.Metadata.CreatorID,
		formatRFC3339(entry.Metadata.UpdatedAt),
		entry.Metadata.LastAuthorID,
	)
	mf.SetContent(content)

	return mf.WriteToFile()
}

// MoveNode moves a page to a other node
func (f *NodeStore) MoveNode(entry *PageNode, parentEntry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "MoveNode", Reason: "an entry is required"}
	}
	if parentEntry == nil {
		return &InvalidOpError{Op: "MoveNode", Reason: "a parent entry is required"}
	}
	if entry.ID == "root" {
		return &InvalidOpError{Op: "MoveNode", Reason: "cannot move root"}
	}

	// Option A: children only under sections (defensive guard)
	if parentEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "MoveNode", Reason: fmt.Sprintf("parent entry must be a section, got %q", parentEntry.Kind)}
	}

	// Parent directory path from tree
	parentDir, err := f.dirPathForNode(parentEntry)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("could not ensure parent directory exists: %w", err)
	}

	// Current base path from tree (still at old location; TreeService updates Parent after success)
	oldBase, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}
	oldFile := oldBase + ".md"
	oldDir := oldBase

	// Destination base path (same slug, under new parent)
	destBase := filepath.Join(parentDir, entry.Slug)
	destFile := destBase + ".md"
	destDir := destBase

	// Collision checks: refuse if destination already exists as file OR dir
	if fileExists(destFile) || fileExists(destDir) {
		return &PageAlreadyExistsError{Path: destBase}
	}

	// STRICT: follow tree.Kind exactly (no disk fallbacks)
	switch entry.Kind {
	case NodeKindSection:
		// src must be a directory
		info, err := os.Stat(oldDir)
		if err != nil {
			if os.IsNotExist(err) {
				f.log.Warn("move drift: expected folder missing", "nodeID", entry.ID, "expectedDir", oldDir)
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldDir, Reason: "expected folder missing"}
			}
			return fmt.Errorf("stat source dir: %w", err)
		}
		if !info.IsDir() {
			f.log.Warn("move drift: expected folder but found file", "nodeID", entry.ID, "expectedDir", oldDir)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldDir, Reason: "expected folder but found file"}
		}

		if err := os.Rename(oldDir, destDir); err != nil {
			return fmt.Errorf("could not move folder: %w", err)
		}

	case NodeKindPage:
		// src must be a file
		info, err := os.Stat(oldFile)
		if err != nil {
			if os.IsNotExist(err) {
				f.log.Warn("move drift: expected file missing", "nodeID", entry.ID, "expectedFile", oldFile)
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldFile, Reason: "expected file missing"}
			}
			return fmt.Errorf("stat source file: %w", err)
		}
		if info.IsDir() {
			f.log.Warn("move drift: expected file but found folder", "nodeID", entry.ID, "expectedFile", oldFile)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldFile, Reason: "expected file but found folder"}
		}

		if err := os.Rename(oldFile, destFile); err != nil {
			return fmt.Errorf("could not move file: %w", err)
		}

	default:
		return &InvalidOpError{Op: "MoveNode", Reason: fmt.Sprintf("unknown node kind: %q", entry.Kind)}
	}

	return nil
}

// DeletePage deletes a page file from disk
func (f *NodeStore) DeletePage(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "DeletePage", Reason: "an entry is required"}
	}
	if entry.ID == "root" {
		return &InvalidOpError{Op: "DeletePage", Reason: "cannot delete root"}
	}
	if entry.Kind != NodeKindPage && entry.Kind != "" {
		return &InvalidOpError{Op: "DeletePage", Reason: "entry must be a page"}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}
	file := base + ".md"

	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			f.log.Warn("delete drift: expected page file missing", "nodeID", entry.ID, "expectedFile", file)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: file, Reason: "expected file missing"}
		}
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		f.log.Warn("delete drift: expected file but found folder", "nodeID", entry.ID, "expectedFile", file)
		return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: file, Reason: "expected file but found folder"}
	}

	if err := os.Remove(file); err != nil {
		return fmt.Errorf("could not delete file: %w", err)
	}

	return nil
}

// DeleteSection deletes a section folder from disk
func (f *NodeStore) DeleteSection(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "DeleteSection", Reason: "an entry is required"}
	}
	if entry.ID == "root" {
		return &InvalidOpError{Op: "DeleteSection", Reason: "cannot delete root"}
	}
	if entry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "DeleteSection", Reason: "entry must be a section"}
	}

	dir, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			f.log.Warn("delete drift: expected section folder missing", "nodeID", entry.ID, "expectedDir", dir)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: dir, Reason: "expected folder missing"}
		}
		return fmt.Errorf("stat dir: %w", err)
	}
	if !info.IsDir() {
		f.log.Warn("delete drift: expected folder but found file", "nodeID", entry.ID, "expectedDir", dir)
		return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: dir, Reason: "expected folder but found file"}
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("could not delete folder: %w", err)
	}

	return nil
}

// RenameNode renames a node's slug on disk
func (f *NodeStore) RenameNode(entry *PageNode, newSlug string) error {
	if entry == nil {
		return &InvalidOpError{Op: "RenameNode", Reason: "an entry is required"}
	}
	if strings.TrimSpace(newSlug) == "" {
		return &InvalidOpError{Op: "RenameNode", Reason: "new slug must not be empty"}
	}
	if entry.Slug == newSlug {
		return nil
	}
	if entry.ID == "root" {
		return &InvalidOpError{Op: "RenameNode", Reason: "cannot rename root"}
	}

	// old base path computed from current entry (still has old slug)
	oldBase, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}

	// new base path: same parent dir, last segment replaced
	newBase := filepath.Join(filepath.Dir(oldBase), newSlug)

	// destination collision checks
	if fileExists(newBase+".md") || fileExists(newBase) {
		return &PageAlreadyExistsError{Path: newBase}
	}
	// perform rename based on kind
	switch entry.Kind {
	case NodeKindSection:
		srcDir := oldBase
		dstDir := newBase

		// strict: source dir must exist and be dir
		info, err := os.Stat(srcDir)
		if err != nil {
			if os.IsNotExist(err) {
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcDir, Reason: "expected folder missing"}
			}
			return fmt.Errorf("stat source dir: %w", err)
		}
		if !info.IsDir() {
			// drift: tree says section but disk is not a folder
			f.log.Warn("drift: tree says section but disk is not a folder", "srcDir", srcDir)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcDir, Reason: "expected folder but found file"}
		}

		if err := os.Rename(srcDir, dstDir); err != nil {
			return fmt.Errorf("could not rename folder: %w", err)
		}
		return nil
	case NodeKindPage:
		srcFile := oldBase + ".md"
		dstFile := newBase + ".md"

		// strict: source file must exist
		info, err := os.Stat(srcFile)
		if err != nil {
			if os.IsNotExist(err) {
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcFile, Reason: "expected file missing"}
			}
			return fmt.Errorf("stat source file: %w", err)
		}
		if info.IsDir() {
			// drift: tree says page but disk is a dir
			f.log.Warn("drift: tree says page but disk is a dir", "srcFile", srcFile)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcFile, Reason: "expected file but found folder"}
		}

		if err := os.Rename(srcFile, dstFile); err != nil {
			return fmt.Errorf("could not rename file: %w", err)
		}
		return nil

	default:
		return &InvalidOpError{Op: "RenameNode", Reason: fmt.Sprintf("unknown node kind: %q", entry.Kind)}
	}
}

// ReadPageRaw returns the raw content of a page including frontmatter
func (f *NodeStore) ReadPageRaw(entry *PageNode) (string, error) {
	filePath, err := f.contentPathForNodeRead(entry)
	if err != nil {
		return "", err
	}

	// Sections may legitimately have no content (missing index.md)
	if entry.Kind == NodeKindSection {
		if !fileExists(filePath) {
			return "", nil
		}
	} else {
		// Pages must have a content file
		if !fileExists(filePath) {
			return "", &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: filePath, Reason: "expected page file missing"}
		}
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ReadPageContent returns the content of a page
func (f *NodeStore) ReadPageContent(entry *PageNode) (string, error) {
	raw, err := f.ReadPageRaw(entry)
	if err != nil {
		return "", err
	}
	_, content, _, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		return string(raw), err
	}
	return content, nil
}

func (f *NodeStore) dirPathForNode(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "dirPathForNode", Reason: "an entry is required"}
	}
	return filepath.Join(f.storageDir, GeneratePathFromPageNode(entry)), nil
}

// contentPathForNodeRead returns the expected content file path for a node
// based purely on the tree Kind (NO side effects, NO mkdir):
// - page   => <base>.md
// - section => <base>/index.md
func (f *NodeStore) contentPathForNodeRead(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "contentPathForNodeRead", Reason: "an entry is required"}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return "", err
	}
	switch entry.Kind {
	case NodeKindSection:
		return filepath.Join(base, "index.md"), nil
	case NodeKindPage:
		return base + ".md", nil
	default:
		return "", &InvalidOpError{Op: "contentPathForNodeRead", Reason: fmt.Sprintf("unknown node kind: %q", entry.Kind)}
	}
}

// contentPathForNodeWrite returns the expected content file path for a node
// based purely on the tree Kind (MAY create dirs for sections):
// - page   => <base>.md
// - section => <base>/index.md (ensures directory exists)
func (f *NodeStore) contentPathForNodeWrite(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "contentPathForNodeWrite", Reason: "an entry is required"}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return "", err
	}
	switch entry.Kind {
	case NodeKindSection:
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", fmt.Errorf("could not ensure folder: %w", err)
		}
		return filepath.Join(base, "index.md"), nil

	case NodeKindPage:
		return base + ".md", nil

	default:
		return "", &InvalidOpError{Op: "contentPathForNodeWrite", Reason: fmt.Sprintf("unknown node kind: %q", entry.Kind)}
	}
}

// ConvertNode converts on-disk representation between page-file and section-folder.
// This is transitional infrastructure, not a canonical "kind metadata update".
// Kind is derived from the resulting filesystem shape.
func (f *NodeStore) ConvertNode(entry *PageNode, target NodeKind) error {
	if entry == nil {
		return &InvalidOpError{Op: "ConvertNode", Reason: "an entry is required"}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}
	filePath := base + ".md"
	folderPath := base
	indexPath := filepath.Join(folderPath, "index.md")

	switch target {
	case NodeKindSection:
		// page -> folder
		if _, err := os.Stat(filePath); err == nil {
			if err := os.MkdirAll(folderPath, 0o755); err != nil {
				return fmt.Errorf("could not create folder: %w", err)
			}
			// keep content: <slug>.md -> <slug>/index.md
			if err := os.Rename(filePath, indexPath); err != nil {
				return fmt.Errorf("could not move page into folder: %w", err)
			}
			return nil
		}
		// already folder (or missing) -> ensure dir exists
		if err := os.MkdirAll(folderPath, 0o755); err != nil {
			return fmt.Errorf("could not ensure folder exists: %w", err)
		}
		return nil

	case NodeKindPage:
		// folder -> page (strict, safe order)
		info, err := os.Stat(folderPath)
		if err != nil {
			if os.IsNotExist(err) {
				// nothing to do if folder doesn't exist
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return &DriftError{NodeID: entry.ID, Kind: NodeKindSection, Path: folderPath, Reason: "expected folder but found file"}
		}

		entries, err := os.ReadDir(folderPath)
		if err != nil {
			return err
		}

		// allow only:
		// - empty folder
		// - folder with only index.md
		allowed := true
		for _, e := range entries {
			name := e.Name()
			if name == "index.md" {
				continue
			}
			allowed = false
			break
		}
		if !allowed {
			return &ConvertNotAllowedError{From: NodeKindSection, To: NodeKindPage, Reason: "folder not empty"}
		}

		// now do the move/create
		if fileExists(indexPath) {
			if err := os.Rename(indexPath, filePath); err != nil {
				return fmt.Errorf("could not move index to page: %w", err)
			}
		} else {
			fm := markdown.Frontmatter{LeafWikiID: entry.ID, LeafWikiTitle: entry.Title}
			md, err := markdown.BuildMarkdownWithFrontmatter(fm, "")
			if err != nil {
				return err
			}
			if err := shared.WriteFileAtomic(filePath, []byte(md), 0o644); err != nil {
				return fmt.Errorf("could not write page file: %w", err)
			}
		}

		// remove folder (must be empty now)
		if err := os.Remove(folderPath); err != nil {
			return err
		}
		return nil

	default:
		return &InvalidOpError{Op: "ConvertNode", Reason: fmt.Sprintf("unknown target kind: %q", target)}
	}
}
