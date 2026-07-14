package tree

import (
	"archive/zip"
	"fmt"
	"io"
)

// ExportSectionZip writes a ZIP archive of the section identified by id — and
// all of its descendant pages and sub-sections — to w.
//
// Each node is written as clean Markdown (system frontmatter stripped) and the
// tree structure is preserved on disk-like paths:
//   - a section → <path>/index.md
//   - a page    → <path>.md
//
// Entries are prefixed with the section's own slug so the archive extracts into
// a single top-level folder. Frontmatter is intentionally omitted so downloads
// stay readable and never leak internal metadata (creator/author IDs) — matching
// the single-page Markdown download.
func (t *TreeService) ExportSectionZip(id string, w io.Writer) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.tree == nil {
		return ErrTreeNotLoaded
	}

	node := t.getNodeByIDLocked(id)
	if node == nil {
		return ErrPageNotFound
	}
	if node.Kind != NodeKindSection {
		return &InvalidOpError{Op: "ExportSectionZip", Reason: "node is not a section"}
	}

	prefix := node.Slug
	if prefix == "" {
		prefix = "section"
	}

	zw := zip.NewWriter(w)
	if err := t.writeNodeToZipLocked(zw, node, prefix); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

// writeNodeToZipLocked recursively writes node and its descendants into zw.
// Callers must hold t.mu (read lock is sufficient).
func (t *TreeService) writeNodeToZipLocked(zw *zip.Writer, node *PageNode, basePath string) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case NodeKindSection:
		content, err := t.store.ReadPageContent(node)
		if err != nil {
			return fmt.Errorf("read section content %q: %w", node.ID, err)
		}
		if err := writeZipEntry(zw, basePath+"/index.md", content); err != nil {
			return err
		}
		for _, child := range node.Children {
			if child == nil {
				continue
			}
			if err := t.writeNodeToZipLocked(zw, child, basePath+"/"+child.Slug); err != nil {
				return err
			}
		}
		return nil
	case NodeKindPage, "":
		content, err := t.store.ReadPageContent(node)
		if err != nil {
			return fmt.Errorf("read page content %q: %w", node.ID, err)
		}
		return writeZipEntry(zw, basePath+".md", content)
	default:
		return &InvalidOpError{Op: "ExportSectionZip", Reason: fmt.Sprintf("unknown node kind: %q", node.Kind)}
	}
}

func writeZipEntry(zw *zip.Writer, name, content string) error {
	fw, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %q: %w", name, err)
	}
	if _, err := io.WriteString(fw, content); err != nil {
		return fmt.Errorf("write zip entry %q: %w", name, err)
	}
	return nil
}
