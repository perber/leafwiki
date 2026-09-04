package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/test_utils"
)

type apiUserSettings struct {
	Language   string `json:"language"`
	AutoSave   bool   `json:"autoSave"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
}

func TestUserSettings_Get_Unauthenticated_Rejected(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	req := httptest.NewRequest(http.MethodGet, "/api/user-settings", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated GET, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserSettings_Put_Unauthenticated_Rejected(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	req := httptest.NewRequest(http.MethodPut, "/api/user-settings", strings.NewReader(`{"autoSave":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated PUT, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserSettings_Get_Authenticated_NoPriorSettings_ReturnsDefaults(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/user-settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got apiUserSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Language != "en" || !got.AutoSave {
		t.Fatalf("expected default settings, got %+v", got)
	}
}

func TestUserSettings_Put_ValidPartialPatch_UpdatesAndPersists(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	putRec := authenticatedRequest(t, router, http.MethodPut, "/api/user-settings", strings.NewReader(`{"autoSave":false}`))
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d: %s", putRec.Code, putRec.Body.String())
	}

	var updated apiUserSettings
	if err := json.Unmarshal(putRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal PUT response: %v", err)
	}
	if updated.AutoSave {
		t.Fatalf("expected AutoSave=false in PUT response, got %+v", updated)
	}
	if updated.Language != "en" {
		t.Fatalf("expected Language to remain the default \"en\", got %q", updated.Language)
	}

	getRec := authenticatedRequest(t, router, http.MethodGet, "/api/user-settings", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var got apiUserSettings
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal GET response: %v", err)
	}
	if got.AutoSave || got.Language != "en" {
		t.Fatalf("expected persisted AutoSave=false and Language=\"en\", got %+v", got)
	}
}

func TestUserSettings_Put_InvalidLanguage_Returns400(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/user-settings", strings.NewReader(`{"language":"xx-not-real"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unsupported language, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserSettings_Put_DateAndTimeFormat_UpdatesAndPersists(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	putRec := authenticatedRequest(t, router, http.MethodPut, "/api/user-settings", strings.NewReader(`{"dateFormat":"dmy_dot","timeFormat":"24h"}`))
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d: %s", putRec.Code, putRec.Body.String())
	}
	var updated apiUserSettings
	if err := json.Unmarshal(putRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal PUT response: %v", err)
	}
	if updated.DateFormat != "dmy_dot" || updated.TimeFormat != "24h" {
		t.Fatalf("expected dmy_dot/24h in PUT response, got %+v", updated)
	}

	getRec := authenticatedRequest(t, router, http.MethodGet, "/api/user-settings", nil)
	var got apiUserSettings
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal GET response: %v", err)
	}
	if got.DateFormat != "dmy_dot" || got.TimeFormat != "24h" {
		t.Fatalf("expected persisted dmy_dot/24h, got %+v", got)
	}
}

func TestUserSettings_Get_NoPriorSettings_ReturnsLocaleFormats(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodGet, "/api/user-settings", nil)
	var got apiUserSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal GET response: %v", err)
	}
	if got.DateFormat != "locale" || got.TimeFormat != "locale" {
		t.Fatalf("expected locale/locale defaults, got %+v", got)
	}
}

func TestUserSettings_Put_InvalidDateFormat_Returns400(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	rec := authenticatedRequest(t, router, http.MethodPut, "/api/user-settings", strings.NewReader(`{"dateFormat":"dd.MM.yyyy"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unsupported date format, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserSettings_Get_IsPerUser_DoesNotLeakOtherUsersSettings(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	if _, err := w.UserService().CreateUser("second-user", "second-user@example.com", "password123", coreauth.RoleEditor); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Admin turns autosave off for themself.
	putRec := authenticatedRequest(t, router, http.MethodPut, "/api/user-settings", strings.NewReader(`{"autoSave":false}`))
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on PUT, got %d: %s", putRec.Code, putRec.Body.String())
	}

	// The second user must still see their own defaults.
	getRec := authenticatedRequestAs(t, router, "second-user", "password123", http.MethodGet, "/api/user-settings", nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
	var got apiUserSettings
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.AutoSave {
		t.Fatalf("expected second user's AutoSave to remain the default true, got %+v", got)
	}
}
