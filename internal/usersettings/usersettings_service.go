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
	Language   *string
	AutoSave   *bool
	DateFormat *string
	TimeFormat *string
}

type UserSettingsService struct {
	store *UserSettingsStore
}

func NewUserSettingsService(store *UserSettingsStore) *UserSettingsService {
	return &UserSettingsService{store: store}
}

// wrapStoreErr propagates err as-is if the store already returned a
// *sharederrors.LocalizedError (e.g. errUserSettingsStoreUnavailable, whose
// code an HTTP caller needs to see in order to map it to 503 — see
// userSettingsErrorStatus) — otherwise wraps it as a generic LocalizedError
// under code/message/template.
func wrapStoreErr(err error, code, message, template string) error {
	if _, ok := sharederrors.AsLocalizedError(err); ok {
		return err
	}
	return sharederrors.NewLocalizedError(code, message, template, err)
}

// Get returns userID's settings, defaulted if the user has never saved any.
func (s *UserSettingsService) Get(userID string) (*UserSettings, error) {
	settings, err := s.store.Get(userID)
	if err != nil {
		return nil, wrapStoreErr(err,
			"usersettings_load_failed",
			"Failed to load user settings",
			"failed to load user settings",
		)
	}
	return settings, nil
}

// Update merges patch's non-nil fields onto userID's current settings and
// saves the result. The read-modify-write happens atomically (UpdateAtomic),
// so two concurrent updates for the same user can't race and silently drop
// one of the two changes.
func (s *UserSettingsService) Update(userID string, patch UserSettingsPatch) (*UserSettings, error) {
	ve := sharederrors.NewValidationErrors()
	if patch.Language != nil && !IsAllowedLanguage(*patch.Language) {
		ve.Add("language", fmt.Sprintf("Language must be one of: %s", strings.Join(AllowedLanguages(), ", ")))
	}
	if patch.DateFormat != nil && !IsAllowedDateFormat(*patch.DateFormat) {
		ve.Add("dateFormat", fmt.Sprintf("Date format must be one of: %s", strings.Join(AllowedDateFormats(), ", ")))
	}
	if patch.TimeFormat != nil && !IsAllowedTimeFormat(*patch.TimeFormat) {
		ve.Add("timeFormat", fmt.Sprintf("Time format must be one of: %s", strings.Join(AllowedTimeFormats(), ", ")))
	}
	if ve.HasErrors() {
		return nil, ve
	}

	updated, err := s.store.UpdateAtomic(userID, func(current *UserSettings) {
		if patch.Language != nil {
			current.Language = *patch.Language
		}
		if patch.AutoSave != nil {
			current.AutoSave = *patch.AutoSave
		}
		if patch.DateFormat != nil {
			current.DateFormat = *patch.DateFormat
		}
		if patch.TimeFormat != nil {
			current.TimeFormat = *patch.TimeFormat
		}
		current.UpdatedAt = time.Now().UTC()
	})
	if err != nil {
		return nil, wrapStoreErr(err,
			"usersettings_update_failed",
			"Failed to update user settings",
			"failed to update user settings",
		)
	}
	return updated, nil
}

// DeleteAllForUser removes userID's saved settings, if any. Called on user delete.
func (s *UserSettingsService) DeleteAllForUser(userID string) error {
	if err := s.store.DeleteAllForUser(userID); err != nil {
		return wrapStoreErr(err,
			"usersettings_delete_failed",
			"Failed to delete user settings",
			"failed to delete user settings",
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
