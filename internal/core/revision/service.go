package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/tree"
)

type Service struct {
	storageDir string
	pages      *tree.TreeService
	store      *FSStore
	log        *slog.Logger
}

func NewService(storageDir string, pages *tree.TreeService, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		storageDir: storageDir,
		pages:      pages,
		store:      NewFSStore(storageDir),
		log:        logger.With("component", "RevisionService"),
	}
}

// CapturePageState returns a full detached snapshot including current assets.
// This is the "expensive" path and is mainly used for asset changes and delete.
func (s *Service) CapturePageState(pageID string) (*RevisionState, error) {
	return s.capturePageState(pageID, true)
}

// RecordContentUpdate records a content revision.
// Performance choice for V1:
//   - only content is re-hashed every time
//   - the latest asset manifest is reused if it already exists
//   - if this is the first revision for the page, assets are captured once
//
// Assumption: asset changes go through Upload/Rename/Delete hooks and call RecordAssetChange.
func (s *Service) RecordContentUpdate(pageID, authorID, summary string) (*Revision, bool, error) {
	prev, err := s.store.GetLatestRevision(pageID)
	if err != nil {
		return nil, false, err
	}

	state, err := s.capturePageState(pageID, false)
	if err != nil {
		return nil, false, err
	}

	if prev != nil && prev.Type != RevisionTypeDelete && prev.ContentHash == state.ContentHash {
		return prev, false, nil
	}

	assetManifestHash := ""
	if prev != nil && prev.AssetManifestHash != "" {
		assetManifestHash = prev.AssetManifestHash
	} else {
		fullState, err := s.capturePageState(pageID, true)
		if err != nil {
			return nil, false, err
		}
		if err := s.persistLiveAssets(pageID, fullState.Assets); err != nil {
			return nil, false, err
		}
		savedManifestHash, err := s.store.SaveAssetManifest(fullState.Assets)
		if err != nil {
			return nil, false, err
		}
		if savedManifestHash != fullState.AssetManifestHash {
			return nil, false, fmt.Errorf("asset manifest hash mismatch: computed=%s saved=%s", fullState.AssetManifestHash, savedManifestHash)
		}
		assetManifestHash = savedManifestHash
	}

	contentHash, err := s.store.SaveContentBlob([]byte(state.Content))
	if err != nil {
		return nil, false, err
	}
	if contentHash != state.ContentHash {
		return nil, false, fmt.Errorf("content hash mismatch: computed=%s saved=%s", state.ContentHash, contentHash)
	}

	rev, err := s.newRevision(RevisionTypeContentUpdate, state, authorID, summary, assetManifestHash)
	if err != nil {
		return nil, false, err
	}
	if err := s.store.SaveRevision(rev); err != nil {
		return nil, false, err
	}

	return rev, true, nil
}

// RecordAssetChange records a full snapshot when live assets changed.
// This method hashes the current assets and only writes a new revision when
// content or the asset manifest actually changed.
func (s *Service) RecordAssetChange(pageID, authorID, summary string) (*Revision, bool, error) {
	prev, err := s.store.GetLatestRevision(pageID)
	if err != nil {
		return nil, false, err
	}

	state, err := s.capturePageState(pageID, true)
	if err != nil {
		return nil, false, err
	}

	if prev != nil &&
		prev.Type != RevisionTypeDelete &&
		prev.ContentHash == state.ContentHash &&
		prev.AssetManifestHash == state.AssetManifestHash {
		return prev, false, nil
	}

	contentHash, err := s.store.SaveContentBlob([]byte(state.Content))
	if err != nil {
		return nil, false, err
	}
	if contentHash != state.ContentHash {
		return nil, false, fmt.Errorf("content hash mismatch: computed=%s saved=%s", state.ContentHash, contentHash)
	}

	if err := s.persistLiveAssets(pageID, state.Assets); err != nil {
		return nil, false, err
	}

	savedManifestHash, err := s.store.SaveAssetManifest(state.Assets)
	if err != nil {
		return nil, false, err
	}
	if savedManifestHash != state.AssetManifestHash {
		return nil, false, fmt.Errorf("asset manifest hash mismatch: computed=%s saved=%s", state.AssetManifestHash, savedManifestHash)
	}

	rev, err := s.newRevision(RevisionTypeAssetUpdate, state, authorID, summary, savedManifestHash)
	if err != nil {
		return nil, false, err
	}
	if err := s.store.SaveRevision(rev); err != nil {
		return nil, false, err
	}

	return rev, true, nil
}

