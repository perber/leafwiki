package tree

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/shared"
)

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func ensureUniqueReconstructedID(seenIDs map[string]string, id string, path string) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return fmt.Errorf("reconstruct tree from fs: empty leafwiki_id at %s", path)
	}
	if existingPath, exists := seenIDs[trimmedID]; exists {
		return fmt.Errorf("duplicate leafwiki_id %q in %s and %s", trimmedID, existingPath, path)
	}
	seenIDs[trimmedID] = path
	return nil
}

func ensureUniqueReconstructedSlug(seenSlugs map[string]string, slug string, path string) error {
	key := strings.ToLower(strings.TrimSpace(slug))
	if key == "" {
		return fmt.Errorf("reconstruct tree from fs: empty slug at %s", path)
	}
	if existingPath, exists := seenSlugs[key]; exists {
		return fmt.Errorf("duplicate slug %q (case-insensitive) in %s and %s", slug, existingPath, path)
	}
	seenSlugs[key] = path
	return nil
}

type ResolvedNode struct {
	Kind       NodeKind
	DirPath    string
	FilePath   string
	HasContent bool
}

type NodeStore struct {
	storageDir string
	log        *slog.Logger
	slugger    *SlugService
}

const reconstructSystemUserID = "system"
const orderFilename = ".order.json"

type childOrderFile struct {
	OrderedIDs []string `json:"ordered_ids"`
}

func NewNodeStore(storageDir string) *NodeStore {
	return &NodeStore{
		storageDir: storageDir,
		log:        slog.Default().With("component", "NodeStore"),
		slugger:    NewSlugService(),
	}
}

// writeIDToMarkdownFile writes a leafwiki_id to a markdown file's frontmatter and logs errors if the write fails
func (f *NodeStore) writeIDToMarkdownFile(mdFile *markdown.MarkdownFile, id string) {
	var originalModTime time.Time
	if info, err := os.Stat(mdFile.GetPath()); err == nil {
		originalModTime = info.ModTime()
	}

	mdFile.SetLeafWikiFrontmatter(id, mdFile.GetFrontmatter().LeafWikiTitle)
	if err := mdFile.WriteToFile(); err != nil {
		f.log.Error("could not write leafwiki_id back to file", "path", mdFile.GetPath(), "error", err)
		return
	}

	if !originalModTime.IsZero() {
		if err := os.Chtimes(mdFile.GetPath(), originalModTime, originalModTime); err != nil {
			f.log.Warn("could not restore file mtime after writing leafwiki_id", "path", mdFile.GetPath(), "error", err)
		}
	}
}

// writeReconstructedFrontmatter writes the full managed frontmatter (ID, title, timestamps, authors)
// back to disk while preserving the file's modification time. Called during reconstruct for files
// that are missing any managed metadata field.
func (f *NodeStore) writeReconstructedFrontmatter(mdFile *markdown.MarkdownFile, entry *PageNode) {
	var originalModTime time.Time
	if info, err := os.Stat(mdFile.GetPath()); err == nil {
		originalModTime = info.ModTime()
	}

	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		f.log.Error("could not write metadata back to file during reconstruct", "path", mdFile.GetPath(), "error", err)
		return
	}

	if !originalModTime.IsZero() {
		if err := os.Chtimes(mdFile.GetPath(), originalModTime, originalModTime); err != nil {
			f.log.Warn("could not restore file mtime after writing metadata", "path", mdFile.GetPath(), "error", err)
		}
	}
}

func formatMetadataTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func (f *NodeStore) syncManagedFrontmatter(mdFile *markdown.MarkdownFile, entry *PageNode) {
	mdFile.SetLeafWikiFrontmatter(strings.TrimSpace(entry.ID), strings.TrimSpace(entry.Title))
	mdFile.SetLeafWikiMetadata(
		formatMetadataTime(entry.Metadata.CreatedAt),
		formatMetadataTime(entry.Metadata.UpdatedAt),
		strings.TrimSpace(entry.Metadata.CreatorID),
		strings.TrimSpace(entry.Metadata.LastAuthorID),
	)
}

