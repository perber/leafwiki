package usersettings

import (
	"errors"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func newTestService(t *testing.T) *UserSettingsService {
	t.Helper()
	return NewUserSettingsService(newTestStore(t))
}

func TestUserSettingsService_Get_NoRow_ReturnsDefaults(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Language != DefaultLanguage || got.AutoSave != true {
		t.Fatalf("expected defaults, got %+v", got)
	}
}

func TestUserSettingsService_Update_ValidLanguageAndAutoSave_Persists(t *testing.T) {
	svc := newTestService(t)

	lang := "en"
	autoSave := false
	updated, err := svc.Update("user-1", UserSettingsPatch{Language: &lang, AutoSave: &autoSave})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Language != lang || updated.AutoSave != autoSave {
		t.Fatalf("expected Language=%q AutoSave=%v, got %+v", lang, autoSave, updated)
	}

	got, err := svc.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Language != lang || got.AutoSave != autoSave {
		t.Fatalf("expected persisted Language=%q AutoSave=%v, got %+v", lang, autoSave, got)
	}
}

// TestUserSettingsService_Update_PartialPatch_LeavesOtherFieldUnchanged pins
// the partial-patch semantics: updating one field must never reset another.
func TestUserSettingsService_Update_PartialPatch_LeavesOtherFieldUnchanged(t *testing.T) {
	svc := newTestService(t)

	lang := "en"
	if _, err := svc.Update("user-1", UserSettingsPatch{Language: &lang}); err != nil {
		t.Fatalf("Update (language): %v", err)
	}

	autoSave := false
	updated, err := svc.Update("user-1", UserSettingsPatch{AutoSave: &autoSave})
	if err != nil {
		t.Fatalf("Update (autosave): %v", err)
	}

	if updated.Language != lang {
		t.Fatalf("expected language to remain %q after an autosave-only patch, got %q", lang, updated.Language)
	}
	if updated.AutoSave != autoSave {
		t.Fatalf("expected autosave=%v, got %v", autoSave, updated.AutoSave)
	}
}

func TestUserSettingsService_Update_InvalidLanguage_ReturnsValidationError(t *testing.T) {
	svc := newTestService(t)

	lang := "xx-not-a-real-language"
	_, err := svc.Update("user-1", UserSettingsPatch{Language: &lang})
	if err == nil {
		t.Fatalf("expected an error for an unsupported language, got nil")
	}

	var vErr *sharederrors.ValidationErrors
	if !errors.As(err, &vErr) {
		t.Fatalf("expected a *sharederrors.ValidationErrors, got: %v (%T)", err, err)
	}
	found := false
	for _, fe := range vErr.Errors {
		if fe.Field == "language" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a validation error on the language field, got: %+v", vErr.Errors)
	}
}