// RecordDelete writes a delete revision + trash entry BEFORE the live delete happens.
// This is intentionally strict: if this fails, the caller should abort the live delete.
func (s *Service) RecordDelete(pageID, authorID, summary string) (*Revision, *TrashEntry, error) {
	state, err := s.capturePageState(pageID, true)
	if err != nil {
		return nil, nil, err
	}

	contentHash, err := s.store.SaveContentBlob([]byte(state.Content))
	if err != nil {
		return nil, nil, err
	}
	if contentHash != state.ContentHash {
		return nil, nil, fmt.Errorf("content hash mismatch: computed=%s saved=%s", state.ContentHash, contentHash)
	}

	if err := s.persistLiveAssets(pageID, state.Assets); err != nil {
		return nil, nil, err
	}

	savedManifestHash, err := s.store.SaveAssetManifest(state.Assets)
	if err != nil {
		return nil, nil, err
	}
	if savedManifestHash != state.AssetManifestHash {
		return nil, nil, fmt.Errorf("asset manifest hash mismatch: computed=%s saved=%s", state.AssetManifestHash, savedManifestHash)
	}

	rev, err := s.newRevision(RevisionTypeDelete, state, authorID, summary, savedManifestHash)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.SaveRevision(rev); err != nil {
		return nil, nil, err
	}

	trash := &TrashEntry{
		PageID:         state.PageID,
		DeletedAt:      rev.CreatedAt,
		DeletedBy:      authorID,
		Title:          state.Title,
		Slug:           state.Slug,
		Path:           state.Path,
		LastRevisionID: rev.ID,
	}
	if err := s.store.SaveTrashEntry(trash); err != nil {
		return nil, nil, err
	}

	return rev, trash, nil
}

func (s *Service) ListRevisions(pageID string) ([]*Revision, error) {
	return s.store.ListRevisions(pageID)
}

func (s *Service) GetLatestRevision(pageID string) (*Revision, error) {
	return s.store.GetLatestRevision(pageID)
}

func (s *Service) GetTrashEntry(pageID string) (*TrashEntry, error) {
	return s.store.GetTrashEntry(pageID)
}

func (s *Service) ListTrash() ([]*TrashEntry, error) {
	return s.store.ListTrash()
}

func (s *Service) capturePageState(pageID string, withAssets bool) (*RevisionState, error) {
	page, err := s.pages.GetPage(pageID)
	if err != nil {
		return nil, err
	}

	state := &RevisionState{
		PageID:      page.ID,
		Title:       page.Title,
		Slug:        page.Slug,
		Kind:        string(page.Kind),
		Path:        page.CalculatePath(),
		Content:     page.Content,
		ContentHash: sha256HexBytes([]byte(page.Content)),
		CapturedAt:  time.Now().UTC(),
	}

	if !withAssets {
		return state, nil
	}

	assets, err := s.scanLiveAssets(pageID)
	if err != nil {
		return nil, err
	}

	state.Assets = assets
	hash, err := computeAssetManifestHash(assets)
	if err != nil {
		return nil, err
	}
	state.AssetManifestHash = hash

	return state, nil
}

func (s *Service) newRevision(t RevisionType, state *RevisionState, authorID, summary, assetManifestHash string) (*Revision, error) {
	revisionID, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("generate revision id: %w", err)
	}

	return &Revision{
		ID:                revisionID,
		PageID:            state.PageID,
		Type:              t,
		AuthorID:          strings.TrimSpace(authorID),
		CreatedAt:         time.Now().UTC(),
		Title:             state.Title,
		Slug:              state.Slug,
		Kind:              state.Kind,
		Path:              state.Path,
		ContentHash:       state.ContentHash,
		AssetManifestHash: assetManifestHash,
		Summary:           summary,
	}, nil
}

func (s *Service) persistLiveAssets(pageID string, refs []AssetRef) error {
	if len(refs) == 0 {
		return nil
	}

	for _, ref := range refs {
		srcPath := filepath.Join(s.liveAssetDir(pageID), ref.Name)
		hash, size, err := s.store.SaveAssetBlobFromPath(srcPath)
		if err != nil {
			return err
		}
		if hash != ref.SHA256 {
			return fmt.Errorf("asset hash mismatch for %s: computed=%s saved=%s", ref.Name, ref.SHA256, hash)
		}
		if size != ref.SizeBytes {
			return fmt.Errorf("asset size mismatch for %s: computed=%d saved=%d", ref.Name, ref.SizeBytes, size)
		}
	}
	return nil
}

func (s *Service) scanLiveAssets(pageID string) ([]AssetRef, error) {
	dir := s.liveAssetDir(pageID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []AssetRef{}, nil
		}
		return nil, fmt.Errorf("read live asset dir %s: %w", dir, err)
	}

	refs := make([]AssetRef, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		absPath := filepath.Join(dir, name)

		ref, err := buildAssetRef(absPath, name)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}

	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Name == refs[j].Name {
			return refs[i].SHA256 < refs[j].SHA256
		}
		return refs[i].Name < refs[j].Name
	})

	return refs, nil
}

// Assumption for V1:
// live assets are stored under <storageDir>/assets/<pageID>/...
// If your AssetService uses a different on-disk layout, only change this method.
func (s *Service) liveAssetDir(pageID string) string {
	return filepath.Join(s.storageDir, "assets", pageID)
}

func buildAssetRef(absPath, name string) (AssetRef, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return AssetRef{}, fmt.Errorf("open asset %s: %w", absPath, err)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return AssetRef{}, fmt.Errorf("hash asset %s: %w", absPath, err)
	}

	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return AssetRef{
		Name:      name,
		SHA256:    hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes: size,
		MIMEType:  mimeType,
	}, nil
}

func computeAssetManifestHash(items []AssetRef) (string, error) {
	canonical := cloneAndSortAssetRefs(items)

	raw, err := json.Marshal(assetManifest{Items: canonical})
	if err != nil {
		return "", fmt.Errorf("marshal asset manifest for hash: %w", err)
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
