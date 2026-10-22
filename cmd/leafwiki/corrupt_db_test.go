package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/test_utils"
)

func TestCorruptDatabaseMessage(t *testing.T) {
	dbPath := filepath.Join("data", "users.db")
	corrupt := &sqliteutil.CorruptDatabaseError{Path: dbPath, Err: errors.New("file is not a database")}

	stranded := dbPath + ".corrupt-20261017T143005Z"
	partial := &sqliteutil.CorruptDatabaseError{Path: dbPath, Err: errors.New("file is not a database"), Stranded: []string{stranded}}

	tests := []struct {
		name       string
		err        error
		wantOK     bool
		wantPart   []string
		wantAbsent []string
	}{
		{
			name:     "wrapped corrupt database names the path and the ways out",
			err:      fmt.Errorf("failed to init user store: %w", corrupt),
			wantOK:   true,
			wantPart: []string{dbPath, "left untouched", "restore-snapshot", "move", "aside"},
		},
		{
			name:       "a half-done quarantine names the moved files instead of claiming nothing changed",
			err:        fmt.Errorf("failed to init favorites store: %w", partial),
			wantOK:     true,
			wantPart:   []string{dbPath, stranded, "move them back", "restore-snapshot"},
			wantAbsent: []string{"left untouched"},
		},
		{name: "other errors are not described", err: errors.New("boom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := corruptDatabaseMessage(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("corruptDatabaseMessage ok = %v, want %v (msg %q)", ok, tt.wantOK, msg)
			}
			for _, part := range tt.wantPart {
				if !strings.Contains(msg, part) {
					t.Errorf("message %q does not contain %q", msg, part)
				}
			}
			for _, part := range tt.wantAbsent {
				if strings.Contains(msg, part) {
					t.Errorf("message %q must not contain %q", msg, part)
				}
			}
		})
	}
}

func TestRunResetAdminPasswordCommand_CorruptUsersDB_ExplainsTheWayOut(t *testing.T) {
	cfg := &serverConfig{}
	cfg.server.dataDir = t.TempDir()
	files := test_utils.WriteCorruptSQLiteDB(t, filepath.Join(cfg.server.dataDir, "users.db"))

	err := runResetAdminPasswordCommand(cfg)

	if err == nil || !strings.Contains(err.Error(), "restore-snapshot") {
		t.Fatalf("runResetAdminPasswordCommand error = %v, want the corrupt-database guidance", err)
	}
	test_utils.AssertFilesUnchanged(t, files)
}
