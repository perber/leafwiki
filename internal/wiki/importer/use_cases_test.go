package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	coreshared "github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	coreimporter "github.com/perber/wiki/internal/importer"
)

// setupCreateImportPlanUseCase builds a real ImporterService (no mocks) with
// deliberately tiny zip-extraction limits so the size/ratio caps trip on
// small, fast test fixtures instead of the multi-hundred-MiB inputs
// production's real 50 MiB/1 GiB/100:1 defaults would require. The planner's
// wiki dependency is nil: every case here fails during extraction, before
// CreateImportPlanFromZipUpload ever reaches the planner.
func setupCreateImportPlanUseCase(t *testing.T, limits coreshared.ExtractionLimits) *CreateImportPlanUseCase {
	t.Helper()
	importerDir := filepath.Join(t.TempDir(), ".importer")
	planner := coreimporter.NewPlanner(nil, tree.NewSlugService(), "")
	store := coreimporter.NewPlanStore(filepath.Join(importerDir, "current-plan.json"))
	svc := coreimporter.NewImporterService(planner, store, filepath.Join(importerDir, "workspaces"), 0)
	svc.SetZipExtractionLimits(limits)
	return NewCreateImportPlanUseCase(svc)
}

func buildZipUpload(t *testing.T, entries map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		zw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := zw.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return &buf
}

// Regression tests for the zip-bomb unbounded-decompression DoS (CWE-409):
// CreateImportPlanUseCase.Execute must map each of the three extraction caps
// to its own distinct, localized error code instead of letting the plain
// sentinel error (or an unwrapped bare error) escape.

func TestCreateImportPlanUseCase_Execute_MapsEntryTooLargeError(t *testing.T) {
	uc := setupCreateImportPlanUseCase(t, coreshared.ExtractionLimits{
		MaxEntryBytes:   100,
		MaxTotalBytes:   1 << 30,
		MaxRatio:        1 << 30,
		RatioFloorBytes: 1 << 30,
	})
	upload := buildZipUpload(t, map[string]string{"big.txt": strings.Repeat("A", 200)})

	_, err := uc.Execute(context.Background(), CreateImportPlanInput{File: upload, TargetBasePath: ""})
	localized := assertLocalizedZipError(t, err, ErrCodeImporterZipEntryTooLarge)
	assertMessageDoesNotLeakInternalDetail(t, localized)
}

func TestCreateImportPlanUseCase_Execute_MapsCumulativeTooLargeError(t *testing.T) {
	uc := setupCreateImportPlanUseCase(t, coreshared.ExtractionLimits{
		MaxEntryBytes:   1 << 30,
		MaxTotalBytes:   100,
		MaxRatio:        1 << 30,
		RatioFloorBytes: 1 << 30,
	})
	upload := buildZipUpload(t, map[string]string{
		"a.txt": strings.Repeat("A", 60),
		"b.txt": strings.Repeat("B", 60),
		"c.txt": strings.Repeat("C", 60),
	})

	_, err := uc.Execute(context.Background(), CreateImportPlanInput{File: upload, TargetBasePath: ""})
	localized := assertLocalizedZipError(t, err, ErrCodeImporterZipExtractedTooLarge)
	assertMessageDoesNotLeakInternalDetail(t, localized)
}

func TestCreateImportPlanUseCase_Execute_MapsRatioTooHighError(t *testing.T) {
	uc := setupCreateImportPlanUseCase(t, coreshared.ExtractionLimits{
		MaxEntryBytes:   1 << 30,
		MaxTotalBytes:   1 << 30,
		MaxRatio:        3,
		RatioFloorBytes: 50,
	})
	upload := buildZipUpload(t, map[string]string{"bomb.txt": strings.Repeat("A", 5000)})

	_, err := uc.Execute(context.Background(), CreateImportPlanInput{File: upload, TargetBasePath: ""})
	localized := assertLocalizedZipError(t, err, ErrCodeImporterZipRatioTooHigh)
	assertMessageDoesNotLeakInternalDetail(t, localized)
}

func assertLocalizedZipError(t *testing.T, err error, wantCode string) *sharederrors.LocalizedError {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var localized *sharederrors.LocalizedError
	if !errors.As(err, &localized) {
		t.Fatalf("expected a *sharederrors.LocalizedError, got: %T (%v)", err, err)
	}
	if localized.Code != wantCode {
		t.Fatalf("expected code %q, got %q", wantCode, localized.Code)
	}
	return localized
}

// assertMessageDoesNotLeakInternalDetail guards against a regression where
// the user-facing Message/Template embedded the full internal wrapped error
// chain (e.g. "extract zip to temp: extract zip: write file: file too
// large: 200 bytes (max 100)") instead of a static, curated string — the
// detail belongs server-side only (see logRejectedZipExtraction), not in a
// response a user reads.
func assertMessageDoesNotLeakInternalDetail(t *testing.T, localized *sharederrors.LocalizedError) {
	t.Helper()
	for _, leaky := range []string{"extract zip", "write file", "bytes (max"} {
		if strings.Contains(localized.Message, leaky) || strings.Contains(localized.Template, leaky) {
			t.Errorf("user-facing message/template leaks internal detail (%q): message=%q template=%q", leaky, localized.Message, localized.Template)
		}
	}
}
