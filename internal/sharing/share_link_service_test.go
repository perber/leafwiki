package sharing

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

var testNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// fakePages knows a fixed set of page IDs; "root" is the tree root, which
// the real TreeService also returns from FindPageByID.
type fakePages map[string]bool

func (p fakePages) FindPageByID(id string) (*tree.PageNode, error) {
	if id == "root" || p[id] {
		return &tree.PageNode{ID: id}, nil
	}
	return nil, tree.ErrPageNotFound
}

func newTestService(t *testing.T) *ShareLinkService {
	t.Helper()
	svc := NewShareLinkService(newTestStore(t), fakePages{"page-1": true, "page-2": true}, nil)
	svc.now = func() time.Time { return testNow }
	return svc
}

func mustCreate(t *testing.T, svc *ShareLinkService, in CreateShareLinkInput) (*ShareLink, string) {
	t.Helper()
	link, token, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return link, token
}

func errorCode(err error) string {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		return loc.Code
	}
	return ""
}

func TestShareLinkService_Create_ReturnsLinkAndTokenOnce(t *testing.T) {
	svc := newTestService(t)
	expires := testNow.Add(24 * time.Hour)

	link, token := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", IncludeDescendants: true, ExpiresAt: &expires, CreatedBy: "editor-1"})

	if !strings.HasPrefix(token, tokenPrefix) || len(token) != tokenLength {
		t.Errorf("token %q: want prefix %q and length %d", token, tokenPrefix, tokenLength)
	}
	if link.ID == "" || link.PageID != "page-1" || !link.IncludeDescendants || link.CreatedBy != "editor-1" ||
		!link.CreatedAt.Equal(testNow) || link.ExpiresAt == nil || !link.ExpiresAt.Equal(expires) {
		t.Errorf("unexpected link %+v", link)
	}

	// Only the hash is stored, and the returned link carries none.
	if link.tokenHash != "" {
		t.Error("the created link must not carry the token hash")
	}
	if ids := storedLinkIDs(t, svc.store); len(ids) != 1 || ids[0] != link.ID {
		t.Fatalf("stored links = %v, want [%s]", ids, link.ID)
	}
	row, err := svc.store.FindByTokenHash(context.Background(), hashToken(token))
	if err != nil {
		t.Fatalf("FindByTokenHash: %v", err)
	}
	if row.tokenHash == token || strings.Contains(row.tokenHash, token) {
		t.Fatal("the plaintext token must never be stored")
	}
}

func TestShareLinkService_Create_TokensAreUnique(t *testing.T) {
	svc := newTestService(t)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		_, token := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", CreatedBy: "editor-1"})
		if seen[token] {
			t.Fatalf("duplicate token after %d creates", i)
		}
		seen[token] = true
	}
}

func TestShareLinkService_Create_ValidatesInput(t *testing.T) {
	past := testNow.Add(-time.Minute)
	now := testNow

	tests := []struct {
		name     string
		in       CreateShareLinkInput
		wantCode string
	}{
		{name: "unknown page", in: CreateShareLinkInput{PageID: "missing", CreatedBy: "editor-1"}, wantCode: ErrCodeShareLinkInvalidPage},
		{name: "empty page", in: CreateShareLinkInput{PageID: "", CreatedBy: "editor-1"}, wantCode: ErrCodeShareLinkInvalidPage},
		{name: "tree root", in: CreateShareLinkInput{PageID: "root", IncludeDescendants: true, CreatedBy: "editor-1"}, wantCode: ErrCodeShareLinkInvalidPage},
		{name: "expiry in the past", in: CreateShareLinkInput{PageID: "page-1", ExpiresAt: &past, CreatedBy: "editor-1"}, wantCode: ErrCodeShareLinkInvalidExpiry},
		{name: "expiry now", in: CreateShareLinkInput{PageID: "page-1", ExpiresAt: &now, CreatedBy: "editor-1"}, wantCode: ErrCodeShareLinkInvalidExpiry},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(t)
			_, token, err := svc.Create(context.Background(), tc.in)
			if got := errorCode(err); got != tc.wantCode {
				t.Fatalf("error code = %q (err %v), want %q", got, err, tc.wantCode)
			}
			if token != "" {
				t.Error("no token may be returned on error")
			}
			if ids := storedLinkIDs(t, svc.store); len(ids) != 0 {
				t.Errorf("no link may be stored on error, got %v", ids)
			}
		})
	}
}

func TestShareLinkService_Create_RequiresCreator(t *testing.T) {
	svc := newTestService(t)
	if _, _, err := svc.Create(context.Background(), CreateShareLinkInput{PageID: "page-1"}); err == nil {
		t.Fatal("expected an error for a link without a creator")
	}
}

