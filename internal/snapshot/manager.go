package snapshot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ErrAlreadyRunning is returned by TriggerSnapshot when a snapshot is
// already in progress. It carries no dynamic arguments, so it is a fixed
// package-level LocalizedError, matching wikiresync.ErrResyncAlreadyRunning.
var ErrAlreadyRunning = sharederrors.NewLocalizedError(
	"snapshot_already_running",
	"A snapshot is already in progress",
	"a snapshot is already in progress",
	nil,
)

type SnapshotEntry struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	SizeBytes int64     `json:"sizeBytes"`
}

type Manager struct {
	cfg    Config
	status Status
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

// TriggerSnapshot starts a backup job asynchronously.
// Returns ErrAlreadyRunning if a backup is already in progress.
func (m *Manager) TriggerSnapshot() error {
	if !m.status.TryStart() {
		return ErrAlreadyRunning
	}

	go func() {
		_, err := createSnapshot(context.Background(), m.cfg)
		if err != nil {
			m.status.SetError(err.Error())
			return
		}
		m.status.SetSuccess(time.Now().UTC())
	}()
	return nil
}

// Status returns the current status (thread-safe).
func (m *Manager) Status() StatusSnapshot {
	return m.status.Snapshot()
}

// List returns all finished snapshots sorted by date (newest first).
func (m *Manager) List() ([]SnapshotEntry, error) {
	dirEntries, err := os.ReadDir(m.cfg.BackupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotEntry{}, nil
		}
		return nil, sharederrors.NewLocalizedError(
			"snapshot_list_failed",
			"Failed to list snapshots",
			"failed to list snapshots",
			err,
		)
	}

	entries := make([]SnapshotEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.cfg.BackupsDir, de.Name()))
		if err != nil {
			return nil, sharederrors.NewLocalizedError(
				"snapshot_list_failed",
				"Failed to list snapshots",
				"failed to read snapshot metadata %s",
				err,
				de.Name(),
			)
		}
		var entry SnapshotEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, sharederrors.NewLocalizedError(
				"snapshot_list_failed",
				"Failed to list snapshots",
				"failed to parse snapshot metadata %s",
				err,
				de.Name(),
			)
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return entries, nil
}

// Delete removes the ZIP + sidecar JSON for a given snapshot ID.
func (m *Manager) Delete(id string) error {
	if err := validateSnapshotID(id); err != nil {
		return err
	}

	zipPath := filepath.Join(m.cfg.BackupsDir, id+".zip")
	jsonPath := filepath.Join(m.cfg.BackupsDir, id+".json")

	zipErr := os.Remove(zipPath)
	jsonErr := os.Remove(jsonPath)

	if os.IsNotExist(zipErr) && os.IsNotExist(jsonErr) {
		return sharederrors.NewLocalizedError(
			"snapshot_not_found",
			"Snapshot not found",
			"snapshot %s not found",
			nil,
			id,
		)
	}
	if zipErr != nil && !os.IsNotExist(zipErr) {
		return sharederrors.NewLocalizedError(
			"snapshot_delete_failed",
			"Failed to delete snapshot",
			"failed to delete snapshot %s",
			zipErr,
			id,
		)
	}
	if jsonErr != nil && !os.IsNotExist(jsonErr) {
		return sharederrors.NewLocalizedError(
			"snapshot_delete_failed",
			"Failed to delete snapshot",
			"failed to delete snapshot %s",
			jsonErr,
			id,
		)
	}
	return nil
}

// BackupsDir returns the configured folder (for the download handler).
func (m *Manager) BackupsDir() string {
	return m.cfg.BackupsDir
}

func validateSnapshotID(id string) error {
	if !strings.HasPrefix(id, "snapshot-") || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return sharederrors.NewLocalizedError(
			"snapshot_invalid_id",
			"Invalid snapshot id",
			"invalid snapshot id %s",
			nil,
			id,
		)
	}
	return nil
}