func (f *NodeStore) ensureSectionIndex(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "ensureSectionIndex", Reason: "an entry is required"}
	}
	if entry.Kind != NodeKindSection {
		return "", &InvalidOpError{Op: "ensureSectionIndex", Reason: "entry must be a section"}
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return "", err
	}

	mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	if fileExists(filePath) {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return "", fmt.Errorf("could not load markdown file: %w", err)
		}
	}

	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return "", fmt.Errorf("could not write markdown file: %w", err)
	}

	return filePath, nil
}

func fallbackMetadataString(value string) string {
	if strings.TrimSpace(value) == "" {
		return reconstructSystemUserID
	}
	return strings.TrimSpace(value)
}

func (f *NodeStore) metadataFallbackTime(filePath string, fallback time.Time) time.Time {
	info, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			f.log.Warn("could not stat path for reconstruct metadata fallback, using runtime fallback", "path", filePath, "fallback", fallback.UTC().Format(time.RFC3339), "error", err)
		}
		return fallback.UTC()
	}
	return info.ModTime().UTC()
}

func (f *NodeStore) parseMetadataTime(value string, fallback time.Time, field string, filePath string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback.UTC()
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, trimmed)
	}
	if err != nil {
		f.log.Warn("invalid frontmatter metadata timestamp, using fallback", "path", filePath, "field", field, "value", trimmed, "fallback", fallback.UTC().Format(time.RFC3339), "error", err)
		return fallback.UTC()
	}
	return parsed.UTC()
}

func (f *NodeStore) metadataFromFrontmatter(fm markdown.Frontmatter, fallbackNow time.Time, filePath string) PageMetadata {
	fallbackTime := f.metadataFallbackTime(filePath, fallbackNow)
	return PageMetadata{
		CreatedAt:    f.parseMetadataTime(fm.LeafWikiCreatedAt, fallbackTime, "leafwiki_created_at", filePath),
		UpdatedAt:    f.parseMetadataTime(fm.LeafWikiUpdatedAt, fallbackTime, "leafwiki_updated_at", filePath),
		CreatorID:    fallbackMetadataString(fm.LeafWikiCreatorID),
		LastAuthorID: fallbackMetadataString(fm.LeafWikiLastAuthorID),
	}
}

func (f *NodeStore) LoadTree(filename string) (*PageNode, error) {
	fullPath := filepath.Join(f.storageDir, filename)

	// check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return &PageNode{
			ID:       "root",
			Slug:     "root",
			Title:    "root",
			Parent:   nil,
			Position: 0,
			Children: []*PageNode{},
			Kind:     NodeKindSection,
		}, nil
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open tree file %s: %w", fullPath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			f.log.Error("could not close tree file", "file", fullPath, "error", err)
		}
	}()
	data, err := io.ReadAll(file)

	if err != nil {
		return nil, fmt.Errorf("read tree file %s: %w", fullPath, err)
	}

	tree := &PageNode{}
	if err := json.Unmarshal(data, tree); err != nil {
		return nil, fmt.Errorf("unmarshal tree data %s: %w", fullPath, err)
	}

	if tree.ID == "root" && tree.Kind == "" {
		tree.Kind = NodeKindSection
	}

	// assigns parent to children
	f.assignParentToChildren(tree)

	return tree, nil
}

