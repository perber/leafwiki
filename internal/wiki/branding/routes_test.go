package branding

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	corebranding "github.com/perber/wiki/internal/branding"
)

func newTestRoutes(t *testing.T) (*Routes, *corebranding.BrandingConfigResponse) {
	t.Helper()

	svc, err := corebranding.NewBrandingService(t.TempDir())
	if err != nil {
		t.Fatalf("NewBrandingService() error: %v", err)
	}

	cfg, err := svc.GetBranding()
	if err != nil {
		t.Fatalf("GetBranding() error: %v", err)
	}

	return &Routes{
		brandingService: svc,
		log:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, cfg
}

func TestResolveBrandingAssetPath_RejectsAbsolutePath(t *testing.T) {
	routes, cfg := newTestRoutes(t)

	absolutePath := filepath.Join(t.TempDir(), "logo.png")
	_, status := routes.resolveBrandingAssetPath(absolutePath, cfg)
	if status != http.StatusForbidden {
		t.Fatalf("expected forbidden for absolute path, got %d", status)
	}
}

func TestResolveBrandingAssetPath_RejectsWindowsVolumePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("filepath volume paths are only recognized on Windows")
	}

	routes, cfg := newTestRoutes(t)

	_, status := routes.resolveBrandingAssetPath("C:logo.png", cfg)
	if status != http.StatusForbidden {
		t.Fatalf("expected forbidden for Windows volume path, got %d", status)
	}
}
