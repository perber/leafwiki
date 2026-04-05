package importer

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/assets"
)

type ImporterService struct {
	planner                 *Planner
	planStore               *PlanStore
	extractor               *ZipExtractor
	logger                  *slog.Logger
	assetMaxUploadSizeBytes int64
	workspaceBaseDir        string
}

type CurrentPlanState struct {
	ID              string           `json:"id"`
	TreeHash        string           `json:"tree_hash"`
	Items           []PlanItem       `json:"items"`
	Errors          []string         `json:"errors"`
	ExecutionStatus ExecutionStatus  `json:"execution_status"`
	CancelRequested bool             `json:"cancel_requested"`
	ExecutionResult *ExecutionResult `json:"execution_result,omitempty"`
	ExecutionError  *string          `json:"execution_error,omitempty"`
	ExecutionProgress
}

func NewImporterService(planner *Planner, planStore *PlanStore, workspaceBaseDir string, assetMaxUploadSizeBytes int64) *ImporterService {
	if assetMaxUploadSizeBytes <= 0 {
		assetMaxUploadSizeBytes = assets.DefaultMaxUploadSizeBytes
	}
	if workspaceBaseDir == "" {
		workspaceBaseDir = filepath.Join(os.TempDir(), "wiki-imports")
	}
	service := &ImporterService{
		planner:                 planner,
		planStore:               planStore,
		extractor:               NewZipExtractor(),
		logger:                  slog.Default().With("component", "ImporterService"),
		assetMaxUploadSizeBytes: assetMaxUploadSizeBytes,
		workspaceBaseDir:        workspaceBaseDir,
	}
	service.resumeInterruptedExecution()
	return service
}

// CreateImportPlanFromFolder creates an import plan from a folder path
func (is *ImporterService) createImportPlanFromFolder(folderPath string, targetBasePath string) (*PlanResult, error) {
	// single-plan semantics: cleanup old plan workspace if present
	if old, err := is.planStore.Get(); err == nil && old != nil {
		if old.ExecutionStatus == ExecutionStatusRunning {
			return nil, ErrImportExecutionRunning
		}
		err = os.RemoveAll(old.WorkspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("cleanup old import workspace: %w", err)
		}
		if _, err := is.planStore.Clear(); err != nil {
			return nil, err
		}
		is.logger.Info("Old import workspace cleaned up")
	}

	entries, err := FindMarkdownEntries(folderPath)
	if err != nil {
		return nil, err
	}

	opts := PlanOptions{
		SourceBasePath: folderPath,
		TargetBasePath: targetBasePath,
	}

	plan, err := is.planner.CreatePlan(entries, opts)
	if err != nil {
		return nil, err
	}

	if err := is.planStore.Set(&StoredPlan{
		Plan:            plan,
		PlanOptions:     opts,
		WorkspaceRoot:   folderPath,
		CreatedAt:       time.Now(),
		ExecutionStatus: ExecutionStatusPlanned,
	}); err != nil {
		return nil, err
	}
	is.logger.Info("Import plan created", "entries", len(entries), "workspace", folderPath)
	return plan, nil
}

// GetCurrentPlan retrieves the currently stored import plan
func (is *ImporterService) GetCurrentPlan() (*CurrentPlanState, error) {
	sp, err := is.planStore.Get()
	if err != nil {
		return nil, err
	}
	return currentPlanStateFromStored(sp), nil
}

// ClearCurrentPlan clears the currently stored import plan
func (is *ImporterService) ClearCurrentPlan() error {
	if sp, err := is.planStore.Get(); err == nil && sp != nil {
		if sp.ExecutionStatus == ExecutionStatusRunning {
			return ErrImportExecutionRunning
		}
		if err := os.RemoveAll(sp.WorkspaceRoot); err != nil {
			is.logger.Error("remove workspace failed", "error", err)
		}
	}
	_, err := is.planStore.Clear()
	return err
}

func (is *ImporterService) CancelCurrentPlan() (*CurrentPlanState, bool, error) {
	sp, requested, err := is.planStore.RequestCancel()
	if err != nil {
		return nil, false, err
	}
	return currentPlanStateFromStored(sp), requested, nil
}

