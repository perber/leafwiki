package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

type Service struct {
	storageDir       string
	pages            *tree.TreeService
	store            *FSStore
	deleteTrashEntry func(string) error
	log              *slog.Logger
}

func NewService(storageDir string, pages *tree.TreeService, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	store := NewFSStore(storageDir)
	return &Service{
		storageDir:       storageDir,
		pages:            pages,
		store:            store,
		deleteTrashEntry: store.DeleteTrashEntry,
		log:              logger.With("component", "RevisionService"),
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

	assetManifestHash, err := s.resolveAssetManifestHash(pageID, prev)
	if err != nil {
		return nil, false, err
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

func (s *Service) RecordStructureChange(pageID, authorID, summary string) (*Revision, bool, error) {
	prev, err := s.store.GetLatestRevision(pageID)
	if err != nil {
		return nil, false, err
	}

	state, err := s.capturePageState(pageID, false)
	if err != nil {
		return nil, false, err
	}

	assetManifestHash, err := s.resolveAssetManifestHash(pageID, prev)
	if err != nil {
		return nil, false, err
	}

	contentHash, err := s.store.SaveContentBlob([]byte(state.Content))
	if err != nil {
		return nil, false, err
	}
	if contentHash != state.ContentHash {
		return nil, false, fmt.Errorf("content hash mismatch: computed=%s saved=%s", state.ContentHash, contentHash)
	}

	rev, err := s.newRevision(RevisionTypeStructureUpdate, state, authorID, summary, assetManifestHash)
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

func (s *Service) resolveAssetManifestHash(pageID string, prev *Revision) (string, error) {
	if prev != nil && prev.AssetManifestHash != "" {
		if _, err := s.store.LoadAssetManifest(prev.AssetManifestHash); err == nil {
			return prev.AssetManifestHash, nil
		}
	}

	fullState, err := s.capturePageState(pageID, true)
	if err != nil {
		return "", err
	}
	if err := s.persistLiveAssets(pageID, fullState.Assets); err != nil {
		return "", err
	}
	savedManifestHash, err := s.store.SaveAssetManifest(fullState.Assets)
	if err != nil {
		return "", err
	}
	if savedManifestHash != fullState.AssetManifestHash {
		return "", fmt.Errorf("asset manifest hash mismatch: computed=%s saved=%s", fullState.AssetManifestHash, savedManifestHash)
	}
	return savedManifestHash, nil
}

func (s *Service) finalizeRestoreCommit(pageID, authorID string) error {
	trash, err := s.store.GetTrashEntry(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}

	latest, err := s.store.GetLatestRevision(pageID)
	if err != nil {
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		if err := s.recordRestoreRevision(pageID, authorID); err != nil {
			return sharederrors.NewLocalizedError(
				"revision_restore_failed",
				"Failed to restore page",
				"failed to restore page %s",
				err,
				pageID,
			)
		}
	}

	if err := s.deleteTrashEntry(pageID); err != nil {
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}

	_ = trash
	return nil
}

func (s *Service) ListRevisions(pageID string) ([]*Revision, error) {
	return s.store.ListRevisions(pageID)
}

func (s *Service) ListRevisionsPage(pageID, cursor string, limit int) ([]*Revision, string, error) {
	return s.store.ListRevisionsPage(pageID, cursor, limit)
}

func (s *Service) GetLatestRevision(pageID string) (*Revision, error) {
	return s.store.GetLatestRevision(pageID)
}

func (s *Service) GetRevisionSnapshot(pageID, revisionID string) (*RevisionSnapshot, error) {
	rev, err := s.store.GetRevision(pageID, revisionID)
	if err != nil {
		return nil, err
	}

	content, err := s.store.ReadContentBlob(rev.ContentHash)
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"revision_preview_content_unavailable",
			"Revision content is unavailable",
			"revision content for page %s revision %s is unavailable",
			err,
			pageID,
			revisionID,
		)
	}

	assets, err := s.store.LoadAssetManifest(rev.AssetManifestHash)
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"revision_preview_assets_unavailable",
			"Revision assets are unavailable",
			"revision assets for page %s revision %s are unavailable",
			err,
			pageID,
			revisionID,
		)
	}

	return &RevisionSnapshot{
		Revision: rev,
		Content:  string(content),
		Assets:   cloneAndSortAssetRefs(assets),
	}, nil
}

