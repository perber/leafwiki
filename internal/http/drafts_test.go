package http_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

type apiDraftResponse struct {
	Page        apiPage `json:"page"`
	BaseVersion string  `json:"baseVersion"`
}

func TestDraftLifecycleKeepsCanonicalPagePublicUntilPublish(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	page := createPageViaAPI(t, router, "Published", "published", nil, pageNodeKind())

	start := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+page.ID+"/draft", nil)
	if start.Code != http.StatusCreated {
		t.Fatalf("start = %d: %s", start.Code, start.Body.String())
	}
	var draft apiDraftResponse
	if err := json.Unmarshal(start.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if draft.BaseVersion != page.Version || draft.Page.Content != page.Content {
		t.Fatalf("unexpected draft %#v", draft)
	}

	saveBody := `{"title":"Draft title","content":"private draft","tags":["go"],"properties":{"status":"editing"}}`
	save := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID+"/draft", strings.NewReader(saveBody))
	if save.Code != http.StatusOK {
		t.Fatalf("save = %d: %s", save.Code, save.Body.String())
	}
	canonical := getPageByPathViaAPI(t, router, "published")
	if canonical.Title != "Published" || canonical.Content == "private draft" {
		t.Fatalf("canonical page changed before publish: %#v", canonical)
	}
	blockedUpdate := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID, strings.NewReader(`{"version":"`+page.Version+`","title":"Changed","slug":"published","content":"changed"}`))
	if blockedUpdate.Code != http.StatusConflict {
		t.Fatalf("canonical update with draft = %d, want 409", blockedUpdate.Code)
	}
	blockedPin := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID+"/pin", strings.NewReader(`{"version":"`+page.Version+`","pinned":true}`))
	if blockedPin.Code != http.StatusConflict {
		t.Fatalf("pin with draft = %d, want 409", blockedPin.Code)
	}
	var pinError struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(blockedPin.Body.Bytes(), &pinError); err != nil {
		t.Fatal(err)
	}
	if pinError.Error.Code != "draft_operation_blocked" {
		t.Fatalf("pin draft block code = %q, want draft_operation_blocked", pinError.Error.Code)
	}
	blockedDelete := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID+"?version="+page.Version, nil)
	if blockedDelete.Code != http.StatusConflict {
		t.Fatalf("delete with draft = %d, want 409", blockedDelete.Code)
	}

	createViewer := `{"username":"draft-viewer","email":"draft-viewer@example.com","password":"viewerpass","role":"viewer"}`
	_ = authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createViewer))
	viewer := authenticatedRequestAs(t, router, "draft-viewer", "viewerpass", http.MethodGet, "/api/pages/"+page.ID+"/draft", nil)
	if viewer.Code != http.StatusForbidden {
		t.Fatalf("viewer GET draft = %d, want 403", viewer.Code)
	}
	viewerPage := authenticatedRequestAs(t, router, "draft-viewer", "viewerpass", http.MethodGet, "/api/pages/"+page.ID, nil)
	if viewerPage.Code != http.StatusOK {
		t.Fatalf("viewer GET canonical page = %d: %s", viewerPage.Code, viewerPage.Body.String())
	}
	var publicPage apiPage
	if err := json.Unmarshal(viewerPage.Body.Bytes(), &publicPage); err != nil {
		t.Fatal(err)
	}
	if publicPage.Title != "Published" || publicPage.Content == "private draft" {
		t.Fatalf("viewer saw draft content before publish: %#v", publicPage)
	}

	publish := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+page.ID+"/draft/publish", nil)
	if publish.Code != http.StatusOK {
		t.Fatalf("publish = %d: %s", publish.Code, publish.Body.String())
	}
	canonical = getPageByPathViaAPI(t, router, "published")
	if canonical.Title != "Draft title" || canonical.Content != "private draft" || len(canonical.Tags) != 1 || canonical.Tags[0] != "go" {
		t.Fatalf("publish did not use normal page update: %#v", canonical)
	}
	missing := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID+"/draft", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("draft after publish = %d, want 404", missing.Code)
	}
}

