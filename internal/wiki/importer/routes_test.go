package importer

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
)

// TestHandleCreatePlan_LargeUploadIsSpooledToDisk pins that the import
// upload is parsed with a small in-memory threshold: a part larger than that
// threshold must end up in a temp file rather than in RAM, so concurrent
// uploads cannot each pin up to the full upload cap in memory. The part is
// sent under a field other than "file", so the handler stops right after
// parsing with 400 importer_missing_file and needs no use case.
func TestHandleCreatePlan_LargeUploadIsSpooledToDisk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Just above the in-memory threshold, far below the 500 MiB upload cap.
	const partSize = httpinternal.MultipartMaxMemory + 4<<20

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("attachment", "big.zip")
	if err != nil {
		t.Fatalf("CreateFormFile() error: %v", err)
	}
	if _, err := fw.Write(make([]byte, partSize)); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/import/plan", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("user", &coreauth.User{ID: "editor-1"})

	routes := &Routes{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	routes.handleCreatePlan(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (missing file); body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if req.MultipartForm == nil {
		t.Fatal("handler did not parse the multipart form")
	}
	t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })

	parts := req.MultipartForm.File["attachment"]
	if len(parts) != 1 {
		t.Fatalf("got %d parts named attachment, want 1", len(parts))
	}
	f, err := parts[0].Open()
	if err != nil {
		t.Fatalf("open part: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, onDisk := f.(*os.File); !onDisk {
		t.Fatalf("%d MiB part is held in memory (%T), want it spooled to a temp file", partSize>>20, f)
	}
}

// TestImporterErrorStatus_ZipLimitCodesAre413 pins that every zip extraction
// limit, including the entry-count cap, answers 413 like the upload cap.
func TestImporterErrorStatus_ZipLimitCodesAre413(t *testing.T) {
	for _, code := range []string{
		ErrCodeImporterUploadTooLarge,
		ErrCodeImporterZipEntryTooLarge,
		ErrCodeImporterZipExtractedTooLarge,
		ErrCodeImporterZipRatioTooHigh,
		ErrCodeImporterZipTooManyEntries,
	} {
		if got := importerErrorStatus(code); got != http.StatusRequestEntityTooLarge {
			t.Errorf("importerErrorStatus(%q) = %d, want %d", code, got, http.StatusRequestEntityTooLarge)
		}
	}
}
