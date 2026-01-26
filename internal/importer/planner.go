package importer

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
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

	var notes []string
	md, err := markdown.LoadMarkdownFile(sourcePath)
	if err != nil {
		notes = append(notes, fmt.Sprintf("Failed to load markdown file for title extraction: %v", err))
	}

	// Determine fallback title
	title := path.Base(wikiPath) // fallback to last segment of wiki path
	if wikiPath == "" {
		// For root-level index.md or empty paths, use filename without extension
		title = strings.TrimSuffix(filenameLower, path.Ext(filenameLower))
		if title == "" {
			title = "root"
		}
	}

	if md != nil {
		var titleErr error
		title, titleErr = md.GetTitle()
		if titleErr != nil {
			notes = append(notes, fmt.Sprintf("Failed to extract title from file: %v", titleErr))
			title = "unknown" // ensure title is set
		}
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