func TestDraftDiscardKeepsCanonicalPage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	page := createPageViaAPI(t, router, "Published", "published", nil, pageNodeKind())
	if rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+page.ID+"/draft", nil); rec.Code != http.StatusCreated {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := authenticatedRequest(t, router, http.MethodPut, "/api/pages/"+page.ID+"/draft", strings.NewReader(`{"title":"Draft title","content":"private draft"}`)); rec.Code != http.StatusOK {
		t.Fatalf("save = %d: %s", rec.Code, rec.Body.String())
	}

	discard := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/"+page.ID+"/draft", nil)
	if discard.Code != http.StatusNoContent {
		t.Fatalf("discard = %d: %s", discard.Code, discard.Body.String())
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID+"/draft", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("draft after discard = %d, want 404", rec.Code)
	}
	canonical := getPageByPathViaAPI(t, router, "published")
	if canonical.Title != "Published" || canonical.Content == "private draft" {
		t.Fatalf("discard changed canonical page: %#v", canonical)
	}
}

func TestPendingDraftLifecycleStaysPrivateUntilPublish(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	created := authenticatedRequest(t, router, http.MethodPost, "/api/pages/drafts", strings.NewReader(`{"title":"Pending","slug":"pending"}`))
	if created.Code != http.StatusCreated {
		t.Fatalf("create pending = %d: %s", created.Code, created.Body.String())
	}
	var pending struct {
		Page apiPage `json:"page"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/tree", nil); strings.Contains(rec.Body.String(), pending.Page.ID) {
		t.Fatalf("pending draft leaked into tree: %s", rec.Body.String())
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path=pending", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("pending public read = %d, want 404", rec.Code)
	}
	createViewer := `{"username":"pending-viewer","email":"pending-viewer@example.com","password":"viewerpass","role":"viewer"}`
	_ = authenticatedRequest(t, router, http.MethodPost, "/api/users", strings.NewReader(createViewer))
	if rec := authenticatedRequestAs(t, router, "pending-viewer", "viewerpass", http.MethodGet, "/api/pages/drafts/"+pending.Page.ID, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer pending read = %d, want 403", rec.Code)
	}
	saved := authenticatedRequest(t, router, http.MethodPut, "/api/pages/drafts/"+pending.Page.ID, strings.NewReader(`{"title":"Pending title","slug":"pending","content":"private body","tags":["go"],"properties":{"status":"draft"}}`))
	if saved.Code != http.StatusOK {
		t.Fatalf("save pending = %d: %s", saved.Code, saved.Body.String())
	}
	reopened := authenticatedRequest(t, router, http.MethodGet, "/api/pages/drafts/"+pending.Page.ID, nil)
	if reopened.Code != http.StatusOK || !strings.Contains(reopened.Body.String(), "private body") {
		t.Fatalf("reopen pending = %d: %s", reopened.Code, reopened.Body.String())
	}
	published := authenticatedRequest(t, router, http.MethodPost, "/api/pages/drafts/"+pending.Page.ID+"/publish", nil)
	if published.Code != http.StatusOK {
		t.Fatalf("publish pending = %d: %s", published.Code, published.Body.String())
	}
	canonical := getPageByPathViaAPI(t, router, "pending")
	if canonical.Title != "Pending title" || canonical.Content != "private body" || canonical.Properties["status"] != "draft" || len(canonical.Tags) != 1 || canonical.Tags[0] != "go" {
		t.Fatalf("published pending page = %#v", canonical)
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/drafts/"+pending.Page.ID, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("pending after publish = %d", rec.Code)
	}

	discarded := authenticatedRequest(t, router, http.MethodPost, "/api/pages/drafts", strings.NewReader(`{"title":"Discarded","slug":"discarded"}`))
	var discardedDraft struct {
		Page apiPage `json:"page"`
	}
	if err := json.Unmarshal(discarded.Body.Bytes(), &discardedDraft); err != nil {
		t.Fatal(err)
	}
	if rec := authenticatedRequest(t, router, http.MethodDelete, "/api/pages/drafts/"+discardedDraft.Page.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("discard pending = %d", rec.Code)
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/by-path?path=discarded", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("discarded public read = %d", rec.Code)
	}
}

func TestCreateChildBelowDraftedLeafIsBlocked(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	parent := createPageViaAPI(t, router, "Drafted parent", "drafted-parent", nil, pageNodeKind())
	if rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+parent.ID+"/draft", nil); rec.Code != http.StatusCreated {
		t.Fatalf("start draft = %d: %s", rec.Code, rec.Body.String())
	}

	blocked := authenticatedRequest(t, router, http.MethodPost, "/api/pages", strings.NewReader(`{"parentId":"`+parent.ID+`","title":"Child","slug":"child"}`))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("create below drafted leaf = %d, want 409: %s", blocked.Code, blocked.Body.String())
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(blocked.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "draft_operation_blocked" {
		t.Fatalf("draft block code = %q, want draft_operation_blocked", response.Error.Code)
	}
	if canonical := getPageByPathViaAPI(t, router, "drafted-parent"); canonical.Kind != *pageNodeKind() {
		t.Fatalf("blocked create converted drafted parent: %#v", canonical)
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+parent.ID+"/draft", nil); rec.Code != http.StatusOK {
		t.Fatalf("draft after blocked create = %d: %s", rec.Code, rec.Body.String())
	}

	undrafted := createPageViaAPI(t, router, "Undrafted parent", "undrafted-parent", nil, pageNodeKind())
	child := createPageViaAPI(t, router, "Child", "child", &undrafted.ID, pageNodeKind())
	if child.Path != "undrafted-parent/child" {
		t.Fatalf("created child path = %q", child.Path)
	}
}

func TestEnsureChildBelowDraftedLeafIsBlocked(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	parent := createPageViaAPI(t, router, "Drafted parent", "drafted-parent", nil, pageNodeKind())
	if rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+parent.ID+"/draft", nil); rec.Code != http.StatusCreated {
		t.Fatalf("start draft = %d: %s", rec.Code, rec.Body.String())
	}

	blocked := authenticatedRequest(t, router, http.MethodPost, "/api/pages/ensure", strings.NewReader(`{"path":"drafted-parent/child","title":"Child"}`))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("ensure below drafted leaf = %d, want 409: %s", blocked.Code, blocked.Body.String())
	}
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(blocked.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "draft_operation_blocked" {
		t.Fatalf("draft block code = %q, want draft_operation_blocked", response.Error.Code)
	}
	if canonical := getPageByPathViaAPI(t, router, "drafted-parent"); canonical.Kind != *pageNodeKind() {
		t.Fatalf("blocked ensure converted drafted parent: %#v", canonical)
	}
	if rec := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+parent.ID+"/draft", nil); rec.Code != http.StatusOK {
		t.Fatalf("draft after blocked ensure = %d: %s", rec.Code, rec.Body.String())
	}

	createPageViaAPI(t, router, "Undrafted parent", "undrafted-parent", nil, pageNodeKind())
	ensured := authenticatedRequest(t, router, http.MethodPost, "/api/pages/ensure", strings.NewReader(`{"path":"undrafted-parent/child","title":"Child"}`))
	if ensured.Code != http.StatusOK {
		t.Fatalf("ensure below undrafted leaf = %d: %s", ensured.Code, ensured.Body.String())
	}
}

func TestDraftPublishConflictPreservesDraft(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)
	page := createPageViaAPI(t, router, "Published", "published", nil, pageNodeKind())
	if rec := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+page.ID+"/draft", nil); rec.Code != http.StatusCreated {
		t.Fatalf("start = %d: %s", rec.Code, rec.Body.String())
	}

	path := filepath.Join(w.GetStorageDir(), "root", "published.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := regexp.MustCompile(`leafwiki_updated_at: .*`).ReplaceAllString(string(raw), "leafwiki_updated_at: "+time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano))
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.ReloadFromFS(); err != nil {
		t.Fatal(err)
	}

	publish := authenticatedRequest(t, router, http.MethodPost, "/api/pages/"+page.ID+"/draft/publish", nil)
	if publish.Code != http.StatusConflict {
		t.Fatalf("publish = %d: %s", publish.Code, publish.Body.String())
	}
	stillThere := authenticatedRequest(t, router, http.MethodGet, "/api/pages/"+page.ID+"/draft", nil)
	if stillThere.Code != http.StatusOK {
		t.Fatalf("draft after conflict = %d: %s", stillThere.Code, stillThere.Body.String())
	}
}
