package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
)

// corruptDatabaseMessage turns a *sqliteutil.CorruptDatabaseError anywhere in
// err's chain into an operator message with the path, the SQLite code and
// the ways out. LeafWiki never replaces such a database by itself, because
// an empty users.db would let startup create a fresh admin (ADR-0013).
func corruptDatabaseMessage(err error) (string, bool) {
	var corrupt *sqliteutil.CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		return "", false
	}
	state := "was left untouched"
	if len(corrupt.Stranded) > 0 {
		// A failed quarantine could not move everything back; say where the
		// bytes are instead of claiming nothing changed.
		state = fmt.Sprintf("could only partly be moved aside: %s now carry a .corrupt-* name, the rest is in place; move them back by hand before anything else",
			strings.Join(corrupt.Stranded, ", "))
	}
	return fmt.Sprintf("%s is unreadable (SQLite code %d) and %s. "+
		"Restore it from a snapshot with `leafwiki restore-snapshot <path-to-zip>` while the server is stopped, "+
		"or move the file and its -wal/-shm/-journal sidecars aside by hand and start again "+
		"(for users.db this starts with no users and creates the admin from the startup flags).",
		corrupt.Path, corrupt.SQLiteCode(), state), true
}
