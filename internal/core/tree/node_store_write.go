package tree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/shared"
)

func (f *NodeStore) SaveChildOrder(parent *PageNode) error {
	if parent == nil {
		return &InvalidOpError{Op: "SaveChildOrder", Reason: errParentEntryRequired}
	}
	if parent.ID != "root" && parent.Kind != NodeKindSection {
		return &InvalidOpError{Op: "SaveChildOrder", Reason: "parent entry must be root or a section"}
	}

	dirPath, err := f.dirPathForNode(parent)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf(errEnsureParentFailed, err)
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

// CreatePage creates a new page file under the given parent entry
func (f *NodeStore) CreatePage(parentEntry *PageNode, newEntry *PageNode) error {
	if parentEntry == nil {
		return &InvalidOpError{Op: "CreatePage", Reason: errParentEntryRequired}
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
		return fmt.Errorf(errEnsureParentFailed, err)
	}

	// Destination paths
	destBase := filepath.Join(parentDir, newEntry.Slug)

	// Guard against ignored paths
	rel, _ := filepath.Rel(filepath.Join(f.storageDir, "root"), destBase)
	if f.isPathIgnored(rel, false) {
		return &InvalidOpError{Op: "CreatePage", Reason: "target path matches .leafwikiignore"}
	}

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

// CreateSection creates a new section (folder) under the given paren
func (f *NodeStore) CreateSection(parentEntry *PageNode, newEntry *PageNode) error {
	if parentEntry == nil {
		return &InvalidOpError{Op: "CreateSection", Reason: errParentEntryRequired}
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
		return fmt.Errorf(errEnsureParentFailed, err)
	}

	// Destination base paths
	destBase := filepath.Join(parentDir, newEntry.Slug)

	// Guard against ignored paths
	rel, _ := filepath.Rel(filepath.Join(f.storageDir, "root"), destBase)
	if f.isPathIgnored(rel, true) {
		return &InvalidOpError{Op: "CreateSection", Reason: "target path matches .leafwikiignore"}
	}

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

// UpsertContent updates the content of a page file on disk, treating the
// incoming content as plain body text. Any frontmatter-like blocks the caller
// passes are stored verbatim in the body and are never extracted into the
// system-managed frontmatter. Use this for all UI-originated writes.
// It creates the file if it does not exist also for sections (index.md).
func (f *NodeStore) UpsertContent(entry *PageNode, content string) error {
	if entry == nil {
		return &InvalidOpError{Op: "UpsertContent", Reason: errEntryRequired}
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return err
	}

	mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	if fileExists(filePath) {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf(errLoadMarkdownFailed, err)
		}
	}

	mdFile.SetContent(content)
	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf(errWriteMarkdownFailed, err)
	}

	return nil
}

// UpsertContentPreservingFrontmatter is the importer variant of UpsertContent.
// It parses any frontmatter block at the top of content and merges the extra
// fields into the system-managed frontmatter block written to disk.
func (f *NodeStore) UpsertContentPreservingFrontmatter(entry *PageNode, content string) error {
	if entry == nil {
		return &InvalidOpError{Op: "UpsertContentPreservingFrontmatter", Reason: errEntryRequired}
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return err
	}

	mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	if fileExists(filePath) {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf(errLoadMarkdownFailed, err)
		}
	}

	if err := mdFile.SetRawContentPreservingManagedFrontmatter(content); err != nil {
		return fmt.Errorf("could not parse markdown content: %w", err)
	}
	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf(errWriteMarkdownFailed, err)
	}

	return nil
}

