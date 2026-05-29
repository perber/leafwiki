package health

import (
	"os"

	"github.com/perber/wiki/internal/search"
)

type checkResult struct {
	sqlite  string
	dataDir string
	search  string
}

type HealthUseCase struct {
	index      *search.SQLiteIndex
	status     *search.IndexingStatus
	storageDir string
}

func NewHealthUseCase(index *search.SQLiteIndex, status *search.IndexingStatus, storageDir string) *HealthUseCase {
	return &HealthUseCase{
		index:      index,
		status:     status,
		storageDir: storageDir,
	}
}

func (uc *HealthUseCase) Execute() (bool, map[string]string) {
	r := checkResult{}

	if err := uc.index.Ping(); err != nil {
		r.sqlite = "failed"
	} else {
		r.sqlite = "ok"
	}

	if info, err := os.Stat(uc.storageDir); err != nil || !info.IsDir() {
		r.dataDir = "failed"
	} else {
		r.dataDir = "ok"
	}

	switch {
	case uc.status.IsFailed():
		r.search = "failed"
	case uc.status.IsReady():
		r.search = "ok"
	default:
		r.search = "indexing"
	}

	checks := map[string]string{
		"sqlite":   r.sqlite,
		"data_dir": r.dataDir,
		"search":   r.search,
	}

	healthy := r.sqlite == "ok" && r.dataDir == "ok" && r.search != "failed"
	return healthy, checks
}
