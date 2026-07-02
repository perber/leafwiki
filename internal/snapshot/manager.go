package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrAlreadyRunning = errors.New("snapshot already in progress")
	ErrNotFound       = errors.New("snapshot not found")
	ErrInvalidID      = errors.New("invalid snapshot id")
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
		return nil, fmt.Errorf("failed to read backups directory: %w", err)
	}

	entries := make([]SnapshotEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.cfg.BackupsDir, de.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", de.Name(), err)
		}
		var entry SnapshotEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", de.Name(), err)
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
		return ErrNotFound
	}
	if zipErr != nil && !os.IsNotExist(zipErr) {
		return fmt.Errorf("failed to delete %s: %w", zipPath, zipErr)
	}
	if jsonErr != nil && !os.IsNotExist(jsonErr) {
		return fmt.Errorf("failed to delete %s: %w", jsonPath, jsonErr)
	}
	return nil
}

// BackupsDir returns the configured folder (for the download handler).
func (m *Manager) BackupsDir() string {
	return m.cfg.BackupsDir
}

func validateSnapshotID(id string) error {
	if !strings.HasPrefix(id, "snapshot-") || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return ErrInvalidID
	}
	return nil
}
