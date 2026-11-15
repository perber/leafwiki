package http_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/pagedisplay"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
	wikiinstancesettings "github.com/perber/wiki/internal/wiki/instancesettings"
)

const displaySettingsURL = "/api/admin/settings/display"

// newDisplaySettingsRouter is the fixture for the display-settings routes: a
// test router whose instance-settings registrar and /api/config share one
// pagedisplay.Service.
func newDisplaySettingsRouter(t *testing.T) (*gin.Engine, *pagedisplay.Service) {
	t.Helper()
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	svc, err := pagedisplay.New(t.TempDir())
	if err != nil {
		t.Fatalf("pagedisplay.New: %v", err)
	}
	return routerWithTocDisplay(t, w, svc), svc
}

// routerWithDisplayAPIKeys builds a wiki with API-key management enabled and
// the instance-settings routes backed by a fresh pagedisplay.Service.
func routerWithDisplayAPIKeys(t *testing.T) (*wiki.Wiki, *gin.Engine, *pagedisplay.Service) {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableAPIKeyManagement: true,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	svc, err := pagedisplay.New(t.TempDir())
	if err != nil {
		t.Fatalf("pagedisplay.New: %v", err)
	}
	paSvc, err := publicaccess.NewSettingsManaged(t.TempDir())
	if err != nil {
		t.Fatalf("publicaccess.NewSettingsManaged: %v", err)
	}
	w.SetInstanceSettingsRoutes(wikiinstancesettings.NewRoutes(paSvc, svc, w.AuthService(), slog.Default()))
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) {
		o.APIKeyService = w.APIKeyService()
		o.EnableAPIKeyManagement = true
		o.PublicAccess = paSvc
		o.PageDisplay = svc
	})
	return w, router, svc
}

func TestDisplaySettings_Put_AppliesGivenFieldsAndReturnsAllSettings(t *testing.T) {
	router, svc := newDisplaySettingsRouter(t)

	steps := []struct {
		name string
		body string
		want string
	}{
		{name: "all fields", body: `{"alwaysShowToc":true,"contentWidth":"wide","showSubpageCount":true}`, want: `{"alwaysShowToc":true,"contentWidth":"wide","showSubpageCount":true}`},
		{name: "only alwaysShowToc keeps the others", body: `{"alwaysShowToc":false}`, want: `{"alwaysShowToc":false,"contentWidth":"wide","showSubpageCount":true}`},
		{name: "only contentWidth keeps the others", body: `{"contentWidth":"default"}`, want: `{"alwaysShowToc":false,"contentWidth":"default","showSubpageCount":true}`},
		{name: "only showSubpageCount keeps the others", body: `{"showSubpageCount":false}`, want: `{"alwaysShowToc":false,"contentWidth":"default","showSubpageCount":false}`},
		{name: "null field is left untouched", body: `{"alwaysShowToc":true,"contentWidth":null}`, want: `{"alwaysShowToc":true,"contentWidth":"default","showSubpageCount":false}`},
	}
	for _, step := range steps {
		rec := authenticatedRequest(t, router, http.MethodPut, displaySettingsURL, strings.NewReader(step.body))
		if rec.Code != http.StatusOK || rec.Body.String() != step.want {
			t.Fatalf("%s: want 200 %s, got %d %s", step.name, step.want, rec.Code, rec.Body.String())
		}
		cfg := readConfigJSON(t, router)
		for key, want := range svc.Settings().ConfigFields() {
			if cfg[key] != want {
				t.Fatalf("%s: /api/config[%s] = %v, want %v", step.name, key, cfg[key], want)
			}
		}
	}
}

func TestDisplaySettings_Put_RequiresAdmin(t *testing.T) {
	router, svc := newDisplaySettingsRouter(t)

	if code := anonStatus(router, http.MethodPut, displaySettingsURL); code != http.StatusUnauthorized {
		t.Fatalf("anon PUT: want 401, got %d", code)
	}

	authenticatedRequest(t, router, http.MethodPost, "/api/users",
		strings.NewReader(`{"username":"ed4","email":"ed4@example.com","password":"editorpass","role":"editor"}`))
	rec := authenticatedRequestAs(t, router, "ed4", "editorpass", http.MethodPut, displaySettingsURL, strings.NewReader(`{"alwaysShowToc":true,"contentWidth":"wide"}`))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("editor PUT: want 403, got %d %s", rec.Code, rec.Body.String())
	}
	if got, want := svc.Settings(), (pagedisplay.Settings{ContentWidth: pagedisplay.ContentWidthDefault}); got != want {
		t.Fatalf("settings changed by a rejected request: %+v", got)
	}
}