// ExecuteCurrentPlan executes the currently stored import plan
func (is *ImporterService) ExecuteCurrentPlan(userID string) (*ExecutionResult, error) {
	sp, started, err := is.planStore.TryStartExecution(userID)
	if err != nil {
		return nil, err
	}
	if !started {
		switch sp.ExecutionStatus {
		case ExecutionStatusRunning:
			return nil, ErrImportExecutionRunning
		case ExecutionStatusCompleted:
			if sp.ExecutionResult == nil {
				return nil, errors.New("import completed without result")
			}
			return sp.ExecutionResult, nil
		case ExecutionStatusFailed:
			if sp.ExecutionError != nil {
				return nil, errors.New(*sp.ExecutionError)
			}
			return nil, errors.New("import execution failed")
		}
	}

	res, err := is.executeStoredPlan(sp)
	if finishErr := is.planStore.FinishExecution(sp.Plan.ID, res, err); finishErr != nil {
		return nil, finishErr
	}
	is.cleanupWorkspace(sp.WorkspaceRoot)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (is *ImporterService) StartCurrentPlanExecution(userID string) (*CurrentPlanState, bool, error) {
	sp, started, err := is.planStore.TryStartExecution(userID)
	if err != nil {
		return nil, false, err
	}

	if started {
		go func(snapshot *StoredPlan) {
			res, execErr := is.executeStoredPlan(snapshot)
			if finishErr := is.planStore.FinishExecution(snapshot.Plan.ID, res, execErr); finishErr != nil {
				is.logger.Error("failed to persist finished import state", "error", finishErr)
				return
			}
			is.cleanupWorkspace(snapshot.WorkspaceRoot)
		}(sp)
	}

	return currentPlanStateFromStored(sp), started, nil
}

// FindMarkdownEntries finds markdown files in the given source base path
func FindMarkdownEntries(sourceBasePath string) ([]ImportMDFile, error) {
	out := []ImportMDFile{}

	err := filepath.WalkDir(sourceBasePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(sourceBasePath, p)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}

		out = append(out, ImportMDFile{
			SourcePath: filepath.ToSlash(rel),
		})
		return nil
	})

	// Order entries by depth (shallow first)
	if err == nil {
		sort.SliceStable(out, func(i, j int) bool {
			depthI := strings.Count(out[i].SourcePath, "/")
			depthJ := strings.Count(out[j].SourcePath, "/")
			if depthI == depthJ {
				return out[i].SourcePath < out[j].SourcePath
			}
			return depthI < depthJ
		})
	}

	// index.md should always come first if present
	if err == nil {
		sort.SliceStable(out, func(i, j int) bool {
			nameI := strings.ToLower(filepath.Base(out[i].SourcePath))
			nameJ := strings.ToLower(filepath.Base(out[j].SourcePath))
			if nameI == "index.md" && nameJ != "index.md" {
				return true
			}
			if nameJ == "index.md" && nameI != "index.md" {
				return false
			}
			return false
		})
	}

	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateImportPlanFromZipUpload creates an import plan from an uploaded zip file
func (is *ImporterService) CreateImportPlanFromZipUpload(
	r io.Reader,
	targetBasePath string,
) (*PlanResult, error) {
	ws, err := is.extractZipReaderToTemp(r)
	if err != nil {
		return nil, fmt.Errorf("extract zip to temp: %w", err)
	}

	plan, err := is.createImportPlanFromFolder(ws.Root, targetBasePath)
	if err != nil {
		if err := ws.Cleanup(); err != nil {
			is.logger.Error("cleanup failed", "error", err)
		}
		return nil, fmt.Errorf("create import plan from folder: %w", err)
	}
	return plan, nil
}

func (is *ImporterService) extractZipReaderToTemp(r io.Reader) (*ZipWorkspace, error) {
	if err := os.MkdirAll(is.workspaceBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create import temp dir: %w", err)
	}

	tmp, err := os.CreateTemp(is.workspaceBaseDir, "upload-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp zip: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil {
			is.logger.Error("remove temp zip failed", "error", err)
		}
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("store uploaded zip: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp zip: %w", err)
	}

	ws, err := is.extractor.ExtractToDir(tmpPath, is.workspaceBaseDir)
	if err != nil {
		return nil, fmt.Errorf("extract zip: %w", err)
	}
	return ws, nil
}

func (is *ImporterService) executeStoredPlan(sp *StoredPlan) (*ExecutionResult, error) {
	exec := NewExecutor(sp.Plan, &sp.PlanOptions, is.assetMaxUploadSizeBytes, is.planner.wiki, is.planner.log).
		WithProgressCallback(func(progress ExecutionProgress, result *ExecutionResult) {
			if err := is.planStore.UpdateExecutionProgress(sp.Plan.ID, progress, result); err != nil {
				is.logger.Error("failed to persist import progress", "error", err)
			}
		}).
		WithCancelCheck(func() bool {
			return is.planStore.IsCancelRequested(sp.Plan.ID)
		}).
		WithResumeState(sp.ProcessedItems, sp.ExecutionResult)
	return exec.Execute(sp.ExecutionUserID)
}

func (is *ImporterService) cleanupWorkspace(workspaceRoot string) {
	if workspaceRoot == "" {
		return
	}
	if err := os.RemoveAll(workspaceRoot); err != nil {
		is.logger.Error("remove workspace failed", "error", err)
	}
}

func currentPlanStateFromStored(sp *StoredPlan) *CurrentPlanState {
	if sp == nil || sp.Plan == nil {
		return nil
	}

	return &CurrentPlanState{
		ID:              sp.Plan.ID,
		TreeHash:        sp.Plan.TreeHash,
		Items:           sp.Plan.Items,
		Errors:          sp.Plan.Errors,
		ExecutionStatus: sp.ExecutionStatus,
		CancelRequested: sp.CancelRequested,
		ExecutionResult: sp.ExecutionResult,
		ExecutionError:  sp.ExecutionError,
		ExecutionProgress: ExecutionProgress{
			ProcessedItems:        sp.ProcessedItems,
			TotalItems:            sp.TotalItems,
			CurrentItemSourcePath: sp.CurrentItemSourcePath,
			StartedAt:             sp.StartedAt,
			FinishedAt:            sp.FinishedAt,
		},
	}
}

func (is *ImporterService) resumeInterruptedExecution() {
	sp, err := is.planStore.Get()
	if err != nil || sp == nil {
		if err != nil && !errors.Is(err, ErrNoPlan) {
			is.logger.Error("failed to load persisted importer state", "error", err)
		}
		return
	}
	if sp.ExecutionStatus != ExecutionStatusRunning {
		return
	}
	if sp.ExecutionUserID == "" {
		sp.ExecutionUserID = "system"
	}

	go func(snapshot *StoredPlan) {
		res, execErr := is.executeStoredPlan(snapshot)
		if finishErr := is.planStore.FinishExecution(snapshot.Plan.ID, res, execErr); finishErr != nil {
			is.logger.Error("failed to persist resumed import completion", "error", finishErr)
			return
		}
		is.cleanupWorkspace(snapshot.WorkspaceRoot)
	}(sp)
}
