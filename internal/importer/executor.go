package importer

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/markdown"
)

type ExecutionResult struct {
	ImportedCount  int                   `json:"imported_count"`
	UpdatedCount   int                   `json:"updated_count"`
	SkippedCount   int                   `json:"skipped_count"`
	Items          []ExecutionItemResult `json:"items"`
	TreeHash       string                `json:"tree_hash"`        // hash of the state of the wiki tree after import
	TreeHashBefore string                `json:"tree_hash_before"` // hash of the state of the wiki tree before import
}

type ExecutionProgress struct {
	ProcessedItems        int        `json:"processed_items"`
	TotalItems            int        `json:"total_items"`
	CurrentItemSourcePath *string    `json:"current_item_source_path,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	FinishedAt            *time.Time `json:"finished_at,omitempty"`
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
	plan          *PlanResult
	planOptions   *PlanOptions
	assetMaxBytes int64
	wiki          ImporterWiki
	logger        *slog.Logger
	progressFn    func(ExecutionProgress, *ExecutionResult)
	cancelFn      func() bool
	startIndex    int
	initialResult *ExecutionResult
}

func NewExecutor(plan *PlanResult, planOptions *PlanOptions, assetMaxBytes int64, wiki ImporterWiki, logger *slog.Logger) *Executor {
	if assetMaxBytes <= 0 {
		assetMaxBytes = assets.DefaultMaxUploadSizeBytes
	}
	return &Executor{
		plan:          plan,
		planOptions:   planOptions,
		assetMaxBytes: assetMaxBytes,
		wiki:          wiki,
		logger:        logger.With("component", "ImporterExecutor"),
	}
}

func (e *Executor) WithProgressCallback(progressFn func(ExecutionProgress, *ExecutionResult)) *Executor {
	e.progressFn = progressFn
	return e
}

func (e *Executor) WithCancelCheck(cancelFn func() bool) *Executor {
	e.cancelFn = cancelFn
	return e
}

func (e *Executor) WithResumeState(startIndex int, initialResult *ExecutionResult) *Executor {
	e.startIndex = startIndex
	e.initialResult = cloneExecutionResult(initialResult)
	return e
}

func buildImportedContent(mdFile *markdown.MarkdownFile) (string, error) {
	return markdown.BuildMarkdownWithExtraFrontmatter(mdFile.GetFrontmatter().ExtraFields, mdFile.GetContent())
}

// Execute runs the import based on the provided plan
func (e *Executor) Execute(userID string) (*ExecutionResult, error) {
	beforeExecution := e.wiki.TreeHash()
	expectedTreeHash := e.plan.TreeHash
	if e.startIndex > 0 {
		if e.initialResult == nil || e.initialResult.TreeHash == "" {
			return nil, fmt.Errorf("resume state missing tree hash")
		}
		expectedTreeHash = e.initialResult.TreeHash
	}
	if expectedTreeHash != beforeExecution {
		return nil, fmt.Errorf("plan is stale: expected tree_hash %s but got %s", expectedTreeHash, beforeExecution)
	}

	transformer := newContentTransformer(e.plan, e.planOptions.SourceBasePath, e.assetMaxBytes)
	startedAt := time.Now()

	result := cloneExecutionResult(e.initialResult)
	if result == nil {
		result = &ExecutionResult{
			TreeHashBefore: beforeExecution,
		}
	}
	if result.TreeHashBefore == "" {
		result.TreeHashBefore = beforeExecution
	}

	e.reportProgress(ExecutionProgress{
		ProcessedItems: e.startIndex,
		TotalItems:     len(e.plan.Items),
		StartedAt:      &startedAt,
	}, result)

	for index := e.startIndex; index < len(e.plan.Items); index++ {
		if e.cancelFn != nil && e.cancelFn() {
			return result, ErrImportCanceled
		}

		item := e.plan.Items[index]
		currentItemSourcePath := item.SourcePath
		e.reportProgress(ExecutionProgress{
			ProcessedItems:        index,
			TotalItems:            len(e.plan.Items),
			CurrentItemSourcePath: &currentItemSourcePath,
			StartedAt:             &startedAt,
		}, result)

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
			importedContent, err = transformer.TransformContent(userID, item.SourcePath, page, importedContent, e.wiki)
			if err != nil {
				errMsg := err.Error()
				execItem.Action = ExecutionActionSkipped
				execItem.Error = &errMsg
				result.SkippedCount++
				result.Items = append(result.Items, execItem)
				e.logger.Error("Failed to transform imported content", "source_path", sourceAbs, "error", err)
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
		result.TreeHash = e.wiki.TreeHash()
		e.reportProgress(ExecutionProgress{
			ProcessedItems: index + 1,
			TotalItems:     len(e.plan.Items),
			StartedAt:      &startedAt,
		}, result)
	}

	result.TreeHash = e.wiki.TreeHash()
	finishedAt := time.Now()
	e.reportProgress(ExecutionProgress{
		ProcessedItems: len(e.plan.Items),
		TotalItems:     len(e.plan.Items),
		StartedAt:      &startedAt,
		FinishedAt:     &finishedAt,
	}, result)

	return result, nil
}

func (e *Executor) reportProgress(progress ExecutionProgress, result *ExecutionResult) {
	if e.progressFn == nil {
		return
	}
	e.progressFn(progress, result)
}