func (f *NodeStore) ReconstructTreeFromFS() (*PageNode, error) {
	reconstructNow := time.Now().UTC()
	rootDir := filepath.Join(f.storageDir, "root")
	root := &PageNode{
		ID:       "root",
		Slug:     "root",
		Title:    "root",
		Parent:   nil,
		Position: 0,
		Children: []*PageNode{},
		Kind:     NodeKindSection,
		Metadata: f.metadataFromFrontmatter(markdown.Frontmatter{}, reconstructNow, rootDir),
	}
	seenIDs := map[string]string{"root": rootDir}

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

	if err := f.reconstructTreeRecursive(rootDir, root, reconstructNow, seenIDs); err != nil {
		return nil, fmt.Errorf("reconstruct tree from fs: %w", err)
	}

	return root, nil
}
func (f *NodeStore) reconstructTreeRecursive(currentPath string, parent *PageNode, reconstructNow time.Time, seenIDs map[string]string) error {
	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", currentPath, err)
	}
	seenSlugs := map[string]string{}

	// stable, deterministic ordering (case-insensitive, with case-sensitive tie-breaker)
	sort.SliceStable(entries, func(i, j int) bool {
		li := strings.ToLower(entries[i].Name())
		lj := strings.ToLower(entries[j].Name())
		if li == lj {
			return entries[i].Name() < entries[j].Name()
		}
		return li < lj
	})

	for _, entry := range entries {
		name := entry.Name()

		// optional: skip hidden stuff
		if strings.HasPrefix(name, ".") {
			continue
		}

		// defaults
		title := name
		id, err := shared.GenerateUniqueID()
		metadata := f.metadataFromFrontmatter(markdown.Frontmatter{}, reconstructNow, filepath.Join(currentPath, name))
		if err != nil {
			return fmt.Errorf("generate unique ID: %w", err)
		}

		if entry.IsDir() {
			if err := f.slugger.IsValidSlug(name); err != nil {
				f.log.Error("skipping directory with invalid slug", "directory", name, "error", err)
				continue
			}

			indexPath := filepath.Join(currentPath, name, "index.md")
			var sectionMdFile *markdown.MarkdownFile
			needsWriteback := false
			if fileExists(indexPath) {
				mdFile, err := markdown.LoadMarkdownFile(indexPath)
				if err != nil {
					f.log.Error("could not load index.md", "path", indexPath, "error", err)
					// fall back to default title and generated ID, but still add the section and recurse
				} else {
					fm := mdFile.GetFrontmatter()
					metadata = f.metadataFromFrontmatter(fm, reconstructNow, indexPath)
					title, err = mdFile.GetTitle()
					if err != nil {
						f.log.Error("could not extract title from index.md", "path", indexPath, "error", err)
						// keep default title; still add the section and recurse
					}
					if fm.LeafWikiID != "" {
						id = fm.LeafWikiID
					}
					if fm.LeafWikiID == "" || fm.LeafWikiUpdatedAt == "" || fm.LeafWikiCreatedAt == "" {
						sectionMdFile = mdFile
						needsWriteback = true
					}
				}
			}

			child := &PageNode{
				ID:       id,
				Slug:     name,
				Title:    title,
				Parent:   parent,
				Position: len(parent.Children),
				Children: []*PageNode{},
				Kind:     NodeKindSection,
				Metadata: metadata,
			}
			if err := ensureUniqueReconstructedSlug(seenSlugs, child.Slug, filepath.Join(currentPath, name)); err != nil {
				return err
			}
			if err := ensureUniqueReconstructedID(seenIDs, child.ID, indexPath); err != nil {
				return err
			}
			parent.Children = append(parent.Children, child)

			if needsWriteback {
				f.writeReconstructedFrontmatter(sectionMdFile, child)
			}

			if !fileExists(indexPath) {
				if _, err := f.ensureSectionIndex(child); err != nil {
					return fmt.Errorf("materialize missing section index for %s: %w", indexPath, err)
				}
			}

			if err := f.reconstructTreeRecursive(filepath.Join(currentPath, name), child, reconstructNow, seenIDs); err != nil {
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
		// skip index.md (handled by section case)
		if strings.EqualFold(baseFilename, "index") {
			continue
		}
		if err := f.slugger.IsValidSlug(baseFilename); err != nil {
			f.log.Error("skipping file with invalid slug", "file", name, "error", err)
			continue
		}

		filePath := filepath.Join(currentPath, name)

		mdFile, err := markdown.LoadMarkdownFile(filePath)
		if err != nil {
			f.log.Error("could not load markdown file", "path", filePath, "error", err)
			continue
		}
		fm := mdFile.GetFrontmatter()
		metadata = f.metadataFromFrontmatter(fm, reconstructNow, filePath)
		title, err = mdFile.GetTitle()
		if err != nil {
			f.log.Error("could not extract title from file", "path", filePath, "error", err)
			continue
		}
		if fm.LeafWikiID != "" {
			id = fm.LeafWikiID
		}
		needsWriteback := fm.LeafWikiID == "" || fm.LeafWikiUpdatedAt == "" || fm.LeafWikiCreatedAt == ""

		child := &PageNode{
			ID:       id,
			Slug:     baseFilename,
			Title:    title,
			Parent:   parent,
			Position: len(parent.Children),
			Children: nil,
			Kind:     NodeKindPage,
			Metadata: metadata,
		}
		if err := ensureUniqueReconstructedSlug(seenSlugs, child.Slug, filePath); err != nil {
			return err
		}
		if err := ensureUniqueReconstructedID(seenIDs, child.ID, filePath); err != nil {
			return err
		}
		if needsWriteback {
			f.writeReconstructedFrontmatter(mdFile, child)
		}
		parent.Children = append(parent.Children, child)
	}

	f.applyChildOrder(parent, currentPath)

	return nil
}

func (f *NodeStore) applyChildOrder(parent *PageNode, dirPath string) {
	if parent == nil || len(parent.Children) < 2 {
		return
	}

	order, err := f.readChildOrder(dirPath)
	if err != nil {
		f.log.Warn("could not read child order file, keeping default order", "path", filepath.Join(dirPath, orderFilename), "error", err)
		return
	}
	if len(order.OrderedIDs) == 0 {
		return
	}

	positions := make(map[string]int, len(order.OrderedIDs))
	for i, id := range order.OrderedIDs {
		if _, exists := positions[id]; exists {
			continue
		}
		positions[id] = i
	}

	sort.SliceStable(parent.Children, func(i, j int) bool {
		pi, okI := positions[parent.Children[i].ID]
		pj, okJ := positions[parent.Children[j].ID]
		switch {
		case okI && okJ:
			return pi < pj
		case okI:
			return true
		case okJ:
			return false
		default:
			return parent.Children[i].Position < parent.Children[j].Position
		}
	})

	for i, child := range parent.Children {
		child.Position = i
	}
}

func (f *NodeStore) readChildOrder(dirPath string) (*childOrderFile, error) {
	raw, err := os.ReadFile(filepath.Join(dirPath, orderFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return &childOrderFile{}, nil
		}
		return nil, err
	}

	var order childOrderFile
	if err := json.Unmarshal(raw, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (f *NodeStore) SaveChildOrder(parent *PageNode) error {
	if parent == nil {
		return &InvalidOpError{Op: "SaveChildOrder", Reason: "a parent entry is required"}
	}
	if parent.ID != "root" && parent.Kind != NodeKindSection {
		return &InvalidOpError{Op: "SaveChildOrder", Reason: "parent entry must be root or a section"}
	}

	dirPath, err := f.dirPathForNode(parent)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("could not ensure parent directory exists: %w", err)
	}

	orderedIDs := make([]string, 0, len(parent.Children))
	for _, child := range parent.Children {
		if child == nil {
			continue
		}
		orderedIDs = append(orderedIDs, child.ID)
	}

	data, err := json.MarshalIndent(childOrderFile{OrderedIDs: orderedIDs}, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal child order: %w", err)
	}
	data = append(data, byte('\n'))

	if err := shared.WriteFileAtomic(filepath.Join(dirPath, orderFilename), data, 0o644); err != nil {
		return fmt.Errorf("could not atomically write child order file: %w", err)
	}

	return nil
}

func (f *NodeStore) assignParentToChildren(parent *PageNode) {
	for _, child := range parent.Children {
		child.Parent = parent
		f.assignParentToChildren(child)
	}
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

	mdFile := markdown.NewMarkdownFile(destFile, "# "+newEntry.Title+"\n", markdown.Frontmatter{})
	f.syncManagedFrontmatter(mdFile, newEntry)
	if err := mdFile.WriteToFile(); err != nil {
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

	// Sections can only be created under sections (Option A)
	if parentEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "CreateSection", Reason: "parent entry must be a section"}
	}
	if newEntry.Kind != NodeKindSection {
		return &InvalidOpError{Op: "CreateSection", Reason: "new entry must be a section"}
	}

	// Parent directory from tree path
	parentDir, err := f.dirPathForNode(parentEntry)
	if err != nil {
		return err
	}

	// Ensure parent directory exists (idempotent)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("could not ensure parent directory exists: %w", err)
	}

	// Destination base paths
	destBase := filepath.Join(parentDir, newEntry.Slug)
	destFile := destBase + ".md"
	destDir := destBase

	// Reject if either a file OR a directory with same slug exists
	if fileExists(destFile) || fileExists(destDir) {
		return &PageAlreadyExistsError{Path: destBase}
	}

	// Create the folder for the section and materialize its metadata container.
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("could not create section folder: %w", err)
	}

	if _, err := f.ensureSectionIndex(newEntry); err != nil {
		return err
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

	mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	if fileExists(filePath) {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf("could not load markdown file: %w", err)
		}
	}

	if err := mdFile.SetRawContentPreservingManagedFrontmatter(content); err != nil {
		return fmt.Errorf("could not parse markdown content: %w", err)
	}
	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf("could not write markdown file: %w", err)
	}

	return nil
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

	filePath, err := f.contentPathForNodeRead(entry)
	if err != nil {
		return "", err
	}

	mdFile, err := markdown.NewMarkdownFileFromRaw(filePath, raw)
	if err != nil {
		return raw, err
	}

	return mdFile.GetContent(), nil
}

