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

type revisionIndex map[string]string

const revisionIndexFileName = "_index.json"

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
	dst := s.AssetBlobPath(hash)

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
	index, err := s.loadRevisionIndex(rev.PageID)
	if err != nil {
		return err
	}
	index[strings.TrimSpace(rev.ID)] = filepath.Base(dst)
	if err := s.saveRevisionIndex(rev.PageID, index); err != nil {
		return err
	}
	return nil
}

func (s *FSStore) ListRevisions(pageID string) ([]*Revision, error) {
	revisions, _, err := s.ListRevisionsPage(pageID, "", 0)
	if err != nil {
		return nil, err
	}
	return revisions, nil
}

func (s *FSStore) ListRevisionsPage(pageID, cursor string, limit int) ([]*Revision, string, error) {
	names, err := s.revisionFileNames(pageID)
	if err != nil {
		return nil, "", err
	}
	if len(names) == 0 {
		return []*Revision{}, "", nil
	}

	start := 0
	cursor = strings.TrimSpace(cursor)
	if cursor != "" {
		start = len(names)
		for i, name := range names {
			if name == cursor {
				start = i + 1
				break
			}
		}
		if start >= len(names) {
			return []*Revision{}, "", nil
		}
	}

	end := len(names)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	dir := s.revisionsPageDir(pageID)
	revisions := make([]*Revision, 0, end-start)
	for _, name := range names[start:end] {
		var rev Revision
		if err := readJSON(filepath.Join(dir, name), &rev); err != nil {
			return nil, "", fmt.Errorf("read revision %s: %w", name, err)
		}
		revisions = append(revisions, &rev)
	}

	nextCursor := ""
	if end < len(names) {
		nextCursor = names[end-1]
	}
	return revisions, nextCursor, nil
}

func (s *FSStore) GetLatestRevision(pageID string) (*Revision, error) {
	names, err := s.revisionFileNames(pageID)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	var rev Revision
	if err := readJSON(filepath.Join(s.revisionsPageDir(pageID), names[0]), &rev); err != nil {
		return nil, fmt.Errorf("read latest revision %s: %w", names[0], err)
	}
	return &rev, nil
}

func (s *FSStore) GetRevision(pageID, revisionID string) (*Revision, error) {
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return nil, os.ErrNotExist
	}

	index, err := s.loadRevisionIndex(pageID)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(index[revisionID]); name != "" {
		var rev Revision
		if err := readJSON(filepath.Join(s.revisionsPageDir(pageID), name), &rev); err != nil {
			return nil, fmt.Errorf("read revision %s: %w", name, err)
		}
		return &rev, nil
	}

	names, err := s.revisionFileNames(pageID)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_"+revisionID+".json") {
			var rev Revision
			if err := readJSON(filepath.Join(s.revisionsPageDir(pageID), name), &rev); err != nil {
				return nil, fmt.Errorf("read revision %s: %w", name, err)
			}
			index[revisionID] = name
			_ = s.saveRevisionIndex(pageID, index)
			return &rev, nil
		}
	}
	return nil, os.ErrNotExist
}

// PruneRevisions removes the oldest revision files beyond keepCount for the given page.
// Files are sorted newest-first, so names[keepCount:] are the oldest ones.
// Content blobs and asset manifests are NOT deleted — they are content-addressed and
// may be shared across multiple revisions.
func (s *FSStore) PruneRevisions(pageID string, keepCount int) error {
	if keepCount <= 0 {
		return nil
	}
	names, err := s.revisionFileNames(pageID)
	if err != nil || len(names) <= keepCount {
		return err
	}

	index, err := s.loadRevisionIndex(pageID)
	if err != nil {
		return err
	}

	// Build reverse map: filename → revisionID for index cleanup
	filenameToID := make(map[string]string, len(index))
	for id, filename := range index {
		filenameToID[filename] = id
	}

	toDelete := names[keepCount:]
	dir := s.revisionsPageDir(pageID)
	indexChanged := false

	for _, name := range toDelete {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete revision file %s: %w", name, err)
		}
		if id, ok := filenameToID[name]; ok {
			delete(index, id)
			indexChanged = true
		}
	}

	if indexChanged {
		return s.saveRevisionIndex(pageID, index)
	}
	return nil
}