func TestShareLinkService_Resolve_ReturnsGrant(t *testing.T) {
	svc := newTestService(t)
	link, token := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", IncludeDescendants: true, CreatedBy: "editor-1"})

	grant, err := svc.Resolve(context.Background(), token)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Grant{ShareID: link.ID, PageID: "page-1", IncludeDescendants: true}
	if *grant != want {
		t.Errorf("grant = %+v, want %+v", *grant, want)
	}
}

// Unknown, malformed, expired and revoked tokens must be indistinguishable to
// the caller, so a share-link recipient can't learn which links exist.
func TestShareLinkService_Resolve_RejectsUniformly(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	expires := testNow.Add(time.Hour)
	_, expiring := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", ExpiresAt: &expires, CreatedBy: "editor-1"})
	revokedLink, revoked := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", CreatedBy: "editor-1"})
	if err := svc.Revoke(ctx, revokedLink.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, valid := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-2", CreatedBy: "editor-1"})

	tests := []struct {
		name  string
		token string
		at    time.Time
	}{
		{name: "empty", token: "", at: testNow},
		{name: "malformed", token: "not-a-token", at: testNow},
		{name: "right prefix, wrong length", token: tokenPrefix + "abc", at: testNow},
		{name: "well-formed but unknown", token: tokenPrefix + strings.Repeat("A", tokenLength-len(tokenPrefix)), at: testNow},
		{name: "valid token with trailing space", token: valid + " ", at: testNow},
		{name: "revoked", token: revoked, at: testNow},
		{name: "expired exactly at expiry", token: expiring, at: expires},
		{name: "expired after expiry", token: expiring, at: expires.Add(time.Second)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc.now = func() time.Time { return tc.at }
			grant, err := svc.Resolve(ctx, tc.token)
			if !errors.Is(err, ErrShareLinkNotFound) {
				t.Fatalf("err = %v, want ErrShareLinkNotFound", err)
			}
			if errorCode(err) != ErrCodeShareLinkNotFound {
				t.Errorf("error code = %q, want %q", errorCode(err), ErrCodeShareLinkNotFound)
			}
			if grant != nil {
				t.Errorf("grant = %+v, want nil", grant)
			}
		})
	}

	svc.now = func() time.Time { return expires.Add(-time.Second) }
	if _, err := svc.Resolve(ctx, expiring); err != nil {
		t.Fatalf("one second before expiry the link must still resolve: %v", err)
	}
}

// Create never stores a link to the tree root or to no page, but a restored
// share_links.db could contain one. Resolve must not turn it into a grant
// that would expose the whole wiki.
func TestShareLinkService_Resolve_RejectsStoredRootOrEmptyPage(t *testing.T) {
	for _, pageID := range []string{"root", ""} {
		t.Run("page "+pageID, func(t *testing.T) {
			svc := newTestService(t)
			token, err := newToken()
			if err != nil {
				t.Fatalf("newToken: %v", err)
			}
			row := testLink("restored", pageID, hashToken(token))
			row.IncludeDescendants = true
			if err := svc.store.Insert(context.Background(), row); err != nil {
				t.Fatalf("Insert: %v", err)
			}

			if _, err := svc.Resolve(context.Background(), token); !errors.Is(err, ErrShareLinkNotFound) {
				t.Fatalf("err = %v, want ErrShareLinkNotFound", err)
			}
		})
	}
}

func TestShareLinkService_Revoke_UnknownIsNotFound(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Revoke(context.Background(), "missing"); errorCode(err) != ErrCodeShareLinkNotFound {
		t.Fatalf("err = %v, want %s", err, ErrCodeShareLinkNotFound)
	}
}

func TestShareLinkService_Resolve_StoreUnavailableIsNotNotFound(t *testing.T) {
	svc := newTestService(t)
	_, token := mustCreate(t, svc, CreateShareLinkInput{PageID: "page-1", CreatedBy: "editor-1"})
	if err := svc.PauseForSwap(); err != nil {
		t.Fatalf("PauseForSwap: %v", err)
	}

	_, err := svc.Resolve(context.Background(), token)
	if errorCode(err) != ErrCodeShareLinkStoreUnavailable {
		t.Fatalf("err = %v, want %s (a restore in progress is not a missing link)", err, ErrCodeShareLinkStoreUnavailable)
	}
}

func TestShareLinkService_Create_ConcurrentCreatesAllPersist(t *testing.T) {
	svc := newTestService(t)
	const n = 20
	var wg sync.WaitGroup
	tokens := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, tokens[i], errs[i] = svc.Create(context.Background(), CreateShareLinkInput{PageID: "page-1", CreatedBy: "editor-1"})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		if _, err := svc.Resolve(context.Background(), tokens[i]); err != nil {
			t.Fatalf("Resolve %d: %v", i, err)
		}
	}
	if ids := storedLinkIDs(t, svc.store); len(ids) != n {
		t.Fatalf("stored %d links, want %d", len(ids), n)
	}
}