func TestDisplaySettings_Put_BadPayload_Returns400AndChangesNothing(t *testing.T) {
	router, svc := newDisplaySettingsRouter(t)

	const invalidPayload = "instance_settings_invalid_payload"
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{name: "empty object", body: `{}`, wantCode: invalidPayload},
		{name: "all fields null", body: `{"alwaysShowToc":null,"contentWidth":null,"showSubpageCount":null}`, wantCode: invalidPayload},
		{name: "empty body", body: ``, wantCode: invalidPayload},
		{name: "json null", body: `null`, wantCode: invalidPayload},
		{name: "not json", body: `not json`, wantCode: invalidPayload},
		{name: "array", body: `[]`, wantCode: invalidPayload},
		{name: "truncated object", body: `{"alwaysShowToc":true`, wantCode: invalidPayload},
		{name: "trailing second value", body: `{"alwaysShowToc":true}{"contentWidth":"wide"}`, wantCode: invalidPayload},
		{name: "trailing garbage", body: `{"alwaysShowToc":true} x`, wantCode: invalidPayload},
		{name: "bool as string", body: `{"alwaysShowToc":"yes"}`, wantCode: invalidPayload},
		{name: "width as number", body: `{"contentWidth":1}`, wantCode: invalidPayload},
		{name: "subpage count as string", body: `{"showSubpageCount":"true"}`, wantCode: invalidPayload},
		{name: "subpage count as number", body: `{"showSubpageCount":1}`, wantCode: invalidPayload},
		{name: "width as array", body: `{"contentWidth":["wide"]}`, wantCode: invalidPayload},
		{name: "old toc-display field name", body: `{"alwaysShow":true}`, wantCode: invalidPayload},
		{name: "unknown field next to a valid one", body: `{"alwaysShowToc":true,"futureSetting":1}`, wantCode: invalidPayload},
		// The route reads at most 4 KiB: a valid patch padded past that, and a
		// huge trailing value, are rejected without being buffered.
		{name: "valid patch over 4 KiB", body: `{"alwaysShowToc":true}` + strings.Repeat(" ", 4<<10), wantCode: invalidPayload},
		{name: "huge trailing value", body: `{"alwaysShowToc":true}"` + strings.Repeat("x", 64<<10) + `"`, wantCode: invalidPayload},
		{name: "unknown width", body: `{"contentWidth":"huge"}`, wantCode: pagedisplay.ErrCodeInvalidContentWidth},
		{name: "unknown width next to a valid flag", body: `{"alwaysShowToc":true,"contentWidth":"huge"}`, wantCode: pagedisplay.ErrCodeInvalidContentWidth},
		{name: "unknown width next to a valid subpage count", body: `{"showSubpageCount":true,"contentWidth":"huge"}`, wantCode: pagedisplay.ErrCodeInvalidContentWidth},
		{name: "empty width", body: `{"contentWidth":""}`, wantCode: pagedisplay.ErrCodeInvalidContentWidth},
		{name: "html in width", body: `{"contentWidth":"<script>alert(1)</script>"}`, wantCode: pagedisplay.ErrCodeInvalidContentWidth},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := authenticatedRequest(t, router, http.MethodPut, displaySettingsURL, strings.NewReader(tc.body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("payload %.80q: want 400, got %d %s", tc.body, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"code":"`+tc.wantCode+`"`) {
				t.Fatalf("payload %.80q: want error code %q, got %s", tc.body, tc.wantCode, rec.Body.String())
			}
		})
	}
	if got, want := svc.Settings(), (pagedisplay.Settings{ContentWidth: pagedisplay.ContentWidthDefault}); got != want {
		t.Fatalf("settings changed by rejected requests: %+v", got)
	}
}

// Anonymous visitors see the subpage count too: /api/config carries the
// setting whether public access is on or off.
func TestDisplaySettings_ShowSubpageCount_ReachesAnonymousConfig(t *testing.T) {
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	svc, err := pagedisplay.New(t.TempDir())
	if err != nil {
		t.Fatalf("pagedisplay.New: %v", err)
	}
	for _, public := range []bool{false, true} {
		t.Run(fmt.Sprintf("public=%v", public), func(t *testing.T) {
			router := newTestRouter(w, func(o *httpinternal.RouterOptions) {
				o.PublicAccess = publicaccess.Fixed(public)
				o.PageDisplay = svc
			})
			if cfg := readConfigJSON(t, router); cfg["showSubpageCount"] != false {
				t.Fatalf("default config: want showSubpageCount=false, got %v", cfg["showSubpageCount"])
			}
			on, off := true, false
			if _, err := svc.Update(pagedisplay.Patch{ShowSubpageCount: &on}); err != nil {
				t.Fatalf("Update: %v", err)
			}
			t.Cleanup(func() { _, _ = svc.Update(pagedisplay.Patch{ShowSubpageCount: &off}) })
			if cfg := readConfigJSON(t, router); cfg["showSubpageCount"] != true || cfg["publicAccess"] != public {
				t.Fatalf("anonymous config: want showSubpageCount=true publicAccess=%v, got %v / %v", public, cfg["showSubpageCount"], cfg["publicAccess"])
			}
		})
	}
}

// Bearer API keys follow role(user) ∩ role(key): only an admin-scoped key
// owned by an admin may change the display settings, exactly like the
// per-setting routes next to it.
func TestDisplaySettings_Put_APIKey_OnlyAdminKeyOfAdminMayChange(t *testing.T) {
	w, router, svc := routerWithDisplayAPIKeys(t)

	tests := []struct {
		name     string
		userRole string
		keyRole  string
		want     int
	}{
		{name: "viewer key of a viewer", userRole: coreauth.RoleViewer, keyRole: coreauth.RoleViewer, want: http.StatusForbidden},
		{name: "editor key of an editor", userRole: coreauth.RoleEditor, keyRole: coreauth.RoleEditor, want: http.StatusForbidden},
		{name: "viewer key of an admin", userRole: coreauth.RoleAdmin, keyRole: coreauth.RoleViewer, want: http.StatusForbidden},
		{name: "editor key of an admin", userRole: coreauth.RoleAdmin, keyRole: coreauth.RoleEditor, want: http.StatusForbidden},
		{name: "admin key of an admin", userRole: coreauth.RoleAdmin, keyRole: coreauth.RoleAdmin, want: http.StatusOK},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			secret := createAPIKeyFor(t, w, router, fmt.Sprintf("dskey%d", i), tc.userRole, tc.keyRole)

			widthRec := httptest.NewRecorder()
			router.ServeHTTP(widthRec, bearerJSONRequest(http.MethodPut, "/api/admin/settings/content-width", secret, `{"contentWidth":"default"}`))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, bearerJSONRequest(http.MethodPut, displaySettingsURL, secret, `{"contentWidth":"wide"}`))

			if rec.Code != tc.want {
				t.Fatalf("display: want %d, got %d %s", tc.want, rec.Code, rec.Body.String())
			}
			if rec.Code != widthRec.Code {
				t.Fatalf("display answered %d, content-width %d for the same key", rec.Code, widthRec.Code)
			}
			wantWidth := pagedisplay.ContentWidthDefault
			if tc.want == http.StatusOK {
				wantWidth = pagedisplay.ContentWidthWide
			}
			if got := svc.Settings().ContentWidth; got != wantWidth {
				t.Fatalf("content width after request = %q, want %q", got, wantWidth)
			}
		})
	}
}

// Two admins saving at the same time, one through the new display route and
// one through the older per-setting routes, must not drop each other's
// change: every route goes through the same serialized read-modify-write.
func TestDisplaySettings_Put_ConcurrentWithPerSettingRoutes_KeepsBothChanges(t *testing.T) {
	router, svc := newDisplaySettingsRouter(t)
	newClient := loginSession(t, router, "admin", "adminpassword")
	oldClient := loginSession(t, router, "admin", "adminpassword")

	const rounds = 40
	var wg sync.WaitGroup
	errs := make(chan string, 2*rounds)
	for i := 0; i < rounds; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if rec := newClient.do(router, http.MethodPut, displaySettingsURL, `{"alwaysShowToc":true}`); rec.Code != http.StatusOK {
				errs <- fmt.Sprintf("display PUT: %d %s", rec.Code, rec.Body.String())
			}
		}()
		go func() {
			defer wg.Done()
			if rec := oldClient.do(router, http.MethodPut, "/api/admin/settings/content-width", `{"contentWidth":"wide"}`); rec.Code != http.StatusOK {
				errs <- fmt.Sprintf("content-width PUT: %d %s", rec.Code, rec.Body.String())
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	want := pagedisplay.Settings{AlwaysShowToc: true, ContentWidth: pagedisplay.ContentWidthWide}
	if got := svc.Settings(); got != want {
		t.Fatalf("settings after concurrent saves = %+v, want %+v", got, want)
	}
}

// The old toc-display route and the new display route write the same field:
// a change through one is visible in the other's reply.
func TestDisplaySettings_Put_SeesChangesFromPerSettingRoutes(t *testing.T) {
	router, _ := newDisplaySettingsRouter(t)
	sess := loginSession(t, router, "admin", "adminpassword")

	steps := []struct {
		name, url, body, want string
	}{
		{name: "old toc route on", url: "/api/admin/settings/toc-display", body: `{"alwaysShow":true}`, want: `{"alwaysShow":true}`},
		{name: "new route changes width only", url: displaySettingsURL, body: `{"contentWidth":"wide"}`, want: `{"alwaysShowToc":true,"contentWidth":"wide","showSubpageCount":false}`},
		{name: "old width route back to default", url: "/api/admin/settings/content-width", body: `{"contentWidth":"default"}`, want: `{"contentWidth":"default"}`},
		{name: "new route turns toc off", url: displaySettingsURL, body: `{"alwaysShowToc":false}`, want: `{"alwaysShowToc":false,"contentWidth":"default","showSubpageCount":false}`},
	}
	for _, step := range steps {
		rec := sess.do(router, http.MethodPut, step.url, step.body)
		if rec.Code != http.StatusOK || rec.Body.String() != step.want {
			t.Fatalf("%s: want 200 %s, got %d %s", step.name, step.want, rec.Code, rec.Body.String())
		}
	}
}
