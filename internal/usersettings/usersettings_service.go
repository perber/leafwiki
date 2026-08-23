package usersettings

import (
	"fmt"
	"strings"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// UserSettingsPatch carries only the fields a caller wants to change —
// nil fields are left untouched by Update.
type UserSettingsPatch struct {
	Language *string
	AutoSave *bool
}

type UserSettingsService struct {
	store *UserSettingsStore
}

func NewUserSettingsService(store *UserSettingsStore) *UserSettingsService {
	return &UserSettingsService{store: store}
}

// Get returns userID's settings, defaulted if the user has never saved any.
func (s *UserSettingsService) Get(userID string) (*UserSettings, error) {
	settings, err := s.store.Get(userID)
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"usersettings_load_failed",
			"Failed to load user settings",
			"failed to load user settings",
			err,
		)
	}
	return settings, nil
}

// Update merges patch's non-nil fields onto userID's current settings and
// saves the result. The read-modify-write happens atomically (UpdateAtomic),
// so two concurrent updates for the same user can't race and silently drop
// one of the two changes.
func (s *UserSettingsService) Update(userID string, patch UserSettingsPatch) (*UserSettings, error) {
	if patch.Language != nil && !IsAllowedLanguage(*patch.Language) {
		ve := sharederrors.NewValidationErrors()
		ve.Add("language", fmt.Sprintf("Language must be one of: %s", strings.Join(AllowedLanguages(), ", ")))
		return nil, ve
	}

	updated, err := s.store.UpdateAtomic(userID, func(current *UserSettings) {
		if patch.Language != nil {
			current.Language = *patch.Language
		}
		if patch.AutoSave != nil {
			current.AutoSave = *patch.AutoSave
		}
		current.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, sharederrors.NewLocalizedError(
			"usersettings_update_failed",
			"Failed to update user settings",
			"failed to update user settings",
			err,
		)
	}
	return updated, nil
}

// DeleteAllForUser removes userID's saved settings, if any. Called on user delete.
func (s *UserSettingsService) DeleteAllForUser(userID string) error {
	if err := s.store.DeleteAllForUser(userID); err != nil {
		return sharederrors.NewLocalizedError(
			"usersettings_delete_failed",
			"Failed to delete user settings",
			"failed to delete user settings",
			err,
		)
	}
	return nil
}

func (s *UserSettingsService) Close() error {
	return s.store.Close()
}

// PauseForSwap releases the store's OS-level file lock on usersettings.db
// before a live restore renames it. See UserSettingsStore.PauseForSwap.
func (s *UserSettingsService) PauseForSwap() error {
	return s.store.PauseForSwap()
}

// Replace reopens the store against storageDir/usersettings.db after a live
// restore has swapped it in. See UserSettingsStore.Replace.
func (s *UserSettingsService) Replace(storageDir string) error {
	return s.store.Replace(storageDir)
}