func (s *Service) CompareRevisionSnapshots(pageID, baseRevisionID, targetRevisionID string) (*RevisionComparison, error) {
	base, err := s.GetRevisionSnapshot(pageID, baseRevisionID)
	if err != nil {
		return nil, err
	}
	target, err := s.GetRevisionSnapshot(pageID, targetRevisionID)
	if err != nil {
		return nil, err
	}
	return &RevisionComparison{
		Base:           base,
		Target:         target,
		ContentChanged: base.Content != target.Content,
		AssetChanges:   compareRevisionAssets(base.Assets, target.Assets),
	}, nil
}

func (s *Service) GetRevisionAsset(pageID, revisionID, assetName string) (*RevisionAssetContent, error) {
	assetName = strings.TrimSpace(strings.TrimPrefix(assetName, "/"))
	if assetName == "" {
		return nil, sharederrors.NewLocalizedError(
			"revision_preview_asset_invalid_name",
			"Revision asset name is invalid",
			"revision asset name for page %s revision %s is invalid",
			fmt.Errorf("asset name is required"),
			pageID,
			revisionID,
		)
	}

	rev, err := s.store.GetRevision(pageID, revisionID)
	if err != nil {
		return nil, err
	}

	assets, err := s.store.LoadAssetManifest(rev.AssetManifestHash)
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"revision_preview_assets_unavailable",
			"Revision assets are unavailable",
			"revision assets for page %s revision %s are unavailable",
			err,
			pageID,
			revisionID,
		)
	}

	for _, asset := range assets {
		if asset.Name != assetName {
			continue
		}

		content, err := s.store.ReadAssetBlob(asset.SHA256)
		if err != nil {
			return nil, sharederrors.NewLocalizedError(
				"revision_preview_asset_blob_unavailable",
				"Revision asset is unavailable",
				"revision asset %s for page %s revision %s is unavailable",
				err,
				assetName,
				pageID,
				revisionID,
			)
		}

		return &RevisionAssetContent{
			Asset:   asset,
			Content: content,
		}, nil
	}

	return nil, sharederrors.NewLocalizedError(
		"revision_preview_asset_not_found",
		"Revision asset not found",
		"revision asset %s for page %s revision %s not found",
		fmt.Errorf("asset %q not found in revision manifest", assetName),
		assetName,
		pageID,
		revisionID,
	)
}

func compareRevisionAssets(baseAssets, targetAssets []AssetRef) []RevisionAssetDelta {
	baseByName := make(map[string]AssetRef, len(baseAssets))
	for _, asset := range baseAssets {
		baseByName[asset.Name] = asset
	}
	targetByName := make(map[string]AssetRef, len(targetAssets))
	for _, asset := range targetAssets {
		targetByName[asset.Name] = asset
	}
	changes := make([]RevisionAssetDelta, 0)
	for name, baseAsset := range baseByName {
		targetAsset, ok := targetByName[name]
		if !ok {
			changes = append(changes, RevisionAssetDelta{Name: name, Status: "removed"})
			continue
		}
		if baseAsset.SHA256 != targetAsset.SHA256 || baseAsset.SizeBytes != targetAsset.SizeBytes {
			changes = append(changes, RevisionAssetDelta{Name: name, Status: "modified"})
		}
	}
	for name := range targetByName {
		if _, ok := baseByName[name]; !ok {
			changes = append(changes, RevisionAssetDelta{Name: name, Status: "added"})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Name < changes[j].Name })
	return changes
}

func (s *Service) GetTrashEntry(pageID string) (*TrashEntry, error) {
	return s.store.GetTrashEntry(pageID)
}

func (s *Service) ListTrash() ([]*TrashEntry, error) {
	return s.store.ListTrash()
}

