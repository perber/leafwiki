package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
)

type FSStore struct {
	storageDir string
	log        *slog.Logger
}

type revisionIndex map[string]string

const revisionIndexFileName = "_index.json"

func NewFSStore(storageDir string, logger *slog.Logger) *FSStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &FSStore{
		storageDir: storageDir,
		log:        logger.With("component", "FSStore"),
	}
}

func (s *FSStore) SaveContentBlob(pageID string, content []byte) (string, error) {
	if err := validateStorageID(pageID); err != nil {
		return "", fmt.Errorf("invalid page ID: %w", err)
	}
	hash := sha256HexBytes(content)
	dst := s.contentBlobPath(pageID, hash)

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
	if err := validateStorageID(rev.PageID); err != nil {
		return fmt.Errorf("page id is required")
	}
	if rev.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	if err := validateStorageID(rev.ID); err != nil {
		return fmt.Errorf("invalid revision id: %s", rev.ID)
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
	if err := validateStorageID(pageID); err != nil {
		return nil, "", fmt.Errorf("invalid page ID: %w", err)
	}
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
	if err := validateStorageID(pageID); err != nil {
		return nil, fmt.Errorf("invalid page ID: %w", err)
	}
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
	if err := validateStorageID(pageID); err != nil {
		return nil, fmt.Errorf("invalid page ID: %w", err)
	}
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

// UpdateRevision overwrites an existing revision file in-place (used for coalescing).
// The revision ID and filename are unchanged; only the JSON content is replaced.
func (s *FSStore) UpdateRevision(rev *Revision) error {
	if err := validateStorageID(rev.PageID); err != nil {
		return fmt.Errorf("invalid page ID: %w", err)
	}
	index, err := s.loadRevisionIndex(rev.PageID)
	if err != nil {
		return err
	}
	filename := filepath.Base(strings.TrimSpace(index[rev.ID]))
	if filename == "" || filename == "." {
		return fmt.Errorf("revision %s not found in index for page %s", rev.ID, rev.PageID)
	}
	dst := filepath.Join(s.revisionsPageDir(rev.PageID), filename)
	return writeJSONAtomic(dst, rev)
}

// PruneRevisions removes the oldest revision files beyond keepCount for the given page.
// Files are sorted newest-first, so names[keepCount:] are the oldest ones.
// Content blobs and asset manifests are NOT deleted — they are content-addressed and
// may be shared across multiple revisions.
func (s *FSStore) PruneRevisions(pageID string, keepCount int) error {
	if keepCount <= 0 {
		return nil
	}
	if err := validateStorageID(pageID); err != nil {
		return fmt.Errorf("invalid page ID: %w", err)
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

func (s *FSStore) ReadContentBlob(pageID, hash string) ([]byte, error) {
	if err := validateStorageID(pageID); err != nil {
		return nil, fmt.Errorf("invalid page ID: %w", err)
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return []byte{}, nil
	}
	if raw, err := os.ReadFile(s.contentBlobPath(pageID, hash)); err == nil {
		return raw, nil
	}
	s.log.Debug("content blob not found at scoped path, falling back to legacy", "pageID", pageID, "hash", hash)
	raw, err := os.ReadFile(s.contentBlobPathLegacy(hash))
	if err != nil {
		return nil, fmt.Errorf("read content blob: %w", err)
	}
	return raw, nil
}

// OpenContentBlob returns a streaming reader for the content blob.
// The caller is responsible for closing the returned ReadCloser.
// Use this instead of ReadContentBlob when you don't need the full content in memory.
func (s *FSStore) OpenContentBlob(pageID, hash string) (io.ReadCloser, error) {
	if err := validateStorageID(pageID); err != nil {
		return nil, fmt.Errorf("invalid page ID: %w", err)
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return io.NopCloser(strings.NewReader("")), nil
	}
	if f, err := os.Open(s.contentBlobPath(pageID, hash)); err == nil {
		return f, nil
	}
	s.log.Debug("content blob not found at scoped path, falling back to legacy", "pageID", pageID, "hash", hash)
	f, err := os.Open(s.contentBlobPathLegacy(hash))
	if err != nil {
		return nil, fmt.Errorf("open content blob: %w", err)
	}
	return f, nil
}

// DeleteContentBlobIfUnreferenced deletes the content blob for the given hash if no
// revision of pageID still references it. Errors are non-fatal — callers should log them.
func (s *FSStore) DeleteContentBlobIfUnreferenced(pageID, hash string) error {
	if err := validateStorageID(pageID); err != nil {
		return fmt.Errorf("invalid page ID: %w", err)
	}
	names, err := s.revisionFileNames(pageID)
	if err != nil {
		return err
	}
	dir := s.revisionsPageDir(pageID)
	for _, name := range names {
		var rev Revision
		if err := readJSON(filepath.Join(dir, name), &rev); err != nil {
			return err
		}
		if rev.ContentHash == hash {
			return nil
		}
	}
	for _, p := range []string{s.contentBlobPath(pageID, hash), s.contentBlobPathLegacy(hash)} {
		if err := os.Remove(p); err == nil {
			s.log.Debug("deleted orphaned content blob", "pageID", pageID, "hash", hash, "path", p)
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("delete orphaned content blob: %w", err)
		}
	}
	return nil
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
	hash = strings.ToLower(strings.TrimSpace(hash))
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
	if err := validateStorageID(pageID); err != nil {
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

func (s *FSStore) contentBlobPath(pageID, hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "content", pageID, "sha256", shardHash(hash), hash)
}

func (s *FSStore) contentBlobPathLegacy(hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "content", "sha256", shardHash(hash), hash)
}

func (s *FSStore) AssetBlobPath(hash string) string {
	return filepath.Join(s.baseDir(), "blobs", "assets", "sha256", shardHash(hash), hash)
}

func (s *FSStore) AssetManifestExists(hash string) bool {
	if hash == "" {
		return false
	}
	_, err := os.Stat(s.assetManifestPath(hash))
	return err == nil
}

func (s *FSStore) assetManifestPath(hash string) string {
	return filepath.Join(s.baseDir(), "manifests", "assets", "sha256", shardHash(hash), hash+".json")
}

// validateStorageID checks that an ID is safe to use as a single file path component.
// Rejects empty strings, path separators, and dot-only segments like "." or "..".
func validateStorageID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("id must not be empty")
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("id must not contain path separators")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("id must not be a dot component")
	}
	return nil
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
