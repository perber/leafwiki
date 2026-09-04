package usersettings

import (
	"context"

	coreusersettings "github.com/perber/wiki/internal/usersettings"
)

// ─── GetUserSettingsUseCase ──────────────────────────────────────────────────

type GetUserSettingsUseCase struct {
	settings *coreusersettings.UserSettingsService
}

func NewGetUserSettingsUseCase(s *coreusersettings.UserSettingsService) *GetUserSettingsUseCase {
	return &GetUserSettingsUseCase{settings: s}
}

func (uc *GetUserSettingsUseCase) Execute(_ context.Context, userID string) (*coreusersettings.UserSettings, error) {
	return uc.settings.Get(userID)
}

// ─── UpdateUserSettingsUseCase ───────────────────────────────────────────────

type UpdateUserSettingsInput struct {
	UserID     string
	Language   *string
	AutoSave   *bool
	DateFormat *string
	TimeFormat *string
}

type UpdateUserSettingsUseCase struct {
	settings *coreusersettings.UserSettingsService
}

func NewUpdateUserSettingsUseCase(s *coreusersettings.UserSettingsService) *UpdateUserSettingsUseCase {
	return &UpdateUserSettingsUseCase{settings: s}
}

func (uc *UpdateUserSettingsUseCase) Execute(_ context.Context, in UpdateUserSettingsInput) (*coreusersettings.UserSettings, error) {
	return uc.settings.Update(in.UserID, coreusersettings.UserSettingsPatch{
		Language:   in.Language,
		AutoSave:   in.AutoSave,
		DateFormat: in.DateFormat,
		TimeFormat: in.TimeFormat,
	})
}