// UpsertContentAndMetadata is the UI-edit path: it replaces the body and the
// editor-visible frontmatter (tags and string-typed properties) while keeping
// system-managed frontmatter (leafwiki_*) and non-string pass-through fields
// (booleans, numbers, non-tag lists) intact.
//
// Ownership rules for ExtraFields:
//   - String-valued keys are editor-owned: only keys present in properties
//     survive; keys absent from properties are removed.
//   - Non-string, non-map values (bool, int, float64, []interface{}) are
//     always preserved — the editor cannot represent them, so it cannot
//     intentionally delete them.
//   - Nested maps (map[string]interface{}) are not preserved; their string
//     leaves round-trip through the editor as dot-notation property keys.
//   - Tags: when tags is non-nil the existing tags are replaced; when tags is
//     nil the existing tags in the file are left unchanged.
func (f *NodeStore) UpsertContentAndMetadata(
	entry *PageNode,
	body string,
	tags []string,
	properties map[string]string,
) error {
	if entry == nil {
		return &InvalidOpError{Op: "UpsertContentAndMetadata", Reason: errEntryRequired}
	}

	filePath, err := f.contentPathForNodeWrite(entry)
	if err != nil {
		return err
	}

	mdFile := markdown.NewMarkdownFile(filePath, "", markdown.Frontmatter{})
	if fileExists(filePath) {
		mdFile, err = markdown.LoadMarkdownFile(filePath)
		if err != nil {
			return fmt.Errorf(errLoadMarkdownFailed, err)
		}
	}

	mdFile.SetContent(body)

	existing := mdFile.GetFrontmatter().ExtraFields
	extra := make(map[string]interface{}, len(properties)+len(existing)+1)

	// Preserve non-string, non-map ExtraFields (bool, int, float64, non-tag
	// lists). The editor cannot represent these types and never sends them back,
	// so they must survive an edit unchanged.
	for k, v := range existing {
		if k == "tags" {
			continue // handled separately below
		}
		switch v.(type) {
		case string, map[string]interface{}:
			// string: editor-owned, replaced by incoming properties
			// map: nested YAML — string leaves round-trip via dot-notation keys
		default:
			extra[k] = v
		}
	}

	// String properties from the editor replace every editor-owned string key.
	for k, v := range properties {
		extra[k] = v
	}

	// Tags: replace when the caller sends an explicit list; preserve otherwise.
	if tags != nil {
		tagList := make([]interface{}, len(tags))
		for i, t := range tags {
			tagList[i] = t
		}
		extra["tags"] = tagList
	} else if existingTags, ok := existing["tags"]; ok {
		extra["tags"] = existingTags
	}

	mdFile.SetExtraFields(extra)

	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return fmt.Errorf(errWriteMarkdownFailed, err)
	}

	return nil
}

// MoveNode moves a page to a other node
func (f *NodeStore) MoveNode(entry *PageNode, parentEntry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "MoveNode", Reason: errEntryRequired}
	}
	if parentEntry == nil {
		return &InvalidOpError{Op: "MoveNode", Reason: errParentEntryRequired}
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
		return fmt.Errorf(errEnsureParentFailed, err)
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

	// Guard against ignored paths
	rel, _ := filepath.Rel(filepath.Join(f.storageDir, "root"), destBase)
	if f.isPathIgnored(rel, entry.Kind == NodeKindSection) {
		return &InvalidOpError{Op: "MoveNode", Reason: "target path matches .leafwikiignore"}
	}

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
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldDir, Reason: errExpectedFolderMissing}
			}
			return fmt.Errorf("stat source dir: %w", err)
		}
		if !info.IsDir() {
			f.log.Warn("move drift: expected folder but found file", "nodeID", entry.ID, "expectedDir", oldDir)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldDir, Reason: errExpectedFolderFoundFile}
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
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldFile, Reason: errExpectedFileMissing}
			}
			return fmt.Errorf("stat source file: %w", err)
		}
		if info.IsDir() {
			f.log.Warn("move drift: expected file but found folder", "nodeID", entry.ID, "expectedFile", oldFile)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: oldFile, Reason: errExpectedFileFoundFolder}
		}

		if err := os.Rename(oldFile, destFile); err != nil {
			return fmt.Errorf("could not move file: %w", err)
		}

	default:
		return &InvalidOpError{Op: "MoveNode", Reason: fmt.Sprintf(errUnknownNodeKind, entry.Kind)}
	}

	return nil
}

// DeletePage deletes a page file from disk
func (f *NodeStore) DeletePage(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "DeletePage", Reason: errEntryRequired}
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
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: file, Reason: errExpectedFileMissing}
		}
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		f.log.Warn("delete drift: expected file but found folder", "nodeID", entry.ID, "expectedFile", file)
		return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: file, Reason: errExpectedFileFoundFolder}
	}

	if err := os.Remove(file); err != nil {
		return fmt.Errorf("could not delete file: %w", err)
	}

	return nil
}