func (s *Service) CheckRevisionIntegrity(pageID string) ([]RevisionIntegrityIssue, error) {
	revisions, err := s.store.ListRevisions(pageID)
	if err != nil {
		return nil, err
	}

	issues := make([]RevisionIntegrityIssue, 0)
	for _, rev := range revisions {
		if rev == nil {
			continue
		}
		if strings.TrimSpace(rev.ContentHash) != "" {
			if _, err := s.store.ReadContentBlob(rev.ContentHash); err != nil {
				issues = append(issues, RevisionIntegrityIssue{PageID: rev.PageID, RevisionID: rev.ID, Code: "missing_content_blob", Message: "Revision content blob is missing or unreadable", Path: s.store.contentBlobPath(rev.ContentHash)})
			}
		}
		refs, err := s.store.LoadAssetManifest(rev.AssetManifestHash)
		if err != nil {
			issues = append(issues, RevisionIntegrityIssue{PageID: rev.PageID, RevisionID: rev.ID, Code: "missing_asset_manifest", Message: "Revision asset manifest is missing or unreadable", Path: s.store.assetManifestPath(rev.AssetManifestHash)})
			continue
		}
		for _, ref := range refs {
			raw, err := s.store.ReadAssetBlob(ref.SHA256)
			if err != nil {
				issues = append(issues, RevisionIntegrityIssue{PageID: rev.PageID, RevisionID: rev.ID, Code: "missing_asset_blob", Message: fmt.Sprintf("Revision asset blob for %s is missing or unreadable", ref.Name), Path: s.store.assetBlobPath(ref.SHA256)})
				continue
			}
			if sha256HexBytes(raw) != ref.SHA256 {
				issues = append(issues, RevisionIntegrityIssue{PageID: rev.PageID, RevisionID: rev.ID, Code: "asset_blob_hash_mismatch", Message: fmt.Sprintf("Revision asset blob for %s failed hash verification", ref.Name), Path: s.store.assetBlobPath(ref.SHA256)})
				continue
			}
			if int64(len(raw)) != ref.SizeBytes {
				issues = append(issues, RevisionIntegrityIssue{PageID: rev.PageID, RevisionID: rev.ID, Code: "asset_blob_size_mismatch", Message: fmt.Sprintf("Revision asset blob for %s failed size verification", ref.Name), Path: s.store.assetBlobPath(ref.SHA256)})
			}
		}
	}
	return issues, nil
}

func (s *Service) RestorePage(pageID, authorID string, targetParentID *string) error {
	pageID = strings.TrimSpace(pageID)
	if pageID == "" {
		return sharederrors.NewLocalizedError(
			"revision_restore_invalid_page_id",
			"Failed to restore page",
			"failed to restore page %s",
			nil,
			pageID,
		)
	}

	if s.pages != nil {
		if page, err := s.pages.GetPage(pageID); err == nil && page != nil {
			return s.finalizeRestoreCommit(pageID, authorID)
		}
	}

	trash, err := s.store.GetTrashEntry(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return sharederrors.NewLocalizedError(
				"revision_restore_trash_not_found",
				"Trash entry not found",
				"trash entry for page %s not found",
				err,
				pageID,
			)
		}
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}

	rev, err := s.store.GetRevision(pageID, trash.LastRevisionID)
	if err != nil {
		if os.IsNotExist(err) {
			return sharederrors.NewLocalizedError(
				"revision_restore_revision_not_found",
				"Restore revision not found",
				"restore revision %s for page %s not found",
				err,
				trash.LastRevisionID,
				pageID,
			)
		}
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}
	if rev.Type != RevisionTypeDelete {
		return sharederrors.NewLocalizedError(
			"revision_restore_invalid_revision",
			"Restore revision is invalid",
			"restore revision %s for page %s is invalid",
			nil,
			trash.LastRevisionID,
			pageID,
		)
	}

	parentID, err := s.resolveRestoreParentID(pageID, rev.ParentID, rev.Path, targetParentID)
	if err != nil {
		return err
	}

	kind, err := restoreNodeKind(rev.Kind)
	if err != nil {
		return sharederrors.NewLocalizedError(
			"revision_restore_invalid_kind",
			"Restore revision has an invalid page kind",
			"restore revision for page %s has invalid kind %s",
			err,
			pageID,
			rev.Kind,
		)
	}

	content, err := s.store.ReadContentBlob(rev.ContentHash)
	if err != nil {
		return sharederrors.NewLocalizedError(
			"revision_restore_content_missing",
			"Restore content is unavailable",
			"restore content for page %s is unavailable",
			err,
			pageID,
		)
	}

	assets, err := s.store.LoadAssetManifest(rev.AssetManifestHash)
	if err != nil {
		return sharederrors.NewLocalizedError(
			"revision_restore_assets_missing",
			"Restore assets are unavailable",
			"restore assets for page %s are unavailable",
			err,
			pageID,
		)
	}

	if _, err := s.pages.RestoreNode(authorID, pageID, parentID, rev.Title, rev.Slug, kind, string(content), tree.PageMetadata{CreatedAt: rev.PageCreatedAt, UpdatedAt: rev.PageUpdatedAt, CreatorID: rev.CreatorID, LastAuthorID: rev.LastAuthorID}); err != nil {
		return s.mapRestoreTreeError(pageID, rev.Slug, parentID, err)
	}

	if err := s.restoreAssets(pageID, assets); err != nil {
		s.rollbackRestoredNode(authorID, pageID)
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}

	return s.finalizeRestoreCommit(pageID, authorID)
}

