package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/sharing"
	"github.com/perber/wiki/internal/wiki"
)

// sharesFixture is a wiki with share links on and this tree:
//
//	customer (section)
//	└── contract
//	internal
type sharesFixture struct {
	w        *wiki.Wiki
	router   *gin.Engine
	customer string
	contract string
	internal string
}

func newSharesFixture(t *testing.T) *sharesFixture {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableShareLinks:    true,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	router := newTestRouter(w, nil)

	section := tree.NodeKindSection
	customer := createPageViaAPI(t, router, "Customer", "customer", nil, &section)
	contract := createPageViaAPI(t, router, "Contract", "contract", &customer.ID, nil)
	internal := createPageViaAPI(t, router, "Internal", "internal", nil, nil)
	return &sharesFixture{w: w, router: router, customer: customer.ID, contract: contract.ID, internal: internal.ID}
}

func (f *sharesFixture) link(t *testing.T, pageID string, includeDescendants bool) string {
	t.Helper()
	_, token, err := f.w.ShareLinkService().Create(context.Background(), sharing.CreateShareLinkInput{
		PageID: pageID, IncludeDescendants: includeDescendants, CreatedBy: "editor-1",
	})
	if err != nil {
		t.Fatalf("Create share link: %v", err)
	}
	return token
}

func (f *sharesFixture) get(url, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func TestShares_GetSharedPage_ServesTheLinkScope(t *testing.T) {
	f := newSharesFixture(t)
	subtree := f.link(t, f.customer, true)

	tests := []struct {
		name   string
		url    string
		wantID string
	}{
		{name: "the shared page", url: "/api/shared/" + subtree + "/page", wantID: f.customer},
		{name: "the shared page by id", url: "/api/shared/" + subtree + "/pages/" + f.customer, wantID: f.customer},
		{name: "a subpage of a subtree link", url: "/api/shared/" + subtree + "/pages/" + f.contract, wantID: f.contract},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.get(tc.url, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got["id"] != tc.wantID {
				t.Errorf("id = %v, want %s", got["id"], tc.wantID)
			}
			var keys []string
			for k := range got {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if strings.Join(keys, ",") != "content,id,kind,title,updatedAt" {
				t.Errorf("response fields = %v, want only the allow-listed page view", keys)
			}
			if rec.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Error("shared responses must carry Referrer-Policy: no-referrer")
			}
		})
	}
}

// Every way of not getting a page answers with the same status and body, so
// a link holder can't tell an unknown link from a page outside the link.
func TestShares_GetSharedPage_AnswersUniformNotFound(t *testing.T) {
	f := newSharesFixture(t)
	single := f.link(t, f.customer, false)
	subtree := f.link(t, f.customer, true)
	revoked := f.link(t, f.customer, true)
	if err := revokeByToken(t, f, revoked); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	urls := map[string]string{
		"unknown token":                  "/api/shared/lws_" + strings.Repeat("A", 43) + "/page",
		"malformed token":                "/api/shared/not-a-token/page",
		"revoked link":                   "/api/shared/" + revoked + "/page",
		"subpage of a single-page link":  "/api/shared/" + single + "/pages/" + f.contract,
		"page outside a subtree link":    "/api/shared/" + subtree + "/pages/" + f.internal,
		"unknown page in a subtree link": "/api/shared/" + subtree + "/pages/does-not-exist",
		"tree root through a link":       "/api/shared/" + subtree + "/pages/root",
	}
	var first string
	for name, url := range urls {
		t.Run(name, func(t *testing.T) {
			rec := f.get(url, "")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
			}
			if first == "" {
				first = rec.Body.String()
			}
			if rec.Body.String() != first {
				t.Errorf("body %s differs from %s", rec.Body.String(), first)
			}
			if !strings.Contains(rec.Body.String(), sharing.ErrCodeShareLinkNotFound) {
				t.Errorf("body %s lacks %s", rec.Body.String(), sharing.ErrCodeShareLinkNotFound)
			}
		})
	}
}

// revokeByToken resolves token to its link and revokes it.
func revokeByToken(t *testing.T, f *sharesFixture, token string) error {
	t.Helper()
	grant, err := f.w.ShareLinkService().Resolve(context.Background(), token)
	if err != nil {
		return err
	}
	return f.w.ShareLinkService().Revoke(context.Background(), grant.ShareID)
}

func TestShares_GetSharedPage_StoreUnavailableDuringRestore(t *testing.T) {
	f := newSharesFixture(t)
	token := f.link(t, f.customer, false)
	if err := f.w.ShareLinkService().PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	rec := f.get("/api/shared/"+token+"/page", "")

	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), sharing.ErrCodeShareLinkStoreUnavailable) {
		t.Fatalf("got %d %s, want 503 %s", rec.Code, rec.Body.String(), sharing.ErrCodeShareLinkStoreUnavailable)
	}
}

// Failed lookups are limited per client; a valid link always works, and other
// clients are unaffected.
func TestShares_GetSharedPage_LimitsFailedLookupsPerClient(t *testing.T) {
	f := newSharesFixture(t)
	valid := f.link(t, f.customer, false)
	const attacker, other = "203.0.113.7:1000", "198.51.100.9:1000"
	bad := "/api/shared/not-a-token/page"

	for i := 0; i < 20; i++ {
		if rec := f.get(bad, attacker); rec.Code != http.StatusNotFound {
			t.Fatalf("failed lookup %d = %d, want 404", i+1, rec.Code)
		}
	}
	rec := f.get(bad, attacker)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "share_link_too_many_attempts") {
		t.Fatalf("21st failed lookup = %d %s, want 429 share_link_too_many_attempts", rec.Code, rec.Body.String())
	}
	if rec := f.get(bad, other); rec.Code != http.StatusNotFound {
		t.Errorf("another client's failed lookup = %d, want 404", rec.Code)
	}
	if rec := f.get("/api/shared/"+valid+"/page", attacker); rec.Code != http.StatusOK {
		t.Errorf("a valid link from the throttled client = %d, want 200", rec.Code)
	}
}

func TestShares_Routes_AbsentWhenShareLinksAreOff(t *testing.T) {
	w := createWikiTestInstance(t)
	t.Cleanup(func() { _ = w.Close() })
	router := newTestRouter(w, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/shared/lws_"+strings.Repeat("A", 43)+"/page", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 with share links off", rec.Code)
	}
}
