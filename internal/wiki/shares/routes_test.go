package shares

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
	"github.com/perber/wiki/internal/scopedread"
	"github.com/perber/wiki/internal/sharing"
)

type fakeGetter struct {
	out *GetSharedPageOutput
	err error
}

func (f fakeGetter) Execute(context.Context, GetSharedPageInput) (*GetSharedPageOutput, error) {
	return f.out, f.err
}

func serveShared(t *testing.T, getter sharedPageGetter) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r := &Routes{getPage: getter, limiter: newFailedTokenLimiter(), log: slog.Default()}
	engine.GET("/api/shared/:token/page", r.handleGetSharedRootPage)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/shared/lws_token/page", nil))
	return rec
}

// Status and body per error class. A non-localized error is answered with a
// generic 500 that never carries the cause, because the caller is anonymous
// and causes can contain file system paths.
func TestRoutes_HandleGetSharedPage_MapsErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantBody string
	}{
		{name: "not found", err: sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkNotFound, "x", "x", nil), wantCode: http.StatusNotFound, wantBody: sharing.ErrCodeShareLinkNotFound},
		{name: "store unavailable", err: sharederrors.NewLocalizedError(sharing.ErrCodeShareLinkStoreUnavailable, "x", "x", nil), wantCode: http.StatusServiceUnavailable, wantBody: sharing.ErrCodeShareLinkStoreUnavailable},
		{name: "operational error", err: errors.New("read /data/root/secret.md: permission denied"), wantCode: http.StatusInternalServerError, wantBody: ErrCodeShareLinkInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveShared(t, fakeGetter{err: tc.err})

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			var body dto.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode %s: %v", rec.Body.String(), err)
			}
			if body.Error.Code != tc.wantBody {
				t.Errorf("code = %q, want %q", body.Error.Code, tc.wantBody)
			}
			if strings.Contains(rec.Body.String(), "/data/") || strings.Contains(rec.Body.String(), "secret.md") {
				t.Errorf("response leaks the cause: %s", rec.Body.String())
			}
		})
	}
}

func TestRoutes_HandleGetSharedPage_ReturnsThePageView(t *testing.T) {
	rec := serveShared(t, fakeGetter{out: &GetSharedPageOutput{Page: &scopedread.PageView{ID: "customer", Title: "Customer", Kind: "section"}}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["id"] != "customer" || got["title"] != "Customer" {
		t.Errorf("body = %v", got)
	}
}

type fakeTreeGetter struct {
	out *GetSharedTreeOutput
	err error
}

func (f fakeTreeGetter) Execute(context.Context, GetSharedTreeInput) (*GetSharedTreeOutput, error) {
	return f.out, f.err
}

func serveSharedTree(t *testing.T, getter sharedTreeGetter) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r := &Routes{getTree: getter, limiter: newFailedTokenLimiter(), log: slog.Default()}
	engine.GET("/api/shared/:token/tree", r.handleGetSharedTree)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/shared/lws_token/tree", nil))
	return rec
}

func TestRoutes_HandleGetSharedTree_ReturnsTheTreeView(t *testing.T) {
	view := &scopedread.TreeNodeView{ID: "customer", Title: "Customer", Kind: "section", Children: []*scopedread.TreeNodeView{
		{ID: "contract", Title: "Contract", Kind: "page", Children: []*scopedread.TreeNodeView{}},
	}}
	rec := serveSharedTree(t, fakeTreeGetter{out: &GetSharedTreeOutput{Tree: view}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	want := `{"id":"customer","title":"Customer","kind":"section","children":[{"id":"contract","title":"Contract","kind":"page","children":[]}]}`
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body.String(), want)
	}
}

func TestRoutes_HandleGetSharedTree_AnswersErrorsGenerically(t *testing.T) {
	rec := serveSharedTree(t, fakeTreeGetter{err: errors.New("read /data/root: permission denied")})

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), ErrCodeShareLinkInternal) {
		t.Fatalf("got %d %s, want 500 %s", rec.Code, rec.Body.String(), ErrCodeShareLinkInternal)
	}
	if strings.Contains(rec.Body.String(), "/data/") {
		t.Errorf("response leaks the cause: %s", rec.Body.String())
	}
}