func (s *Service) capturePageState(pageID string, withAssets bool) (*RevisionState, error) {
	page, err := s.pages.GetPage(pageID)
	if err != nil {
		return nil, err
	}

	parentID := ""
	if page.Parent != nil && page.Parent.ID != "root" {
		parentID = page.Parent.ID
	}

	state := &RevisionState{
		PageID:        page.ID,
		ParentID:      parentID,
		Title:         page.Title,
		Slug:          page.Slug,
		Kind:          string(page.Kind),
		Path:          page.CalculatePath(),
		Content:       page.Content,
		ContentHash:   sha256HexBytes([]byte(page.Content)),
		PageCreatedAt: page.Metadata.CreatedAt.UTC(),
		PageUpdatedAt: page.Metadata.UpdatedAt.UTC(),
		CreatorID:     strings.TrimSpace(page.Metadata.CreatorID),
		LastAuthorID:  strings.TrimSpace(page.Metadata.LastAuthorID),
		CapturedAt:    time.Now().UTC(),
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
		ParentID:          state.ParentID,
		Type:              t,
		AuthorID:          strings.TrimSpace(authorID),
		CreatedAt:         time.Now().UTC(),
		Title:             state.Title,
		Slug:              state.Slug,
		Kind:              state.Kind,
		Path:              state.Path,
		ContentHash:       state.ContentHash,
		AssetManifestHash: assetManifestHash,
		PageCreatedAt:     state.PageCreatedAt.UTC(),
		PageUpdatedAt:     state.PageUpdatedAt.UTC(),
		CreatorID:         strings.TrimSpace(state.CreatorID),
		LastAuthorID:      strings.TrimSpace(state.LastAuthorID),
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

func (s *Service) resolveRestoreParentID(pageID, storedParentID, pagePath string, targetParentID *string) (*string, error) {
	if targetParentID != nil {
		trimmed := strings.TrimSpace(*targetParentID)
		switch trimmed {
		case "", "root":
			return nil, nil
		default:
			if _, err := s.pages.GetPage(trimmed); err != nil {
				return nil, sharederrors.NewLocalizedError(
					"revision_restore_parent_not_found",
					"Restore target parent not found",
					"restore target parent %s for page %s not found",
					err,
					trimmed,
					pageID,
				)
			}
			parentID := trimmed
			return &parentID, nil
		}
	}

	trimmedStoredParentID := strings.TrimSpace(storedParentID)
	if trimmedStoredParentID != "" {
		if parent, err := s.pages.GetPage(trimmedStoredParentID); err == nil && parent != nil {
			parentID := trimmedStoredParentID
			return &parentID, nil
		}
	}

	parentPath := restoreParentRoutePath(pagePath)
	if parentPath == "" {
		return nil, nil
	}

	parent, err := s.pages.FindPageByRoutePath(parentPath)
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"revision_restore_parent_required",
			"Original parent no longer exists; choose a restore target",
			"original parent for page %s no longer exists, choose a restore target",
			err,
			pageID,
		)
	}

	parentID := parent.ID
	return &parentID, nil
}

