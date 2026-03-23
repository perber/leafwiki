package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
)

type FSStore struct {
	storageDir string
}

func NewFSStore(storageDir string) *FSStore {
	return &FSStore{storageDir: storageDir}
}

func (s *FSStore) SaveContentBlob(content []byte) (string, error) {
	hash := sha256HexBytes(content)
	dst := s.contentBlobPath(hash)

	if fileExists(dst) {
		return hash, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("ensure content blob dir: %w", err)
	}
	if err := shared.WriteFileAtomic(dst, content, 0o644); err != nil {
		if fileExists(dst) {
			return hash, nil
		}
		return "", fmt.Errorf("write content blob: %w", err)
	}

	return hash, nil
}

func (s *FSStore) SaveAssetBlobFromPath(srcPath string) (string, int64, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", 0, fmt.Errorf("open live asset %s: %w", srcPath, err)
	}
	defer func() { _ = src.Close() }()

	tmpDir := filepath.Join(s.baseDir(), "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("ensure tmp dir: %w", err)
	}

	tmp, err := os.CreateTemp(tmpDir, "asset-blob-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp asset blob: %w", err)
	}

	cleanupTmp := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), src)
	if err != nil {
		cleanupTmp()
		return "", 0, fmt.Errorf("copy asset to temp blob: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	dst := s.assetBlobPath(hash)

	if err := tmp.Chmod(0o644); err != nil {
		cleanupTmp()
		return "", 0, fmt.Errorf("chmod temp asset blob: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", 0, fmt.Errorf("close temp asset blob: %w", err)
	}

	if fileExists(dst) {
		_ = os.Remove(tmp.Name())
		return hash, written, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		_ = os.Remove(tmp.Name())
		return "", 0, fmt.Errorf("ensure asset blob dir: %w", err)
	}

	if err := os.Rename(tmp.Name(), dst); err != nil {
		if fileExists(dst) {
			_ = os.Remove(tmp.Name())
			return hash, written, nil
		}
		_ = os.Remove(tmp.Name())
		return "", 0, fmt.Errorf("move asset blob into place: %w", err)
	}

	return hash, written, nil
}

func (s *FSStore) SaveAssetManifest(items []AssetRef) (string, error) {
	canonical := cloneAndSortAssetRefs(items)

	raw, err := json.Marshal(assetManifest{Items: canonical})
	if err != nil {
		return "", fmt.Errorf("marshal asset manifest: %w", err)
	}

	hash := sha256HexBytes(raw)
	dst := s.assetManifestPath(hash)

	if fileExists(dst) {
		return hash, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("ensure manifest dir: %w", err)
	}
	if err := shared.WriteFileAtomic(dst, raw, 0o644); err != nil {
		if fileExists(dst) {
			return hash, nil
		}
		return "", fmt.Errorf("write asset manifest: %w", err)
	}

	return hash, nil
}

func (s *FSStore) SaveRevision(rev *Revision) error {
	if rev == nil {
		return fmt.Errorf("revision is required")
	}
	if strings.TrimSpace(rev.ID) == "" {
		return fmt.Errorf("revision id is required")
	}
	if strings.TrimSpace(rev.PageID) == "" {
		return fmt.Errorf("page id is required")
	}
	if rev.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}

	dst := s.revisionFilePath(rev.PageID, rev.ID, rev.CreatedAt)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ensure revision dir: %w", err)
	}
	if err := writeJSONAtomic(dst, rev); err != nil {
		return fmt.Errorf("write revision: %w", err)
	}
	return nil
}

