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

// TestUserSettingsService_Get_StoreUnavailable_PropagatesOriginalErrorCode
// is the regression test for a real bug found in review: the service layer
// used to rewrap every store error into a generic usersettings_load_failed
// code, losing errUserSettingsStoreUnavailable's ErrCodeUserSettingsStoreUnavailable
// — which an HTTP caller needs to see in order to map the response to 503
// instead of a generic 500 (see internal/wiki/usersettings/errors.go).
func TestUserSettingsService_Get_StoreUnavailable_PropagatesOriginalErrorCode(t *testing.T) {
	store := newTestStore(t)
	svc := NewUserSettingsService(store)

	if err := store.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	_, err := svc.Get("user-1")
	if err == nil {
		t.Fatal("expected an error while the store is suspended")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected a *sharederrors.LocalizedError, got %T: %v", err, err)
	}
	if loc.Code != ErrCodeUserSettingsStoreUnavailable {
		t.Fatalf("expected code %q to survive service-layer wrapping, got %q", ErrCodeUserSettingsStoreUnavailable, loc.Code)
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

func TestUserSettingsService_Update_ValidFormats_Persist(t *testing.T) {
	svc := newTestService(t)

	dateFormat := "dmy_dot"
	timeFormat := "24h"
	updated, err := svc.Update("user-1", UserSettingsPatch{
		DateFormat: &dateFormat,
		TimeFormat: &timeFormat,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DateFormat != dateFormat || updated.TimeFormat != timeFormat {
		t.Fatalf("expected DateFormat=%q TimeFormat=%q, got %+v", dateFormat, timeFormat, updated)
	}

	got, err := svc.Get("user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DateFormat != dateFormat || got.TimeFormat != timeFormat {
		t.Fatalf("expected persisted formats, got %+v", got)
	}
	// A format-only patch must not touch the language default.
	if got.Language != DefaultLanguage {
		t.Fatalf("expected language untouched, got %+v", got)
	}
}

func TestUserSettingsService_Update_InvalidFormats_ReturnValidationErrors(t *testing.T) {
	svc := newTestService(t)

	badDate := "dd.MM.yyyy"
	badTime := "military"
	_, err := svc.Update("user-1", UserSettingsPatch{
		DateFormat: &badDate,
		TimeFormat: &badTime,
	})
	if err == nil {
		t.Fatalf("expected a validation error, got nil")
	}

	var vErr *sharederrors.ValidationErrors
	if !errors.As(err, &vErr) {
		t.Fatalf("expected a *sharederrors.ValidationErrors, got: %v (%T)", err, err)
	}
	fields := map[string]bool{}
	for _, fe := range vErr.Errors {
		fields[fe.Field] = true
	}
	if !fields["dateFormat"] || !fields["timeFormat"] {
		t.Fatalf("expected validation errors on dateFormat and timeFormat, got: %+v", vErr.Errors)
	}
}
