// Package draft persists private working copies for published pages.
package draft

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/perber/wiki/internal/core/shared"
)

var (
	ErrNotFound      = errors.New("draft not found")
	ErrInvalidPageID = errors.New("invalid page ID")
	ErrCorrupt       = errors.New("corrupt draft")
)

// Draft is the single shared working copy for an existing page.
// Structural fields intentionally do not belong to a draft.
type Draft struct {
	PageID      string            `json:"pageId"`
	BaseVersion string            `json:"baseVersion"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Tags        []string          `json:"tags"`
	Properties  map[string]string `json:"properties"`
}

// PendingDraft is a new page that has not entered the canonical tree yet.
type PendingDraft struct {
	ID         string            `json:"id"`
	ParentID   string            `json:"parentId"`
	Title      string            `json:"title"`
	Slug       string            `json:"slug"`
	Content    string            `json:"content"`
	Tags       []string          `json:"tags"`
	Properties map[string]string `json:"properties"`
}

type Store struct{ dir string }

func NewStore(storageDir string) *Store {
	return &Store{dir: filepath.Join(storageDir, ".leafwiki", "drafts")}
}

func (s *Store) Get(pageID string) (*Draft, error) {
	path, err := s.path(pageID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read draft: %w", err)
	}
	var d Draft
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if d.PageID != pageID || strings.TrimSpace(d.BaseVersion) == "" || strings.TrimSpace(d.Title) == "" {
		return nil, fmt.Errorf("%w: invalid fields", ErrCorrupt)
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Properties == nil {
		d.Properties = map[string]string{}
	}
	return &d, nil
}

func (s *Store) Save(d Draft) error {
	path, err := s.path(d.PageID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(d.BaseVersion) == "" || strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("%w: missing required fields", ErrCorrupt)
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Properties == nil {
		d.Properties = map[string]string{}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal draft: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create draft directory: %w", err)
	}
	if err := shared.WriteFileAtomic(path, raw, 0o600); err != nil {
		return fmt.Errorf("write draft: %w", err)
	}
	return nil
}

func (s *Store) Delete(pageID string) error {
	path, err := s.path(pageID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}

func (s *Store) Exists(pageID string) (bool, error) {
	path, err := s.path(pageID)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat draft: %w", err)
}

func (s *Store) CreatePending(parentID, title, slug string) (*PendingDraft, error) {
	id, err := shared.GenerateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("generate pending draft id: %w", err)
	}
	d := PendingDraft{ID: id, ParentID: parentID, Title: title, Slug: slug, Tags: []string{}, Properties: map[string]string{}}
	return &d, s.SavePending(d)
}

func (s *Store) GetPending(id string) (*PendingDraft, error) {
	path, err := s.pendingPath(id)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read pending draft: %w", err)
	}
	var d PendingDraft
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if d.ID != id || strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Slug) == "" {
		return nil, fmt.Errorf("%w: invalid fields", ErrCorrupt)
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Properties == nil {
		d.Properties = map[string]string{}
	}
	return &d, nil
}

func (s *Store) SavePending(d PendingDraft) error {
	path, err := s.pendingPath(d.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Slug) == "" {
		return fmt.Errorf("%w: missing required fields", ErrCorrupt)
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Properties == nil {
		d.Properties = map[string]string{}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal pending draft: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create pending drafts directory: %w", err)
	}
	return shared.WriteFileAtomic(path, raw, 0o600)
}

func (s *Store) DeletePending(id string) error {
	path, err := s.pendingPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("delete pending draft: %w", err)
	}
	return nil
}

func (s *Store) path(pageID string) (string, error) {
	if !validPageID(pageID) {
		return "", ErrInvalidPageID
	}
	return filepath.Join(s.dir, pageID+".json"), nil
}

func (s *Store) pendingPath(id string) (string, error) {
	if !validPageID(id) {
		return "", ErrInvalidPageID
	}
	return filepath.Join(s.dir, "pending", id+".json"), nil
}

func validPageID(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*-_", r) {
			return false
		}
	}
	return true
}
