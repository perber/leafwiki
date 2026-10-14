package http_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreavatar "github.com/perber/wiki/internal/avatar"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/wiki"
)

// newAvatarRouterTest builds a fresh Wiki + router and returns the admin user's ID,
// which is whose avatar the admin session uploads and deletes.
func newAvatarRouterTest(t *testing.T) (*wiki.Wiki, http.Handler, string) {
	t.Helper()
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	admin, err := w.UserService().GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(admin): %v", err)
	}
	return w, createRouterTestInstance(w, t), admin.ID
}

func avatarTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// multipartBody builds a multipart form with one file part (field/filename/content);
// an empty field produces a form that carries only a non-file field.
func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	if field == "" {
		if err := mw.WriteField("note", "no file here"); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	} else {
		part, err := mw.CreateFormFile(field, filename)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("part.Write: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart Close: %v", err)
	}
	return body, mw.FormDataContentType()
}

// sendAvatarRequest issues method on path; with withSession it logs in as admin and
// attaches the session cookies and CSRF token.
func sendAvatarRequest(t *testing.T, router http.Handler, withSession bool, method, path string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if withSession {
		sess := loginSession(t, router, "admin", "adminpassword")
		for _, c := range sess.cookies {
			req.AddCookie(c)
		}
		req.Header.Set("X-CSRF-Token", sess.csrf)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func avatarErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return resp.Error.Code
}

func TestAvatarRoutes_UploadRejections_ReturnStatusAndCode(t *testing.T) {
	tooLarge := make([]byte, coreavatar.MaxUploadSize+1024)

	tests := []struct {
		name     string
		field    string
		filename string
		content  []byte
		want     int
		wantCode string
	}{
		{name: "form without a file part", field: "", want: http.StatusBadRequest, wantCode: "avatar_upload_failed"},
		{name: "file under the wrong field name", field: "avatar", filename: "a.png", content: []byte("x"), want: http.StatusBadRequest, wantCode: "avatar_upload_failed"},
		{name: "upload larger than MaxUploadSize", field: "file", filename: "a.png", content: tooLarge, want: http.StatusRequestEntityTooLarge, wantCode: "avatar_upload_failed"},
		{name: "disallowed extension", field: "file", filename: "a.bmp", content: nil, want: http.StatusBadRequest, wantCode: "avatar_invalid_type"},
		{name: "png extension with undecodable bytes", field: "file", filename: "a.png", content: []byte("not an image"), want: http.StatusBadRequest, wantCode: "avatar_decode_failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, router, adminID := newAvatarRouterTest(t)
			content := tc.content
			if content == nil {
				content = avatarTestPNG(t)
			}
			body, ct := multipartBody(t, tc.field, tc.filename, content)

			rec := sendAvatarRequest(t, router, true, http.MethodPost, "/api/user/avatar", body, ct)

			if rec.Code != tc.want {
				t.Fatalf("POST /api/user/avatar: got %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if got := avatarErrorCode(t, rec); got != tc.wantCode {
				t.Errorf("error code: got %q, want %q", got, tc.wantCode)
			}
			if get := sendAvatarRequest(t, router, false, http.MethodGet, "/avatars/"+adminID, nil, ""); get.Code != http.StatusNotFound {
				t.Errorf("GET avatar after rejected upload: got %d, want 404", get.Code)
			}
		})
	}
}

func TestAvatarRoutes_Unauthenticated_WritesRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "upload", method: http.MethodPost},
		{name: "delete", method: http.MethodDelete},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, router, _ := newAvatarRouterTest(t)
			body, ct := multipartBody(t, "file", "a.png", avatarTestPNG(t))

			rec := sendAvatarRequest(t, router, false, tc.method, "/api/user/avatar", body, ct)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s /api/user/avatar without session: got %d, want 401", tc.method, rec.Code)
			}
		})
	}
}

func TestAvatarRoutes_UploadServeDelete_RoundTrip(t *testing.T) {
	_, router, adminID := newAvatarRouterTest(t)
	avatarURL := "/avatars/" + adminID

	body, ct := multipartBody(t, "file", "me.png", avatarTestPNG(t))
	if rec := sendAvatarRequest(t, router, true, http.MethodPost, "/api/user/avatar", body, ct); rec.Code != http.StatusOK {
		t.Fatalf("upload: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	served := sendAvatarRequest(t, router, false, http.MethodGet, avatarURL, nil, "")
	if served.Code != http.StatusOK {
		t.Fatalf("serve after upload: got %d, want 200", served.Code)
	}
	if got := served.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type: got %q, want image/png", got)
	}
	if got := served.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q, want no-store", got)
	}
	if _, err := png.Decode(bytes.NewReader(served.Body.Bytes())); err != nil {
		t.Errorf("served avatar is not a PNG: %v", err)
	}

	if rec := sendAvatarRequest(t, router, true, http.MethodDelete, "/api/user/avatar", nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("delete: got %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if rec := sendAvatarRequest(t, router, false, http.MethodGet, avatarURL, nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("serve after delete: got %d, want 404", rec.Code)
	}
	if rec := sendAvatarRequest(t, router, true, http.MethodDelete, "/api/user/avatar", nil, ""); rec.Code != http.StatusOK {
		t.Errorf("second delete (nothing left): got %d, want 200", rec.Code)
	}
}

func TestAvatarRoutes_Serve_UnknownOrUnsafeID_Returns404(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "unknown user", path: "/avatars/no-such-user"},
		{name: "dot-dot", path: "/avatars/.."},
		{name: "encoded slash traversal", path: "/avatars/..%2F..%2Fusers.db"},
		{name: "encoded backslash", path: "/avatars/a%5Cb"},
		{name: "unicode id", path: "/avatars/" + strings.Repeat("ü", 3)},
	}
	_, router, _ := newAvatarRouterTest(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := sendAvatarRequest(t, router, false, http.MethodGet, tc.path, nil, "")
			if rec.Code != http.StatusNotFound {
				t.Errorf("GET %s: got %d, want 404", tc.path, rec.Code)
			}
		})
	}
}
