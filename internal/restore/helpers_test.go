package restore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/favorites"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/usersettings"
	_ "modernc.org/sqlite" // Import SQLite driver
)

// fixtureSnapshot builds a real snapshot ZIP (root/assets/branding/avatars/
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
	avatarsDir := filepath.Join(src, "avatars")

	test_utils.WriteFile(t, rootDir, "welcome.md", "# Snapshot content\n")
	test_utils.WriteFile(t, assetsDir, "logo.png", "fake-asset-bytes")
	test_utils.WriteFile(t, brandingDir, "logo.png", "fake-logo-bytes")
	test_utils.WriteFile(t, avatarsDir, "snapshot-user.png", "fake-avatar-bytes")
	brandingConfigFile := test_utils.WriteFile(t, src, "branding.json", brandingJSON)
	schemaFile := test_utils.WriteFile(t, src, "schema.json", `{"version":5}`)

	createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")
	usersDBPath := filepath.Join(src, "users.db")

	m := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:         filepath.Join(src, "backups"),
		RootDir:            rootDir,
		AssetsDir:          assetsDir,
		BrandingDir:        brandingDir,
		AvatarsDir:         avatarsDir,
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
// expects. Returns the created user's ID (e.g. for createRealAPIKeysDB, which
// needs an existing owner to mint a key against).
func createRealUsersDB(t *testing.T, dataDir, username, email, password string) string {
	t.Helper()
	store, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user, err := auth.NewUserService(store).CreateUser(username, email, password, "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user.ID
}

// createRealAPIKeysDB creates dataDir/api_keys.db via the real
// auth.APIKeyStore/APIKeyService and mints a single key owned by ownerUserID
// (must already exist in dataDir/users.db — CreateAPIKey looks the owner up),
// returning the plaintext bearer token. NewAPIKeyService requires a real
// *AuthService (owner lookups always go through AuthService.UserService() —
// see apikey_service.go) — a throwaway sessions.db in its own tempdir is
// built alongside it purely to satisfy the constructor; this helper never
// logs in or otherwise exercises sessions.
func createRealAPIKeysDB(t *testing.T, dataDir, ownerUserID, keyName string) string {
	t.Helper()
	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(userStore.Close, t)

	sessionStore, err := auth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(sessionStore.Close, t)
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)

	apiKeyStore, err := auth.NewAPIKeyStore(dataDir)
	if err != nil {
		t.Fatalf("NewAPIKeyStore failed: %v", err)
	}
	apiKeyService := auth.NewAPIKeyService(apiKeyStore, authService)
	defer test_utils.WrapCloseWithErrorCheck(apiKeyService.Close, t)

	_, token, err := apiKeyService.CreateAPIKey(auth.CreateAPIKeyParams{
		Name:      keyName,
		UserID:    ownerUserID,
		CreatedBy: ownerUserID,
	})
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	return token
}

// createRealFavoritesDB creates dataDir/favorites.db via the real
// favorites.FavoritesStore and adds a single favorite for userID/pageID.
func createRealFavoritesDB(t *testing.T, dataDir, userID, pageID string) {
	t.Helper()
	store, err := favorites.NewFavoritesStore(dataDir, nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if err := store.Add(userID, pageID); err != nil {
		t.Fatalf("Add favorite failed: %v", err)
	}
}

// createRealUserSettingsDB creates dataDir/usersettings.db via the real
// usersettings.UserSettingsStore and saves language for userID.
func createRealUserSettingsDB(t *testing.T, dataDir, userID, language string) {
	t.Helper()
	store, err := usersettings.NewUserSettingsStore(dataDir, nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if err := store.Upsert(&usersettings.UserSettings{UserID: userID, Language: language, AutoSave: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Upsert user settings failed: %v", err)
	}
}

// createTestUsersDB creates a minimal users.db with just usersRequiredColumns
// (swap.go) — enough to pass sanityCheckSQLiteDB's schema probe wherever a
// test needs "a users.db that validates" without the real auth.UserStore
// schema/migrations (see createRealUsersDB for that).
func createTestUsersDB(t *testing.T, path, email string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open users db: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)

	if _, err := db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		email TEXT NOT NULL UNIQUE,
		role TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (id, username, password, email, role) VALUES (?, ?, ?, ?, ?)",
		"test-user-id", "test-user", "test-password-hash", email, "admin"); err != nil {
		t.Fatalf("failed to seed users table: %v", err)
	}
}
