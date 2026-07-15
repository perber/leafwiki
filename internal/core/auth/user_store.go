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

	_ "modernc.org/sqlite"
)

const logCloseRowsFailed = "could not close rows"

type UserStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
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

	err := u.Connect()
	if err != nil {
		return nil, err
	}

	return u, u.ensureSchema()

}

func (f *UserStore) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.db != nil {
		return nil
	}
	db, err := sql.Open("sqlite", databasePath(f.storageDir, f.filename))
	if err != nil {
		return err
	}
	f.db = db
	return nil
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
			totp_last_reset_at TIMESTAMP NULL
		);
	`)
	if err != nil {
		return err
	}
	return f.ensureTOTPColumns()
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
		totp_secret_encrypted, totp_enabled, totp_recovery_codes_json, totp_enabled_at, totp_last_reset_at`

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanUser scans a row produced by a query selecting userColumns into a User.
func scanUser(row scanner) (*User, error) {
	user := &User{}
	var totpEnabledInt int
	var recoveryCodesJSON string
	var enabledAt, lastResetAt sql.NullString

	err := row.Scan(
		&user.ID, &user.Username, &user.Password, &user.Email, &user.Role,
		&user.TOTPSecretEncrypted, &totpEnabledInt, &recoveryCodesJSON, &enabledAt, &lastResetAt,
	)
	if err != nil {
		return nil, err
	}

	user.TOTPEnabled = totpEnabledInt != 0
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

// UpdateRecoveryCodeHashes replaces the stored recovery-code hashes for userID,
// e.g. to atomically remove a hash after it has been consumed at login.
func (f *UserStore) UpdateRecoveryCodeHashes(userID string, recoveryCodeHashes []string) error {
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
		SET totp_recovery_codes_json = ?
		WHERE id = ?;
	`, string(codesJSON), userID)
	return err
}
