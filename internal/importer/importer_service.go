package importer

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ImporterService struct {
	planner   *Planner
	planStore *PlanStore
	extractor *ZipExtractor
	logger    *slog.Logger
}

func NewImporterService(planner *Planner, planStore *PlanStore) *ImporterService {
	return &ImporterService{
		planner:   planner,
		planStore: planStore,
		extractor: NewZipExtractor(),
		logger:    slog.Default().With("component", "ImporterService"),
	}
}

// CreateImportPlanFromFolder creates an import plan from a folder path
func (is *ImporterService) createImportPlanFromFolder(folderPath string) (*PlanResult, error) {
	// single-plan semantics: cleanup old plan workspace if present
	if old, err := is.planStore.Get(); err == nil && old != nil {
		err = os.RemoveAll(old.WorkspaceRoot)
		if err != nil {
			return nil, fmt.Errorf("cleanup old import workspace: %w", err)
		}
		is.planStore.Clear()
		is.logger.Info("Old import workspace cleaned up")
	}

	entries, err := FindMarkdownEntries(folderPath)
	if err != nil {
		return nil, err
	}

	opts := PlanOptions{
		SourceBasePath: folderPath,
	}

	plan, err := is.planner.CreatePlan(entries, opts)
	if err != nil {
		return nil, err
	}

	is.planStore.Set(&StoredPlan{
		Plan:          plan,
		PlanOptions:   opts,
		WorkspaceRoot: folderPath,
		CreatedAt:     time.Now(),
	})
	is.logger.Info("Import plan created", "entries", len(entries), "workspace", folderPath)
	return plan, nil
}

// GetCurrentPlan retrieves the currently stored import plan
func (is *ImporterService) GetCurrentPlan() (*PlanResult, error) {
	sp, err := is.planStore.Get()
	if err != nil {
		return nil, err
	}
	return sp.Plan, nil
}

// ClearCurrentPlan clears the currently stored import plan
func (is *ImporterService) ClearCurrentPlan() {
	if sp, err := is.planStore.Get(); err == nil && sp != nil {
		if err := os.RemoveAll(sp.WorkspaceRoot); err != nil {
			is.logger.Error("remove workspace failed", "error", err)
		}
	}
	is.planStore.Clear()
}

// ExecuteCurrentPlan executes the currently stored import plan
func (is *ImporterService) ExecuteCurrentPlan(userID string) (*ExecutionResult, error) {
	sp, err := is.planStore.Get()
	if err != nil {
		return nil, err
	}

	exec := NewExecutor(sp.Plan, &sp.PlanOptions, is.planner.wiki, is.planner.log)
	res, err := exec.Execute(userID)
	if err != nil {
		return nil, err
	}

	// After successful execution, clear the plan
	if sp, err := is.planStore.Get(); err == nil && sp != nil {
		if err := os.RemoveAll(sp.WorkspaceRoot); err != nil {
			is.logger.Error("remove workspace failed", "error", err)
		}
	}
	is.planStore.Clear()

	return res, nil
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
) (*PlanResult, error) {
	ws, err := is.extractZipReaderToTemp(r)
	if err != nil {
		return nil, fmt.Errorf("extract zip to temp: %w", err)
	}

	plan, err := is.createImportPlanFromFolder(ws.Root)
	if err != nil {
		if err := ws.Cleanup(); err != nil {
			is.logger.Error("cleanup failed", "error", err)
		}
		return nil, fmt.Errorf("create import plan from folder: %w", err)
	}
	return plan, nil
}

func (is *ImporterService) extractZipReaderToTemp(r io.Reader) (*ZipWorkspace, error) {
	tempDir := filepath.Join(os.TempDir(), "wiki-imports")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("create import temp dir: %w", err)
	}

	tmp, err := os.CreateTemp(tempDir, "import-*.zip")
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

	ws, err := is.extractor.ExtractToTemp(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("extract zip: %w", err)
	}
	return ws, nil
}