// DeleteSection deletes a section folder from disk
func (f *NodeStore) DeleteSection(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "DeleteSection", Reason: errEntryRequired}
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
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: dir, Reason: errExpectedFolderMissing}
		}
		return fmt.Errorf("stat dir: %w", err)
	}
	if !info.IsDir() {
		f.log.Warn("delete drift: expected folder but found file", "nodeID", entry.ID, "expectedDir", dir)
		return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: dir, Reason: errExpectedFolderFoundFile}
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("could not delete folder: %w", err)
	}

	return nil
}

// RenameNode renames a node's slug on disk
func (f *NodeStore) RenameNode(entry *PageNode, newSlug string) error {
	if entry == nil {
		return &InvalidOpError{Op: "RenameNode", Reason: errEntryRequired}
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

	// Guard against ignored paths
	rel, _ := filepath.Rel(filepath.Join(f.storageDir, "root"), newBase)
	if f.isPathIgnored(rel, entry.Kind == NodeKindSection) {
		return &InvalidOpError{Op: "RenameNode", Reason: "target path matches .leafwikiignore"}
	}

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
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcDir, Reason: errExpectedFolderMissing}
			}
			return fmt.Errorf("stat source dir: %w", err)
		}
		if !info.IsDir() {
			// drift: tree says section but disk is not a folder
			f.log.Warn("drift: tree says section but disk is not a folder", "srcDir", srcDir)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcDir, Reason: errExpectedFolderFoundFile}
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
				return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcFile, Reason: errExpectedFileMissing}
			}
			return fmt.Errorf("stat source file: %w", err)
		}
		if info.IsDir() {
			// drift: tree says page but disk is a dir
			f.log.Warn("drift: tree says page but disk is a dir", "srcFile", srcFile)
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: srcFile, Reason: errExpectedFileFoundFolder}
		}

		if err := os.Rename(srcFile, dstFile); err != nil {
			return fmt.Errorf("could not rename file: %w", err)
		}
		return nil

	default:
		return &InvalidOpError{Op: "RenameNode", Reason: fmt.Sprintf(errUnknownNodeKind, entry.Kind)}
	}
}

// SyncFrontmatterIfExists updates the frontmatter of a page file on disk if it exists
func (f *NodeStore) SyncFrontmatterIfExists(entry *PageNode) error {
	if entry == nil {
		return &InvalidOpError{Op: "SyncFrontmatterIfExists", Reason: errEntryRequired}
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
			return &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: filePath, Reason: errExpectedPageMissing}
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

// ConvertNode converts the on-disk representation between page <-> folder.
// NOTE: TreeService must ensure folder->page is allowed (no children).
func (f *NodeStore) ConvertNode(entry *PageNode, target NodeKind) error {
	if entry == nil {
		return &InvalidOpError{Op: "ConvertNode", Reason: errEntryRequired}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return err
	}
	filePath := base + ".md"
	folderPath := base
	indexPath := filepath.Join(folderPath, indexFilename)

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
			return &DriftError{NodeID: entry.ID, Kind: NodeKindSection, Path: folderPath, Reason: errExpectedFolderFoundFile}
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
			if name == indexFilename || name == orderFilename {
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

// SetPinnedFrontmatter updates the leafwiki_pinned flag in the page file on disk.
// Returns the body content (without frontmatter) for use in the caller.
func (f *NodeStore) SetPinnedFrontmatter(entry *PageNode, pinned bool) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "SetPinnedFrontmatter", Reason: errEntryRequired}
	}

	filePath, err := f.contentPathForNodeRead(entry)
	if err != nil {
		return "", err
	}

	if !fileExists(filePath) {
		if entry.Kind == NodeKindPage || entry.Kind == "" {
			return "", &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: filePath, Reason: errExpectedPageMissing}
		}
		return "", &InvalidOpError{Op: "SetPinnedFrontmatter", Reason: "section has no index file and cannot be pinned"}
	}

	mdFile, err := markdown.LoadMarkdownFile(filePath)
	if err != nil {
		return "", fmt.Errorf("load markdown file: %w", err)
	}

	mdFile.SetLeafWikiPinned(pinned)

	if err := mdFile.WriteToFile(); err != nil {
		return "", fmt.Errorf("write markdown file: %w", err)
	}

	return mdFile.GetContent(), nil
}