func (s *FSStore) revisionFileNames(pageID string) ([]string, error) {
	dir := s.revisionsPageDir(pageID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read revisions dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") || entry.Name() == revisionIndexFileName {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (s *FSStore) ReadContentBlob(hash string) ([]byte, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return []byte{}, nil
	}

	raw, err := os.ReadFile(s.contentBlobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("read content blob: %w", err)
	}
	return raw, nil
}

func (s *FSStore) LoadAssetManifest(hash string) ([]AssetRef, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return []AssetRef{}, nil
	}

	var manifest assetManifest
	if err := readJSON(s.assetManifestPath(hash), &manifest); err != nil {
		return nil, fmt.Errorf("read asset manifest: %w", err)
	}
	return cloneAndSortAssetRefs(manifest.Items), nil
}

func (s *FSStore) ReadAssetBlob(hash string) ([]byte, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("asset hash is required")
	}

	raw, err := os.ReadFile(s.AssetBlobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("read asset blob: %w", err)
	}
	return raw, nil
}

// OpenAssetBlob returns an open file handle for the given asset blob.
// The caller is responsible for closing the returned file.
func (s *FSStore) OpenAssetBlob(hash string) (*os.File, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("asset hash is required")
	}
	f, err := os.Open(s.AssetBlobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("open asset blob: %w", err)
	}
	return f, nil
}

// CopyAssetBlobToPath streams the asset blob identified by hash to dstPath,
// verifying hash and size during the copy. The write is atomic (temp + rename).
func (s *FSStore) CopyAssetBlobToPath(hash string, expectedSize int64, dstPath string) error {
	src, err := s.OpenAssetBlob(hash)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	tmpDir := filepath.Dir(dstPath)
	tmp, err := os.CreateTemp(tmpDir, "asset-restore-*")
	if err != nil {
		return fmt.Errorf("create temp restore file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = tmp.Close(); _ = os.Remove(tmpName) }

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), src)
	if err != nil {
		cleanup()
		return fmt.Errorf("stream asset blob to %s: %w", dstPath, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("chmod restored asset: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp restore file: %w", err)
	}
	if computedHash := hex.EncodeToString(hasher.Sum(nil)); computedHash != hash {
		_ = os.Remove(tmpName)
		return fmt.Errorf("asset blob hash mismatch: computed %s, want %s", computedHash, hash)
	}
	if written != expectedSize {
		_ = os.Remove(tmpName)
		return fmt.Errorf("asset blob size mismatch: got %d, want %d", written, expectedSize)
	}
	if err := os.Rename(tmpName, dstPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("move restored asset into place: %w", err)
	}
	return nil
}


func (s *FSStore) DeletePageRevisions(pageID string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return nil
	}

	if err := os.RemoveAll(s.revisionsPageDir(pageID)); err != nil {
		return fmt.Errorf("delete page revisions: %w", err)
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

func (s *FSStore) AssetBlobPath(hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "assets", "sha256", shardHash(hash), hash)
}

func (s *FSStore) assetManifestPath(hash string) string {
	return filepath.Join(s.baseDir(), "manifests", "assets", "sha256", shardHash(hash), hash+".json")
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

func (s *FSStore) revisionIndexPath(pageID string) string {
	return filepath.Join(s.revisionsPageDir(pageID), revisionIndexFileName)
}

func (s *FSStore) loadRevisionIndex(pageID string) (revisionIndex, error) {
	path := s.revisionIndexPath(pageID)
	var index revisionIndex
	if err := readJSON(path, &index); err != nil {
		if os.IsNotExist(err) {
			return revisionIndex{}, nil
		}
		return nil, fmt.Errorf("read revision index: %w", err)
	}
	if index == nil {
		return revisionIndex{}, nil
	}
	return index, nil
}

func (s *FSStore) saveRevisionIndex(pageID string, index revisionIndex) error {
	if index == nil {
		index = revisionIndex{}
	}
	path := s.revisionIndexPath(pageID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure revision dir: %w", err)
	}
	if err := writeJSONAtomic(path, index); err != nil {
		return fmt.Errorf("write revision index: %w", err)
	}
	return nil
}
