package usersettings

import (
	"context"
	"testing"

	coreusersettings "github.com/perber/wiki/internal/usersettings"
)

func setupUserSettingsUseCases(t *testing.T) (*GetUserSettingsUseCase, *UpdateUserSettingsUseCase) {
	t.Helper()

	store, err := coreusersettings.NewUserSettingsStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close user settings store: %v", err)
		}
	})
	svc := coreusersettings.NewUserSettingsService(store)

	return NewGetUserSettingsUseCase(svc), NewUpdateUserSettingsUseCase(svc)
}

func TestGetUserSettings_NoPriorSettings_ReturnsDefaults(t *testing.T) {
	get, _ := setupUserSettingsUseCases(t)

	out, err := get.Execute(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Language != coreusersettings.DefaultLanguage || !out.AutoSave {
		t.Fatalf("expected defaults, got %+v", out)
	}
}

func TestUpdateUserSettings_ValidPatch_Succeeds(t *testing.T) {
	get, update := setupUserSettingsUseCases(t)

	autoSave := false
	if _, err := update.Execute(context.Background(), UpdateUserSettingsInput{UserID: "user-1", AutoSave: &autoSave}); err != nil {
		t.Fatalf("Execute (update): %v", err)
	}

	out, err := get.Execute(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Execute (get): %v", err)
	}
	if out.AutoSave != false {
		t.Fatalf("expected AutoSave=false, got %v", out.AutoSave)
	}
}

func TestUpdateUserSettings_InvalidLanguage_Rejected(t *testing.T) {
	_, update := setupUserSettingsUseCases(t)

	lang := "not-a-real-language"
	if _, err := update.Execute(context.Background(), UpdateUserSettingsInput{UserID: "user-1", Language: &lang}); err == nil {
		t.Fatalf("expected an error for an unsupported language, got nil")
	}
}
