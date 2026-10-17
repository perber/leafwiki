package tree

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/ignore"
	"github.com/perber/wiki/internal/core/markdown"
)

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

type ResolvedNode struct {
	Kind       NodeKind
	DirPath    string
	FilePath   string
	HasContent bool
}

type NodeStore struct {
	storageDir  string
	log         *slog.Logger
	slugger     *SlugService
	ignoreCache *ignore.Cache
}

// SetIgnoreCache sets the ignore cache to use for multi-level ignore resolution.
func (f *NodeStore) SetIgnoreCache(ignoreCache *ignore.Cache) {
	f.ignoreCache = ignoreCache
}

const (
	reconstructSystemUserID    = "system"
	orderFilename              = ".order.json"
	indexFilename              = "index.md"
	errEntryRequired           = "an entry is required"
	errParentEntryRequired     = "a parent entry is required"
	errExpectedPageMissing     = "expected page file missing"
	errExpectedFolderMissing   = "expected folder missing"
	errExpectedFolderFoundFile = "expected folder but found file"
	errExpectedFileMissing     = "expected file missing"
	errExpectedFileFoundFolder = "expected file but found folder"
	errUnknownNodeKind         = "unknown node kind: %q"
	errLoadMarkdownFailed      = "could not load markdown file: %w"
	errWriteMarkdownFailed     = "could not write markdown file: %w"
	errEnsureParentFailed      = "could not ensure parent directory exists: %w"
)

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
		return "", &InvalidOpError{Op: "ensureSectionIndex", Reason: errEntryRequired}
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
			return "", fmt.Errorf(errLoadMarkdownFailed, err)
		}
	}

	f.syncManagedFrontmatter(mdFile, entry)
	if err := mdFile.WriteToFile(); err != nil {
		return "", fmt.Errorf(errWriteMarkdownFailed, err)
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
			return "", &DriftError{NodeID: entry.ID, Kind: entry.Kind, Path: filePath, Reason: errExpectedPageMissing}
		}
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ReadPageAndRaw returns both the stripped content and the raw markdown string
// (including frontmatter) from a single disk read.
func (f *NodeStore) ReadPageAndRaw(entry *PageNode) (content, raw string, err error) {
	raw, err = f.ReadPageRaw(entry)
	if err != nil || raw == "" {
		return "", raw, err
	}

	filePath, err := f.contentPathForNodeRead(entry)
	if err != nil {
		return "", raw, err
	}

	mdFile, err := markdown.NewMarkdownFileFromRaw(filePath, raw)
	if err != nil {
		return raw, raw, err
	}

	return mdFile.GetContent(), raw, nil
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

// getIgnoreForDir returns the compiled ignore rules for the given directory,
// delegating to the shared cache. Returns nil if no rules apply.
func (f *NodeStore) getIgnoreForDir(dir string) *ignore.IgnoreFile {
	if f.ignoreCache == nil {
		return nil
	}
	return f.ignoreCache.Get(dir)
}

// isPathIgnored checks if a relative path matches the ignore rules
// governed by the nearest .leafwikiignore file.
func (f *NodeStore) isPathIgnored(relPath string, isDir bool) bool {
	if relPath == "" {
		return false
	}
	// Resolve the absolute path to find the enclosing directory
	absPath := filepath.Join(f.storageDir, "root", relPath)
	dir := filepath.Dir(absPath)
	ig := f.getIgnoreForDir(dir)
	if ig == nil {
		return false
	}
	return ig.Matches(filepath.ToSlash(relPath), isDir)
}

func (f *NodeStore) dirPathForNode(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "dirPathForNode", Reason: errEntryRequired}
	}
	return filepath.Join(f.storageDir, GeneratePathFromPageNode(entry)), nil
}

// contentPathForNodeRead returns the expected content file path for a node
// based purely on the tree Kind (NO side effects, NO mkdir):
// - page   => <base>.md
// - section => <base>/index.md
func (f *NodeStore) contentPathForNodeRead(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "contentPathForNodeRead", Reason: errEntryRequired}
	}

	base, err := f.dirPathForNode(entry)
	if err != nil {
		return "", err
	}
	switch entry.Kind {
	case NodeKindSection:
		return filepath.Join(base, indexFilename), nil
	case NodeKindPage:
		return base + ".md", nil
	default:
		return "", &InvalidOpError{Op: "contentPathForNodeRead", Reason: fmt.Sprintf(errUnknownNodeKind, entry.Kind)}
	}
}

// contentPathForNodeWrite returns the expected content file path for a node
// based purely on the tree Kind (MAY create dirs for sections):
// - page   => <base>.md
// - section => <base>/index.md (ensures directory exists)
func (f *NodeStore) contentPathForNodeWrite(entry *PageNode) (string, error) {
	if entry == nil {
		return "", &InvalidOpError{Op: "contentPathForNodeWrite", Reason: errEntryRequired}
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
		return filepath.Join(base, indexFilename), nil

	case NodeKindPage:
		return base + ".md", nil

	default:
		return "", &InvalidOpError{Op: "contentPathForNodeWrite", Reason: fmt.Sprintf(errUnknownNodeKind, entry.Kind)}
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
		index := filepath.Join(basePath, indexFilename)
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
