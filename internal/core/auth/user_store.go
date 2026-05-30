package auth

import (
	"database/sql"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

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
	// Create the users table if it doesn't exist
	_, err = f.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return err
	}
	return nil
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

func (f *UserStore) GetUserByID(id string) (*User, error) {
	// Ensure the database is connected
	err := f.Connect()
	if err != nil {
		return nil, err
	}

	// Query the user by ID
	row := f.db.QueryRow(`
		SELECT id, username, password, email, role
		FROM users
		WHERE id = ?;
	`, id)

	user := &User{}
	err = row.Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Role)
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
	row := f.db.QueryRow(`
		SELECT id, username, password, email, role
		FROM users
		WHERE username = ?;
	`, username)

	user := &User{}
	err = row.Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Role)
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
	row := f.db.QueryRow(`
		SELECT id, username, password, email, role
		FROM users
		WHERE email = ?;
	`, email)
	user := &User{}
	err = row.Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Role)
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
	_, err = f.GetUserByID(user.ID)
	if err != nil {
		if err == ErrUserNotFound {
			return ErrUserNotFound
		}
		return err
	}

	// Update the user in the database
	_, err = f.db.Exec(`
		UPDATE users
		SET username = ?, password = ?, email = ?, role = ?
		WHERE id = ?;
	`, user.Username, user.Password, user.Email, user.Role, user.ID)
	if err != nil {
		return f.mapConstraintViolationToError(err)
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
	row := f.db.QueryRow(`
		SELECT id, username, password, email, role
		FROM users
		WHERE role = 'admin'
		LIMIT 1;
	`)

	user := &User{}
	err = row.Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Role)
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
	rows, err := f.db.Query(`
		SELECT id, username, password, email, role
		FROM users;
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error("could not close rows", "error", err)
		}
	}()

	var users []*User
	for rows.Next() {
		user := &User{}
		err = rows.Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.Role)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
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
