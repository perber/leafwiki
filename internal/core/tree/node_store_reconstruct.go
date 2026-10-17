package tree

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/shared"
)

// ensureUniqueReconstructedID records id as seen for this reconstruct walk.
// It returns the path of the earlier occurrence if id is a duplicate (the
// caller should skip this node rather than add it to the tree), or an error
// only if id is empty (defensive — callers always default to a freshly
// generated ID before this is called, so this should be unreachable).
func ensureUniqueReconstructedID(seenIDs map[string]string, id string, path string) (conflictPath string, err error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return "", fmt.Errorf("reconstruct tree from fs: empty leafwiki_id at %s", path)
	}
	if existingPath, exists := seenIDs[trimmedID]; exists {
		return existingPath, nil
	}
	seenIDs[trimmedID] = path
	return "", nil
}

func ensureUniqueReconstructedSlug(seenSlugs map[string]string, slug string, path string) error {
	key := strings.ToLower(strings.TrimSpace(slug))
	if key == "" {
		return fmt.Errorf("reconstruct tree from fs: empty slug at %s", path)
	}
	if existingPath, exists := seenSlugs[key]; exists {
		parentDir := filepath.Base(filepath.Dir(path))
		return fmt.Errorf(
			"duplicate slug %q: a directory and a .md file share the same name in %s/. "+
				"Rename or remove one of them (e.g. rename %s.md -> %s-page.md) to resolve the conflict. "+
				"Conflicting paths: %s and %s",
			slug, parentDir, slug, slug, existingPath, path,
		)
	}
	seenSlugs[key] = path
	return nil
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
	return f.ReconstructTreeFromFSContext(context.Background())
}

func (f *NodeStore) ReconstructTreeFromFSContext(ctx context.Context) (*PageNode, error) {
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

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := f.reconstructTreeRecursive(ctx, rootDir, root, reconstructNow, seenIDs); err != nil {
		return nil, fmt.Errorf("reconstruct tree from fs: %w", err)
	}

	return root, nil
}

func (f *NodeStore) reconstructTreeRecursive(ctx context.Context, currentPath string, parent *PageNode, reconstructNow time.Time, seenIDs map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", currentPath, err)
	}
	seenSlugs := map[string]string{}

	sortReconstructionEntries(entries)

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		name := entry.Name()

		// optional: skip hidden stuff
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Check .leafwikiignore
		if ig := f.getIgnoreForDir(currentPath); ig != nil {
			relPath, _ := filepath.Rel(filepath.Join(f.storageDir, "root"), filepath.Join(currentPath, name))
			if relPath != "" && ig.Matches(filepath.ToSlash(relPath), entry.IsDir()) {
				continue
			}
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

			indexPath := filepath.Join(currentPath, name, indexFilename)
			var sectionMdFile *markdown.MarkdownFile
			needsWriteback := false
			sectionPinned := false
			if fileExists(indexPath) {
				mdFile, err := markdown.LoadMarkdownFile(indexPath)
				if err != nil {
					f.log.Error("could not load index.md", "path", indexPath, "error", err)
					// fall back to default title and generated ID, but still add the section and recurse
				} else {
					fm := mdFile.GetFrontmatter()
					if fm.WasRepaired() {
						f.log.Warn("frontmatter did not parse as-is and was auto-repaired", "path", indexPath)
					}
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
					sectionPinned = fm.LeafWikiPinned
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
				Pinned:   sectionPinned,
			}
			if err := ensureUniqueReconstructedSlug(seenSlugs, child.Slug, filepath.Join(currentPath, name)); err != nil {
				return err
			}
			conflictPath, err := ensureUniqueReconstructedID(seenIDs, child.ID, indexPath)
			if err != nil {
				return err
			}
			if conflictPath != "" {
				f.log.Warn("skipping section: duplicate leafwiki_id, keeping first occurrence",
					"leafwikiID", child.ID, "path", indexPath, "conflictingPath", conflictPath)
				continue
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

			if err := f.reconstructTreeRecursive(ctx, filepath.Join(currentPath, name), child, reconstructNow, seenIDs); err != nil {
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
		if fm.WasRepaired() {
			f.log.Warn("frontmatter did not parse as-is and was auto-repaired", "path", filePath)
		}
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
			Pinned:   fm.LeafWikiPinned,
		}
		if err := ensureUniqueReconstructedSlug(seenSlugs, child.Slug, filePath); err != nil {
			return err
		}
		conflictPath, err := ensureUniqueReconstructedID(seenIDs, child.ID, filePath)
		if err != nil {
			return err
		}
		if conflictPath != "" {
			f.log.Warn("skipping page: duplicate leafwiki_id, keeping first occurrence",
				"leafwikiID", child.ID, "path", filePath, "conflictingPath", conflictPath)
			continue
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

func (f *NodeStore) assignParentToChildren(parent *PageNode) {
	for _, child := range parent.Children {
		child.Parent = parent
		f.assignParentToChildren(child)
	}
}

func sortReconstructionEntries(entries []os.DirEntry) {
	// os.ReadDir already sorts by name. Most directories use lowercase slugs,
	// so avoid allocating sort keys when that order also matches case folding.
	if len(entries) < 2 || sort.SliceIsSorted(entries, func(i, j int) bool {
		left, right := entries[i].Name(), entries[j].Name()
		foldedLeft, foldedRight := strings.ToLower(left), strings.ToLower(right)
		if foldedLeft == foldedRight {
			return left < right
		}
		return foldedLeft < foldedRight
	}) {
		return
	}
	type keyedEntry struct {
		entry  os.DirEntry
		folded string
	}
	keyed := make([]keyedEntry, len(entries))
	for i, entry := range entries {
		name := entry.Name()
		keyed[i] = keyedEntry{entry: entry, folded: strings.ToLower(name)}
	}
	// Case-sensitive names break ties after Unicode case folding.
	sort.SliceStable(keyed, func(i, j int) bool {
		if keyed[i].folded == keyed[j].folded {
			return keyed[i].entry.Name() < keyed[j].entry.Name()
		}
		return keyed[i].folded < keyed[j].folded
	})
	for i, entry := range keyed {
		entries[i] = entry.entry
	}
}