// SyncFrontmatterIfExists updates the frontmatter of a page file on disk if it exists
func (f *NodeStore) SyncFrontmatterIfExists(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "SyncFrontmatterIfExists", Reason: "an entry is required"}
	}

	// keine side effects: write-path NICHT verwenden (würde mkdir + bei Section implizit index.md Pfad liefern)
	// aber read-path reicht, weil wir nur syncen, wenn Datei existiert
	filePath, err := f.contentPathForNodeRead(entry)
	if err != nil {
		return err
	}

	// Datei existiert?
	if !fileExists(filePath) {
		// Page: muss existieren
		if entry.Kind == NodeKindPage || entry.Kind == "" {
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: filePath, Reason: "expected page file missing"}
		}
		// Section: kein index.md -> NICHT erzeugen
		return nil
	}

	mdFile, err := markdown.LoadMarkdownFile(filePath)
	if err != nil {
		return fmt.Errorf("load markdown file: %w", err)
	}

	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf("write markdown file: %w", err)
	}
	return nil
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

// resolveNode inspects the filesystem to determine if the given PageNode
// corresponds to a file or folder, returning a ResolvedNode with details.
// This function is only used for migration. Other parts of the system should rely on contentPathForNodeRead or contentPathForNodeWrite.
// If this function is used outside of migration, it may lead to inconsistencies between the tree and the actual filesystem state.
func (f *NodeStore) resolveNode(entry *PageNode) (*ResolvedNode, error) {
	basePath, err := f.dirPathForNode(entry)
	if err != nil {
		return nil, err
	}

	// 1) File?
	if _, err := os.Stat(basePath + ".md"); err == nil {
		f.log.Debug("resolved as file node", "filePath", basePath+".md")
		return &ResolvedNode{
			Kind:       NodeKindPage,
			FilePath:   basePath + ".md",
			HasContent: true,
		}, nil
	}

	// 2) Folder?
	if info, err := os.Stat(basePath); err == nil && info.IsDir() {
		index := filepath.Join(basePath, "index.md")
		if _, err := os.Stat(index); err == nil {
			f.log.Debug("resolved as section node with content", "dirPath", basePath, "filePath", index)
			return &ResolvedNode{
				Kind:       NodeKindSection,
				DirPath:    basePath,
				FilePath:   index,
				HasContent: true,
			}, nil
		}
		f.log.Debug("resolved as section node without content", "dirPath", basePath)
		return &ResolvedNode{
			Kind:       NodeKindSection,
			DirPath:    basePath,
			FilePath:   "", // no index.md present
			HasContent: false,
		}, nil
	}

	return nil, &NotFoundError{Resource: "node", Path: basePath, ID: entry.ID}
}

// ConvertNode converts the on-disk representation between page <-> folder.
// NOTE: TreeService must ensure folder->page is allowed (no children).
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
			entry.Kind = NodeKindSection
			if _, err := f.ensureSectionIndex(entry); err != nil {
				return err
			}
			return nil
		}
		// already folder (or missing) -> ensure dir exists and materialize index.md
		if err := os.MkdirAll(folderPath, 0o755); err != nil {
			return fmt.Errorf("could not ensure folder exists: %w", err)
		}
		entry.Kind = NodeKindSection
		if _, err := f.ensureSectionIndex(entry); err != nil {
			return err
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
		// - internal child-order metadata file (alone or alongside index.md)
		allowed := true
		for _, e := range entries {
			name := e.Name()
			if name == "index.md" || name == orderFilename {
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
			mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
			f.syncManagedFrontmatter(mdFile, entry)
			if err := mdFile.WriteToFile(); err != nil {
				return fmt.Errorf("could not write page file: %w", err)
			}
		}

		if err := os.Remove(filepath.Join(folderPath, orderFilename)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("could not remove child order file: %w", err)
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
