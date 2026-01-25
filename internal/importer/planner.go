package importer

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/tree"
)

type PlanAction string

const (
	PlanActionCreate PlanAction = "create" // creates new node
	PlanActionUpdate PlanAction = "update" // updates existing node
	PlanActionSkip   PlanAction = "skip"   // skips existing node
)

// ImportMDFile represents a markdown file to be imported
type ImportMDFile struct {
	SourcePath string // relative path to the markdown file in the zip directory
}

// PlanItem represents a single item in the import plan
type PlanItem struct {
	SourcePath  string        `json:"source_path"`
	TargetPath  string        `json:"target_path"`
	Title       string        `json:"title"`
	DesiredSlug string        `json:"desired_slug"`
	Kind        tree.NodeKind `json:"kind"`
	Exists      bool          `json:"exists"`
	ExistingID  *string       `json:"existing_id"`

	Action    PlanAction `json:"action"`
	Conflicts []string   `json:"conflicts"`
	Notes     []string   `json:"notes"`
}

// PlanOptions represents options for creating an import plan
type PlanOptions struct {
	SourceBasePath string // base path in the import source
	TargetBasePath string // base path in the wiki where to import
}

// PlanResult represents the result of the import plan
type PlanResult struct {
	ID       string     `json:"id"`
	TreeHash string     `json:"tree_hash"` // hash of the state of the wiki tree before import
	Items    []PlanItem `json:"items"`
	Errors   []string   `json:"errors"`
}

// Planner is responsible for creating an import plan
type Planner struct {
	log     *slog.Logger
	wiki    ImporterWiki
	slugger *tree.SlugService
}

// NewPlanner creates a new Planner
func NewPlanner(wiki ImporterWiki, slugger *tree.SlugService) *Planner {
	return &Planner{
		log:     slog.Default().With("component", "Planner"),
		wiki:    wiki,
		slugger: slugger,
	}
}

// CreatePlan creates an import plan based on the provided entries and options
func (p *Planner) CreatePlan(entries []ImportMDFile, options PlanOptions) (*PlanResult, error) {
	// Generate a unique ID for the new page
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("could not generate unique ID: %w", err)
	}
	result := &PlanResult{
		ID:       id,
		Items:    []PlanItem{},
		Errors:   []string{},
		TreeHash: p.wiki.TreeHash(),
	}
	for _, entry := range entries {
		resEntry, err := p.analyzeEntry(entry, options)
		if err != nil {
			p.log.Warn("could not import resource", "source_path", entry.SourcePath, "error", err)
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.Items = append(result.Items, *resEntry)
	}
	return result, nil
}

// analyzeEntry analyzes a entry (directory or file) to be imported
func (p *Planner) analyzeEntry(mdFile ImportMDFile, options PlanOptions) (*PlanItem, error) {
	// FS path for reading
	sourcePath := filepath.Join(options.SourceBasePath, filepath.FromSlash(mdFile.SourcePath))

	// Validate if sourcePath exists and is a file
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("source path is a directory, expected a file: " + mdFile.SourcePath)
	}

	// normalize source path (zip-ish)
	rel := filepath.ToSlash(strings.TrimSpace(mdFile.SourcePath))
	rel = strings.TrimPrefix(rel, "/")

	filenameLower := strings.ToLower(path.Base(rel))
	sourceDir := path.Dir(rel)
	if sourceDir == "." {
		sourceDir = ""
	}

	// normalize ONLY the source dir segments
	normalizedSourceDir, err := p.slugger.NormalizePath(sourceDir, true)
	if err != nil {
		return nil, err
	}
	normalizedSourceDir = strings.Trim(normalizedSourceDir, "/")

	// compute wiki path (route)
	targetBase := strings.Trim(strings.TrimSpace(options.TargetBasePath), "/")

	kind := tree.NodeKindPage
	var wikiPath string

	if filenameLower == "index.md" {
		kind = tree.NodeKindSection
		wikiPath = strings.Trim(path.Join(targetBase, normalizedSourceDir), "/")
	} else {
		normalizedFilename := p.slugger.NormalizeFilename(filenameLower) // e.g. "my-page.md"
		baseSlug := strings.TrimSuffix(normalizedFilename, path.Ext(normalizedFilename))
		wikiPath = strings.Trim(path.Join(targetBase, normalizedSourceDir, baseSlug), "/")
	}

	// lookup existing
	result, err := p.wiki.LookupPagePath(wikiPath)
	if err != nil {
		return nil, err
	}

	title, titleErr := p.extractTitleFromMDFile(sourcePath)
	var notes []string
	if titleErr != nil {
		notes = append(notes, fmt.Sprintf("Failed to extract title from file: %v", titleErr))
	}

	if !result.Exists {

		// slug = last segment
		slug := ""
		if wikiPath != "" {
			segs := strings.Split(wikiPath, "/")
			slug = segs[len(segs)-1]
		}

		return &PlanItem{
			SourcePath:  mdFile.SourcePath,
			TargetPath:  wikiPath,
			Title:       title,
			DesiredSlug: slug,
			Kind:        kind,
			Exists:      false,
			Action:      PlanActionCreate,
			Notes:       notes,
		}, nil
	}

	if len(result.Segments) == 0 {
		return nil, errors.New("invalid lookup result with zero segments for existing path")
	}

	last := result.Segments[len(result.Segments)-1]
	return &PlanItem{
		SourcePath:  mdFile.SourcePath,
		TargetPath:  wikiPath,
		Title:       title,
		DesiredSlug: last.Slug,
		Exists:      true,
		ExistingID:  last.ID,
		Kind:        kind,
		Action:      PlanActionSkip,
		Notes:       notes,
	}, nil
}

func (p *Planner) extractTitleFromMDFile(mdFilePath string) (string, error) {
	// Helper to get filename-based fallback
	filenameFallback := func() string {
		base := path.Base(mdFilePath)
		return strings.TrimSuffix(base, path.Ext(base))
	}

	// Read the file content
	content, err := os.ReadFile(mdFilePath)
	if err != nil {
		// If we can't read the file, return filename as fallback but keep the error
		return filenameFallback(), err
	}

	stripSingleAndDoubleQuotes := func(s string, err error) (string, error) {
		if err != nil {
			return "", err
		}
		s = strings.Trim(s, `"`)
		s = strings.Trim(s, `'`)
		return s, nil
	}

	// Try to extract title from frontmatter
	title, err := stripSingleAndDoubleQuotes(p.extractTitleFromFrontMatter(content))
	if err == nil && title != "" {
		return title, nil
	}

	// Try to extract title from first heading
	title, err = stripSingleAndDoubleQuotes(p.extractTitleFromFirstHeading(content))
	if err == nil && title != "" {
		return title, nil
	}

	// strip extension from filename
	return stripSingleAndDoubleQuotes(filenameFallback(), nil)
}

func (p *Planner) extractTitleFromFrontMatter(content []byte) (string, error) {
	frontMatter, _, has := tree.SplitFrontmatter(string(content))
	if !has {
		return "", errors.New("no frontmatter found")
	}

	// Look for title or leafwiki_title in the frontmatter
	lines := strings.Split(frontMatter, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "title:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "title:")), nil
		}
		if strings.HasPrefix(line, "leafwiki_title:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "leafwiki_title:")), nil
		}
	}
	return "", errors.New("no title found in frontmatter")
}

func (p *Planner) extractTitleFromFirstHeading(content []byte) (string, error) {
	// Simple first heading extraction
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	return "", errors.New("no heading found")
}