func (s *FSStore) ListRevisions(pageID string) ([]*Revision, error) {
	dir := s.revisionsPageDir(pageID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Revision{}, nil
		}
		return nil, fmt.Errorf("read revisions dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	revisions := make([]*Revision, 0, len(names))
	for _, name := range names {
		var rev Revision
		if err := readJSON(filepath.Join(dir, name), &rev); err != nil {
			return nil, fmt.Errorf("read revision %s: %w", name, err)
		}
		revisions = append(revisions, &rev)
	}

	return revisions, nil
}

func (s *FSStore) GetLatestRevision(pageID string) (*Revision, error) {
	revisions, err := s.ListRevisions(pageID)
	if err != nil {
		return nil, err
	}
	if len(revisions) == 0 {
		return nil, nil
	}
	return revisions[0], nil
}

func (s *FSStore) GetRevision(pageID, revisionID string) (*Revision, error) {
	revisions, err := s.ListRevisions(pageID)
	if err != nil {
		return nil, err
	}
	for _, rev := range revisions {
		if rev.ID == revisionID {
			return rev, nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *FSStore) SaveTrashEntry(entry *TrashEntry) error {
	if entry == nil {
		return fmt.Errorf("trash entry is required")
	}
	if strings.TrimSpace(entry.PageID) == "" {
		return fmt.Errorf("page id is required")
	}

	dst := s.trashEntryPath(entry.PageID)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("ensure trash dir: %w", err)
	}
	if err := writeJSONAtomic(dst, entry); err != nil {
		return fmt.Errorf("write trash entry: %w", err)
	}
	return nil
}

func (s *FSStore) GetTrashEntry(pageID string) (*TrashEntry, error) {
	path := s.trashEntryPath(pageID)
	var entry TrashEntry
	if err := readJSON(path, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *FSStore) ListTrash() ([]*TrashEntry, error) {
	dir := s.trashDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*TrashEntry{}, nil
		}
		return nil, fmt.Errorf("read trash dir: %w", err)
	}

	trash := make([]*TrashEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		var t TrashEntry
		if err := readJSON(filepath.Join(dir, entry.Name()), &t); err != nil {
			return nil, fmt.Errorf("read trash entry %s: %w", entry.Name(), err)
		}
		trash = append(trash, &t)
	}

	sort.SliceStable(trash, func(i, j int) bool {
		return trash[i].DeletedAt.After(trash[j].DeletedAt)
	})

	return trash, nil
}

func (s *FSStore) DeleteTrashEntry(pageID string) error {
	err := os.Remove(s.trashEntryPath(pageID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete trash entry: %w", err)
	}
	return nil
}

func (s *FSStore) baseDir() string {
	return filepath.Join(s.storageDir, ".leafwiki")
}

func (s *FSStore) revisionsDir() string {
	return filepath.Join(s.baseDir(), "revisions")
}

func (s *FSStore) revisionsPageDir(pageID string) string {
	return filepath.Join(s.revisionsDir(), pageID)
}

func (s *FSStore) revisionFilePath(pageID, revisionID string, createdAt time.Time) string {
	filename := fmt.Sprintf("%s_%s.json", revisionFileTimestamp(createdAt), revisionID)
	return filepath.Join(s.revisionsPageDir(pageID), filename)
}

func (s *FSStore) contentBlobPath(hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "content", "sha256", shardHash(hash), hash)
}

func (s *FSStore) assetBlobPath(hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "assets", "sha256", shardHash(hash), hash)
}

func (s *FSStore) assetManifestPath(hash string) string {
	return filepath.Join(s.baseDir(), "manifests", "assets", "sha256", shardHash(hash), hash+".json")
}

func (s *FSStore) trashDir() string {
	return filepath.Join(s.baseDir(), "trash")
}

func (s *FSStore) trashEntryPath(pageID string) string {
	return filepath.Join(s.trashDir(), pageID+".json")
}

func shardHash(hash string) string {
	if len(hash) < 2 {
		return "00"
	}
	return hash[:2]
}

func revisionFileTimestamp(ts time.Time) string {
	return ts.UTC().Format("20060102T150405.000000000Z0700")
}

func writeJSONAtomic(dst string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, byte('\n'))
	return shared.WriteFileAtomic(dst, raw, 0o644)
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return err
	}
	return nil
}

func sha256HexBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneAndSortAssetRefs(items []AssetRef) []AssetRef {
	cloned := make([]AssetRef, len(items))
	copy(cloned, items)

	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].Name == cloned[j].Name {
			return cloned[i].SHA256 < cloned[j].SHA256
		}
		return cloned[i].Name < cloned[j].Name
	})

	return cloned
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
