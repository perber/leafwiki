package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite"
)

const logCloseRowsFailed = "could not close rows"

type UserStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
	// suspended is set by suspend() and makes Connect() refuse to lazily
	// reopen db — see suspend's doc comment.
	suspended bool
}

func databasePath(storageDir string, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

func NewUserStore(storageDir string) (*UserStore, error) {
	u := &UserStore{
		storageDir: storageDir,
		filename:   "users.db",
	}

	// ensureSchema calls Connect() itself (idempotently), so it alone is
	// enough to bring the store to a working state.
	err := sqliteutil.RetryOnCorruption(databasePath(u.storageDir, u.filename), func() error {
		if err := u.ensureSchema(); err != nil {
			if u.db != nil {
				_ = u.db.Close()
				u.db = nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (f *UserStore) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.suspended {
		return errUserStoreUnavailable()
	}
	if f.db != nil {
		return nil
	}
	// busy_timeout makes concurrent writers (e.g. two requests racing to
	// consume the same TOTP recovery code via ConsumeRecoveryCodeHash) block
	// and retry internally for up to 5s instead of failing immediately with
	// SQLITE_BUSY, which ConsumeRecoveryCodeHash's caller does not retry on.
	// journal_mode(WAL) is load-test-verified: GetUserByID runs on every
	// authenticated request (RequireAuth -> ValidateToken), and under the
	// previous rollback-journal mode a handful of concurrent writes to this
	// store (role/profile updates) measurably stalled reads system-wide
	// (p95 +120-180ms at just 3 concurrent writers) via SQLite's file-level
	// commit lock — WAL lets readers proceed without blocking on a writer's
	// commit. Because this store (unlike search/tags/links) is
	// non-derived source-of-truth data, enabling WAL here also required
	// restore/swap.go to explicitly clean up -wal/-shm sidecars before a
	// live or offline restore swap (see removeStaleWALSidecars) — read that
	// comment before touching this pragma.
	db, err := sql.Open("sqlite", databasePath(f.storageDir, f.filename)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return err
	}
	f.db = db
	return nil
}

// suspend closes db (if open) and marks the store so Connect() refuses to
// lazily reopen it — unlike a plain Close(), whose reconnect-on-next-query
// behavior NewUserStore's corruption-retry path deliberately relies on.
// Used by AuthService.PauseUserStoreForSwap before a live restore renames
// users.db out from under this store: on Windows, an open file handle (or
// one silently reopened by a query landing mid-swap) blocks the rename with
// a sharing violation, which POSIX doesn't have. suspend is permanent for
// this *UserStore instance — restore always continues with a brand new one
// afterward (see AuthService.ReplaceUserStore), so there's no un-suspend.
func (f *UserStore) suspend() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suspended = true
	if f.db == nil {
		return nil
	}
	err := f.db.Close()
	f.db = nil
	return err
}

func (f *UserStore) ensureSchema() error {
	err := f.Connect()
	if err != nil {
		return err
	}
	// Create the users table if it doesn't exist. Fresh installs get the full
	// TOTP schema immediately; existing users.db files are migrated additively
	// by ensureTOTPColumns below.
	_, err = f.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			totp_secret_encrypted TEXT NOT NULL DEFAULT '',
			totp_enabled INTEGER NOT NULL DEFAULT 0,
			totp_recovery_codes_json TEXT NOT NULL DEFAULT '[]',
			totp_enabled_at TIMESTAMP NULL,
			totp_last_reset_at TIMESTAMP NULL,
			must_set_password INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return err
	}
	if err := f.ensureTOTPColumns(); err != nil {
		return err
	}
	return f.ensureMustSetPasswordColumn()
}

// ensureMustSetPasswordColumn additively migrates a pre-invite users.db by
// adding must_set_password if missing. Same idempotent pattern as
// ensureTOTPColumns.
func (f *UserStore) ensureMustSetPasswordColumn() error {
	existing, err := f.existingColumns()
	if err != nil {
		return err
	}
	if existing["must_set_password"] {
		return nil
	}
	if _, err := f.db.Exec(`ALTER TABLE users ADD COLUMN must_set_password INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("failed to add column must_set_password to users table: %w", err)
	}
	return nil
}

// ensureTOTPColumns additively migrates a pre-TOTP users.db by adding any
// missing totp_* columns. Existing rows and every other column are left
// untouched; new columns get the same safe defaults as a fresh install
// (TOTP disabled, no secret, no recovery codes). Safe to run on every
// startup — columns already present are skipped, so it is idempotent.
func (f *UserStore) ensureTOTPColumns() error {
	existing, err := f.existingColumns()
	if err != nil {
		return err
	}

	migrations := []struct {
		column string
		ddl    string
	}{
		{"totp_secret_encrypted", "ALTER TABLE users ADD COLUMN totp_secret_encrypted TEXT NOT NULL DEFAULT ''"},
		{"totp_enabled", "ALTER TABLE users ADD COLUMN totp_enabled INTEGER NOT NULL DEFAULT 0"},
		{"totp_recovery_codes_json", "ALTER TABLE users ADD COLUMN totp_recovery_codes_json TEXT NOT NULL DEFAULT '[]'"},
		{"totp_enabled_at", "ALTER TABLE users ADD COLUMN totp_enabled_at TIMESTAMP NULL"},
		{"totp_last_reset_at", "ALTER TABLE users ADD COLUMN totp_last_reset_at TIMESTAMP NULL"},
	}

	for _, m := range migrations {
		if existing[m.column] {
			continue
		}
		if _, err := f.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("failed to add column %s to users table: %w", m.column, err)
		}
	}
	return nil
}

func (f *UserStore) existingColumns() (map[string]bool, error) {
	rows, err := f.db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	cols := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func (f *UserStore) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.db != nil {
		err := f.db.Close()
		if err != nil {
			return err
		}
		f.db = nil
	}
	return nil
}

func (f *UserStore) CreateUser(user *User) error {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return err
	}
	// Insert the user into the database
	_, err = f.db.Exec(`
		INSERT INTO users (id, username, password, email, role)
		VALUES (?, ?, ?, ?, ?);
	`, user.ID, user.Username, user.Password, user.Email, user.Role)
	if err != nil {
		return f.mapConstraintViolationToError(err)
	}
	return nil
}

const userColumns = `id, username, password, email, role,
		totp_secret_encrypted, totp_enabled, totp_recovery_codes_json, totp_enabled_at, totp_last_reset_at,
		must_set_password`

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanUser scans a row produced by a query selecting userColumns into a User.
func scanUser(row scanner) (*User, error) {
	user := &User{}
	var totpEnabledInt, mustSetPasswordInt int
	var recoveryCodesJSON string
	var enabledAt, lastResetAt sql.NullString

	err := row.Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role,
		&user.TOTPSecretEncrypted, &totpEnabledInt, &recoveryCodesJSON, &enabledAt, &lastResetAt,
		&mustSetPasswordInt,
	)
	if err != nil {
		return nil, err
	}

	user.TOTPEnabled = totpEnabledInt != 0
	user.MustSetPassword = mustSetPasswordInt != 0
	if recoveryCodesJSON != "" {
		if err := json.Unmarshal([]byte(recoveryCodesJSON), &user.TOTPRecoveryCodeHashes); err != nil {
			return nil, fmt.Errorf("failed to parse stored recovery codes for user %s: %w", user.ID, err)
		}
	}
	if enabledAt.Valid {
		if t, err := time.Parse(time.RFC3339, enabledAt.String); err == nil {
			user.TOTPEnabledAt = &t
		}
	}
	if lastResetAt.Valid {
		if t, err := time.Parse(time.RFC3339, lastResetAt.String); err == nil {
			user.TOTPLastResetAt = &t
		}
	}
	return user, nil
}

func (f *UserStore) GetUserByID(id string) (*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}

	// Query the user by ID
	row := f.db.QueryRow(`SELECT `+userColumns+`
		FROM users
		WHERE id = ?;
	`, id)

	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (f *UserStore) GetUserByUsername(username string) (*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}
	// Query the user by username
	row := f.db.QueryRow(`SELECT `+userColumns+`
		FROM users
		WHERE username = ?;
	`, username)

	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (f *UserStore) GetUserByEmail(email string) (*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}
	// Query the user by email
	row := f.db.QueryRow(`SELECT `+userColumns+`
		FROM users
		WHERE email = ?;
	`, email)

	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (f *UserStore) UpdateUser(user *User) error {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return err
	}

	// Check if a user with the given ID exists
	existingUser, err := f.GetUserByID(user.ID)
	if err != nil {
		if err == ErrUserNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Update the user in the database
	result, err := f.db.Exec(`
		UPDATE users
		SET username = ?, password = ?, email = ?, role = ?
		WHERE id = ?
		  AND NOT (
			role = ?
			AND ? != ?
			AND (SELECT COUNT(*) FROM users WHERE role = ?) <= 1
		  );
	`, user.Username, user.Password, user.Email, user.Role, user.ID, RoleAdmin, user.Role, RoleAdmin, RoleAdmin)
	if err != nil {
		return f.mapConstraintViolationToError(err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 && existingUser.Role == RoleAdmin && user.Role != RoleAdmin {
		return ErrLastAdminCannotBeDemoted
	}
	return nil
}

func (f *UserStore) DeleteUser(id string) error {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return err
	}

	// Check if a user with the given ID exists
	_, err = f.GetUserByID(id)
	if err != nil {
		if err == ErrUserNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Delete the user from the database
	_, err = f.db.Exec(`
		DELETE FROM users
		WHERE id = ?;
	`, id)
	if err != nil {
		return err
	}
	return nil
}

func (f *UserStore) GetAdminUser() (*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}
	// Query the admin user
	row := f.db.QueryRow(`SELECT ` + userColumns + `
		FROM users
		WHERE role = 'admin'
		LIMIT 1;
	`)

	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (f *UserStore) GetAllUsers() ([]*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}
	// Query all users
	rows, err := f.db.Query(`SELECT ` + userColumns + `
		FROM users;
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (f *UserStore) CountAdminUsers() (int, error) {
	err := f.Connect()
	if err != nil {
		return 0, err
	}
	row := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin';`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountEditorUsers counts users with role admin or editor — the same
// "editor" definition used across the plan-tier pricing (viewers are always
// unlimited, admin+editor together count against the plan's editor limit).
func (f *UserStore) CountEditorUsers() (int, error) {
	err := f.Connect()
	if err != nil {
		return 0, err
	}
	row := f.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role IN ('admin', 'editor');`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (f *UserStore) GetUserCount() (int, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return 0, err
	}
	// Query the user count
	row := f.db.QueryRow(`
		SELECT COUNT(*)
		FROM users;
	`)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (f *UserStore) mapConstraintViolationToError(err error) error {
	// Check if the error is a constraint violation

	if err, ok := err.(interface{ Error() string }); ok {
		msg := err.Error()
		// Check for unique constraint violation
		if strings.Contains(msg, "UNIQUE constraint failed: users.username") {
			return ErrUserAlreadyExists
		}

		if strings.Contains(msg, "UNIQUE constraint failed: users.email") {
			return ErrUserAlreadyExists
		}
	}
	return err
}

func (f *UserStore) UpdatePassword(userID string, newPassword string) error {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return err
	}

	// Check if a user with the given ID exists
	_, err = f.GetUserByID(userID)
	if err != nil {
		if err == ErrUserNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Update the user's password in the database
	_, err = f.db.Exec(`
		UPDATE users
		SET password = ?
		WHERE id = ?;
	`, newPassword, userID)
	if err != nil {
		return err
	}
	return nil
}

// SetPendingTOTPSecret stores a freshly generated, not-yet-confirmed encrypted
// TOTP secret for userID. TOTP remains disabled until EnableTOTP confirms a
// valid code against this secret.
func (f *UserStore) SetPendingTOTPSecret(userID, encryptedSecret string) error {
	err := f.Connect()
	if err != nil {
		return err
	}

	if _, err := f.GetUserByID(userID); err != nil {
		return err
	}

	_, err = f.db.Exec(`
		UPDATE users
		SET totp_secret_encrypted = ?
		WHERE id = ?;
	`, encryptedSecret, userID)
	return err
}

// EnableTOTP marks TOTP enabled for userID with the confirmed encrypted secret
// and the hashed recovery codes generated alongside it.
func (f *UserStore) EnableTOTP(userID, encryptedSecret string, recoveryCodeHashes []string) error {
	err := f.Connect()
	if err != nil {
		return err
	}

	if _, err := f.GetUserByID(userID); err != nil {
		return err
	}

	codesJSON, err := json.Marshal(recoveryCodeHashes)
	if err != nil {
		return fmt.Errorf("failed to encode recovery codes for user %s: %w", userID, err)
	}

	_, err = f.db.Exec(`
		UPDATE users
		SET totp_secret_encrypted = ?, totp_enabled = 1, totp_recovery_codes_json = ?, totp_enabled_at = ?
		WHERE id = ?;
	`, encryptedSecret, string(codesJSON), time.Now().UTC().Format(time.RFC3339), userID)
	return err
}

// DisableTOTP clears TOTP secret, enabled flag, and recovery codes for userID.
func (f *UserStore) DisableTOTP(userID string) error {
	err := f.Connect()
	if err != nil {
		return err
	}

	if _, err := f.GetUserByID(userID); err != nil {
		return err
	}

	_, err = f.db.Exec(`
		UPDATE users
		SET totp_secret_encrypted = '', totp_enabled = 0, totp_recovery_codes_json = '[]', totp_last_reset_at = ?
		WHERE id = ?;
	`, time.Now().UTC().Format(time.RFC3339), userID)
	return err
}

// ConsumeRecoveryCodeHash atomically replaces oldHashes with newHashes for
// userID via compare-and-swap on the stored JSON column: the write only takes
// effect if the row's current totp_recovery_codes_json still serializes to
// exactly oldHashes. This guards against two concurrent requests both
// consuming the same recovery code — only the first compare-and-swap can
// match the row's current state; the loser gets swapped=false and must
// re-read and retry against the now-current hashes (see
// AuthService.verifyTOTPOrRecoveryCode).
//
// json.Marshal of a []string is deterministic, so re-encoding the
// previously-read oldHashes here reproduces exactly what was last written,
// with no need to carry the raw JSON string alongside the parsed slice.
func (f *UserStore) ConsumeRecoveryCodeHash(userID string, oldHashes, newHashes []string) (swapped bool, err error) {
	if err := f.Connect(); err != nil {
		return false, err
	}

	oldJSON, err := json.Marshal(oldHashes)
	if err != nil {
		return false, fmt.Errorf("failed to encode recovery codes for user %s: %w", userID, err)
	}
	newJSON, err := json.Marshal(newHashes)
	if err != nil {
		return false, fmt.Errorf("failed to encode recovery codes for user %s: %w", userID, err)
	}

	result, err := f.db.Exec(`
		UPDATE users
		SET totp_recovery_codes_json = ?
		WHERE id = ? AND totp_recovery_codes_json = ?;
	`, string(newJSON), userID, string(oldJSON))
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

// SetMustSetPassword flips the must_set_password flag for userID — set to
// true by InviteUser at creation, cleared by CompleteInvite once the invite
// is accepted.
func (f *UserStore) SetMustSetPassword(userID string, value bool) error {
	if err := f.Connect(); err != nil {
		return err
	}
	if _, err := f.GetUserByID(userID); err != nil {
		return err
	}

	v := 0
	if value {
		v = 1
	}
	_, err := f.db.Exec(`
		UPDATE users
		SET must_set_password = ?
		WHERE id = ?;
	`, v, userID)
	return err
}