func restoreParentRoutePath(pagePath string) string {
	trimmed := strings.Trim(strings.TrimSpace(pagePath), "/")
	if trimmed == "" {
		return ""
	}
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		return ""
	}
	return trimmed[:idx]
}

func restoreNodeKind(kind string) (tree.NodeKind, error) {
	switch tree.NodeKind(strings.TrimSpace(kind)) {
	case tree.NodeKindPage:
		return tree.NodeKindPage, nil
	case tree.NodeKindSection:
		return tree.NodeKindSection, nil
	default:
		return "", fmt.Errorf("invalid node kind: %s", kind)
	}
}

func (s *Service) mapRestoreTreeError(pageID, slug string, parentID *string, err error) error {
	if err == nil {
		return nil
	}

	parentArg := "root"
	if parentID != nil && strings.TrimSpace(*parentID) != "" {
		parentArg = strings.TrimSpace(*parentID)
	}

	switch {
	case errors.Is(err, tree.ErrParentNotFound):
		return sharederrors.NewLocalizedError(
			"revision_restore_parent_not_found",
			"Restore target parent not found",
			"restore target parent %s for page %s not found",
			err,
			parentArg,
			pageID,
		)
	case errors.Is(err, tree.ErrPageAlreadyExists):
		return sharederrors.NewLocalizedError(
			"revision_restore_slug_conflict",
			"Restore target already contains a page with the same slug",
			"restore target already contains a page with slug %s",
			err,
			slug,
		)
	default:
		return sharederrors.NewLocalizedError(
			"revision_restore_failed",
			"Failed to restore page",
			"failed to restore page %s",
			err,
			pageID,
		)
	}
}

func (s *Service) rollbackRestoredNode(authorID, pageID string) {
	if err := s.pages.DeleteNode(authorID, pageID, true); err != nil {
		s.log.Warn("failed to rollback restored page", "pageID", pageID, "error", err)
	}
	if err := os.RemoveAll(s.liveAssetDir(pageID)); err != nil {
		s.log.Warn("failed to rollback restored assets", "pageID", pageID, "error", err)
	}
}

func (s *Service) restoreAssets(pageID string, refs []AssetRef) error {
	dir := s.liveAssetDir(pageID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reset live asset dir: %w", err)
	}
	if len(refs) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure live asset dir: %w", err)
	}

	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		name := strings.TrimSpace(ref.Name)
		if name == "" || filepath.Base(name) != name || strings.Contains(name, string(os.PathSeparator)) {
			return fmt.Errorf("invalid asset name: %s", ref.Name)
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate asset name in manifest: %s", name)
		}
		seen[name] = struct{}{}

		raw, err := s.store.ReadAssetBlob(ref.SHA256)
		if err != nil {
			return err
		}
		if sha256HexBytes(raw) != ref.SHA256 {
			return fmt.Errorf("restored asset hash mismatch for %s", name)
		}
		if int64(len(raw)) != ref.SizeBytes {
			return fmt.Errorf("restored asset size mismatch for %s", name)
		}
		if err := shared.WriteFileAtomic(filepath.Join(dir, name), raw, 0o644); err != nil {
			return fmt.Errorf("write restored asset %s: %w", name, err)
		}
	}

	return nil
}

func (s *Service) recordRestoreRevision(pageID, authorID string) error {
	state, err := s.capturePageState(pageID, true)
	if err != nil {
		return err
	}

	contentHash, err := s.store.SaveContentBlob([]byte(state.Content))
	if err != nil {
		return err
	}
	if contentHash != state.ContentHash {
		return fmt.Errorf("content hash mismatch: computed=%s saved=%s", state.ContentHash, contentHash)
	}

	if err := s.persistLiveAssets(pageID, state.Assets); err != nil {
		return err
	}

	savedManifestHash, err := s.store.SaveAssetManifest(state.Assets)
	if err != nil {
		return err
	}
	if savedManifestHash != state.AssetManifestHash {
		return fmt.Errorf("asset manifest hash mismatch: computed=%s saved=%s", state.AssetManifestHash, savedManifestHash)
	}

	rev, err := s.newRevision(RevisionTypeRestore, state, authorID, "page restored", savedManifestHash)
	if err != nil {
		return err
	}
	return s.store.SaveRevision(rev)
}
