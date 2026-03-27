package importer

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"

	"github.com/perber/wiki/internal/core/markdown"
	yaml "gopkg.in/yaml.v3"
)

type ExecutionResult struct {
	ImportedCount  int                   `json:"imported_count"`
	UpdatedCount   int                   `json:"updated_count"`
	SkippedCount   int                   `json:"skipped_count"`
	Items          []ExecutionItemResult `json:"items"`
	TreeHash       string                `json:"tree_hash"`        // hash of the state of the wiki tree after import
	TreeHashBefore string                `json:"tree_hash_before"` // hash of the state of the wiki tree before import
}

type ExecutionAction string

const (
	ExecutionActionCreated ExecutionAction = "created"
	ExecutionActionUpdated ExecutionAction = "updated"
	ExecutionActionSkipped ExecutionAction = "skipped"
)

type ExecutionItemResult struct {
	SourcePath string          `json:"source_path"`
	TargetPath string          `json:"target_path"`
	Action     ExecutionAction `json:"action"`
	Error      *string         `json:"error,omitempty"`
	Notes      []string        `json:"notes,omitempty"`
}

type Executor struct {
	plan        *PlanResult
	planOptions *PlanOptions
	wiki        ImporterWiki
	logger      *slog.Logger
}

func NewExecutor(plan *PlanResult, planOptions *PlanOptions, wiki ImporterWiki, logger *slog.Logger) *Executor {
	return &Executor{
		plan:        plan,
		planOptions: planOptions,
		wiki:        wiki,
		logger:      logger.With("component", "ImporterExecutor"),
	}
}

func buildImportedContent(mdFile *markdown.MarkdownFile) (string, error) {
	body := mdFile.GetContent()
	extraFields := mdFile.GetFrontmatter().ExtraFields
	if len(extraFields) == 0 {
		return body, nil
	}

	mapping := &yaml.Node{Kind: yaml.MappingNode}
	keys := make([]string, 0, len(extraFields))
	for key := range extraFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		var valueNode yaml.Node
		if err := valueNode.Encode(extraFields[key]); err != nil {
			return "", err
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&valueNode,
		)
	}

	rawFM, err := yaml.Marshal(mapping)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(rawFM)
	out.WriteString("---\n")
	out.WriteString(body)
	return out.String(), nil
}

// Execute runs the import based on the provided plan
func (e *Executor) Execute(userID string) (*ExecutionResult, error) {
	beforeExecution := e.wiki.TreeHash()
	if e.plan.TreeHash != beforeExecution {
		return nil, fmt.Errorf("plan is stale: expected tree_hash %s but got %s", e.plan.TreeHash, beforeExecution)
	}

	result := &ExecutionResult{
		TreeHashBefore: beforeExecution,
	}

	for _, item := range e.plan.Items {
		execItem := ExecutionItemResult{
			SourcePath: item.SourcePath,
			TargetPath: item.TargetPath,
			Notes:      append([]string{}, item.Notes...),
			Error:      nil,
		}

		switch item.Action {
		case PlanActionCreate:
			// Creates the page or section and also all necessary parent sections
			page, err := e.wiki.EnsurePath(userID, item.TargetPath, item.Title, &item.Kind)
			if err != nil {
				errMsg := err.Error()
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Failed to ensure path", "target_path", item.TargetPath, "error", err)
				continue
			}
			// Read the content from the source path
			// And update the page content
			if page == nil {
				errMsg := "could not create page"
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Could not create page", "target_path", item.TargetPath, "error", errMsg)
				continue
			}
			sourceAbs := filepath.Join(e.planOptions.SourceBasePath, filepath.FromSlash(item.SourcePath))
			mdFile, err := markdown.LoadMarkdownFile(sourceAbs)
			if err != nil {
				errMsg := err.Error()
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Failed to load source file", "source_path", sourceAbs, "error", err)
				continue
			}
			importedContent, err := buildImportedContent(mdFile)
			if err != nil {
				errMsg := err.Error()
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Failed to prepare imported content", "source_path", sourceAbs, "error", err)
				continue
			}
			if _, err := e.wiki.UpdatePage(userID, page.ID, page.Title, page.Slug, &importedContent, &page.Kind); err != nil {
				errMsg := err.Error()
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Failed to update page content", "page_id", page.ID, "error", err)
				continue
			}
			execItem.Action = ExecutionActionCreated
			result.ImportedCount++
			e.logger.Info("Imported page", "source_path", item.SourcePath, "target_path", item.TargetPath, "page_id", page.ID)
		case PlanActionSkip:
			execItem.Action = ExecutionActionSkipped
			e.logger.Info("Skipped page", "source_path", item.SourcePath, "target_path", item.TargetPath)
			result.SkippedCount++
		default:
			errMsg := "unknown action"
			execItem.Action = ExecutionActionSkipped
			execItem.Error = &errMsg
			e.logger.Info("Skipped page with unknown action", "source_path", item.SourcePath, "target_path", item.TargetPath)
			result.SkippedCount++
		}

		result.Items = append(result.Items, execItem)
	}

	result.TreeHash = e.wiki.TreeHash()

	return result, nil
}
