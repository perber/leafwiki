package restore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/core/auth"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
	_ "modernc.org/sqlite" // Import SQLite driver
)

// fixtureSnapshot builds a real snapshot ZIP (root/assets/branding/
// branding.json/schema.json/users.db) from a fresh source layout — a
// separate temp dir from whatever "live" dataDir a test then restores into —
// and returns the returned snapshot.Manager (for SnapshotZipPath / List) and
// the built snapshot's id.
func fixtureSnapshot(t *testing.T, wikiVersion string) (*snapshotSvc.Manager, string) {
	t.Helper()
	return fixtureSnapshotWithBranding(t, wikiVersion, `{"siteName":"Snapshot Site"}`)
}

// fixtureSnapshotWithBranding is fixtureSnapshot with control over the exact
// branding.json content — used to build a snapshot whose branding.json is
// intentionally invalid, to exercise BrandingService.Reload failing after the
// file swap has already succeeded.
func fixtureSnapshotWithBranding(t *testing.T, wikiVersion, brandingJSON string) (*snapshotSvc.Manager, string) {
	t.Helper()
	src := t.TempDir()

	rootDir := filepath.Join(src, "root")
	assetsDir := filepath.Join(src, "assets")
	brandingDir := filepath.Join(src, "branding")

	test_utils.WriteFile(t, rootDir, "welcome.md", "# Snapshot content\n")
	test_utils.WriteFile(t, assetsDir, "logo.png", "fake-asset-bytes")
	test_utils.WriteFile(t, brandingDir, "logo.png", "fake-logo-bytes")
	brandingConfigFile := test_utils.WriteFile(t, src, "branding.json", brandingJSON)
	schemaFile := test_utils.WriteFile(t, src, "schema.json", `{"version":5}`)

	createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")
	usersDBPath := filepath.Join(src, "users.db")

	m := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:         filepath.Join(src, "backups"),
		RootDir:            rootDir,
		AssetsDir:          assetsDir,
		BrandingDir:        brandingDir,
		BrandingConfigFile: brandingConfigFile,
		SchemaFile:         schemaFile,
		UsersDBPath:        usersDBPath,
		WikiVersion:        wikiVersion,
	})
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := m.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	return m, entries[0].ID
}

// buildFixtureSnapshot is a convenience wrapper around fixtureSnapshot for
// tests that only need the resulting ZIP path.
func buildFixtureSnapshot(t *testing.T, wikiVersion string) string {
	t.Helper()
	m, id := fixtureSnapshot(t, wikiVersion)
	zipPath, err := m.SnapshotZipPath(id)
	if err != nil {
		t.Fatalf("SnapshotZipPath failed: %v", err)
	}
	return zipPath
}

// createRealUsersDB creates dataDir/users.db via the real auth.UserStore
// schema (not a hand-rolled minimal one) and seeds a single user, so that
// tests exercising AuthService.ReplaceUserStore/Login against the result see
// a genuinely valid users.db rather than one missing columns the real schema
// expects.
func createRealUsersDB(t *testing.T, dataDir, username, email, password string) {
	t.Helper()
	store, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := auth.NewUserService(store).CreateUser(username, email, password, "admin"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
}

func createTestUsersDB(t *testing.T, path, email string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open users db: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)

	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (email) VALUES (?)", email); err != nil {
		t.Fatalf("failed to seed users table: %v", err)
	}
}
