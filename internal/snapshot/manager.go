package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

const errFailedToListSnapshots = "Failed to list snapshots"

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

// TriggerSnapshot starts a snapshot job asynchronously.
// Returns ErrAlreadyRunning if a snapshot is already in progress.
func (m *Manager) TriggerSnapshot() error {
	if !m.status.TryStart() {
		return ErrAlreadyRunning
	}
	go func() {
		_ = m.runOnceLocked(context.Background())
	}()
	return nil
}

// RunOnce runs a snapshot synchronously, including retention pruning
// afterward, and updates status. Returns ErrAlreadyRunning if a snapshot is
// already in progress. Intended for the Scheduler, which serializes calls
// from its own goroutine loop.
func (m *Manager) RunOnce(ctx context.Context) error {
	if !m.status.TryStart() {
		return ErrAlreadyRunning
	}
	return m.runOnceLocked(ctx)
}

// runOnceLocked performs the snapshot + prune + status update. The caller
// must have already won the TryStart race. IsRunning is only released (via
// SetSuccess/SetError) once both the snapshot and pruning are done, so a
// concurrent RunOnce/TriggerSnapshot cannot start while pruning is still
// in flight. A panic anywhere in this chain (e.g. a corrupted-input panic
// inside zip/sqlite handling) is recovered here — not just logged by the
// Scheduler — so IsRunning is always reset and the feature never gets stuck.
func (m *Manager) runOnceLocked(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during snapshot: %v", r)
			m.status.SetError(err.Error())
		}
	}()

	if _, createErr := createSnapshot(ctx, m.cfg); createErr != nil {
		m.status.SetError(createErr.Error())
		return createErr
	}

	var pruneErrMsg string
	if pruneErr := m.pruneOldSnapshots(); pruneErr != nil {
		slog.Warn("snapshot retention pruning failed", "error", pruneErr)
		pruneErrMsg = pruneErr.Error()
	}
	m.status.SetSuccess(time.Now().UTC(), pruneErrMsg)
	return nil
}

// pruneOldSnapshots deletes the oldest snapshots beyond cfg.RetentionCount.
// A RetentionCount <= 0 means unlimited (no pruning).
func (m *Manager) pruneOldSnapshots() error {
	if m.cfg.RetentionCount <= 0 {
		return nil
	}
	entries, err := m.List()
	if err != nil {
		return err
	}
	if len(entries) <= m.cfg.RetentionCount {
		return nil
	}
	for _, entry := range entries[m.cfg.RetentionCount:] {
		if err := m.Delete(entry.ID); err != nil {
			return err
		}
		slog.Info("snapshot pruned", "id", entry.ID, "retention_count", m.cfg.RetentionCount)
	}
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
			errFailedToListSnapshots,
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
				errFailedToListSnapshots,
				"failed to read snapshot metadata %s",
				err,
				de.Name(),
			)
		}
		var entry SnapshotEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, sharederrors.NewLocalizedError(
				"snapshot_list_failed",
				errFailedToListSnapshots,
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

// SnapshotZipPath validates id and returns the absolute path to its ZIP
// file, for the HTTP download handler to open and stream. This is the
// security boundary against path traversal (via validateSnapshotID).
func (m *Manager) SnapshotZipPath(id string) (string, error) {
	if err := validateSnapshotID(id); err != nil {
		return "", err
	}
	path := filepath.Join(m.cfg.BackupsDir, id+".zip")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", sharederrors.NewLocalizedError(
				"snapshot_not_found",
				"Snapshot not found",
				"snapshot %s not found",
				nil,
				id,
			)
		}
		return "", sharederrors.NewLocalizedError(
			"snapshot_internal_error",
			"Failed to access snapshot",
			"failed to access snapshot %s",
			err,
			id,
		)
	}
	return path, nil
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
