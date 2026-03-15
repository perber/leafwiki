package wiki

import (
	"testing"
	"time"

	verrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

func createWikiTestInstance(t *testing.T) *Wiki {
	t.Helper()

	w, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return w
}

func pageKind() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func mustCreateNode(t *testing.T, w *Wiki, parentID *string, title, slug string, kind *tree.NodeKind) *tree.Page {
	t.Helper()

	p, err := w.CreateNode("system", parentID, title, slug, kind)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	return p
}

func TestWiki_CreateNode(t *testing.T) {
	t.Run("creates page at root", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		page := mustCreateNode(t, w, nil, "Home", "home", pageKind())

		if page.Title != "Home" {
			t.Fatalf("expected title %q, got %q", "Home", page.Title)
		}
		if page.Slug != "home" {
			t.Fatalf("expected slug %q, got %q", "home", page.Slug)
		}
	})

	t.Run("creates page with parent", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		parent := mustCreateNode(t, w, nil, "Docs", "docs", pageKind())
		page := mustCreateNode(t, w, &parent.ID, "API Doc", "api-doc", pageKind())

		if page.Parent == nil || page.Parent.ID != parent.ID {
			t.Fatalf("expected parent ID %q, got %#v", parent.ID, page.Parent)
		}
	})

	t.Run("rejects duplicate slug under same parent", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_ = mustCreateNode(t, w, nil, "Duplicate", "duplicate", pageKind())

		_, err := w.CreateNode("system", nil, "Duplicate", "duplicate", pageKind())
		if err == nil {
			t.Fatalf("expected error for duplicate page")
		}
	})
}

func TestWiki_CreateNode_Validation(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		slug     string
		parentID *string
		wantErr  bool
		check    func(t *testing.T, err error)
	}{
		{
			name:    "rejects empty title",
			title:   "",
			slug:    "empty",
			wantErr: true,
		},
		{
			name:    "rejects reserved slug",
			title:   "Reserved",
			slug:    "e",
			wantErr: true,
			check: func(t *testing.T, err error) {
				t.Helper()
				ve, ok := err.(*verrors.ValidationErrors)
				if !ok {
					t.Fatalf("expected ValidationErrors, got %T", err)
				}
				if len(ve.Errors) != 1 || ve.Errors[0].Field != "slug" {
					t.Fatalf("expected validation error for slug, got %#v", ve)
				}
			},
		},
		{
			name:  "rejects invalid parent",
			title: "Broken",
			slug:  "broken",
			parentID: func() *string {
				s := "not-real"
				return &s
			}(),
			wantErr: true,
		},
		{
			name:    "rejects nil kind",
			title:   "Broken",
			slug:    "broken",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := createWikiTestInstance(t)
			defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

			var kind *tree.NodeKind
			if tc.name != "rejects nil kind" {
				kind = pageKind()
			}

			_, err := w.CreateNode("system", tc.parentID, tc.title, tc.slug, kind)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.check != nil {
				tc.check(t, err)
			}
		})
	}
}

func TestWiki_GetPage(t *testing.T) {
	t.Run("valid id", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		page := mustCreateNode(t, w, nil, "ReadMe", "readme", pageKind())

		found, err := w.GetPage(page.ID)
		if err != nil {
			t.Fatalf("GetPage failed: %v", err)
		}
		if found.ID != page.ID {
			t.Fatalf("expected ID %q, got %q", page.ID, found.ID)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_, err := w.GetPage("unknown")
		if err == nil {
			t.Fatalf("expected error for unknown ID")
		}
	})
}

func TestWiki_UpdatePage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page := mustCreateNode(t, w, nil, "Draft", "draft", pageKind())

	content := "# Updated"
	updatedPage, err := w.UpdatePage("system", page.ID, "Final", "final", &content, pageKind())
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	if updatedPage.Title != "Final" {
		t.Fatalf("expected title %q, got %q", "Final", updatedPage.Title)
	}
	if updatedPage.Slug != "final" {
		t.Fatalf("expected slug %q, got %q", "final", updatedPage.Slug)
	}

	updated, err := w.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if updated.Title != "Final" {
		t.Fatalf("expected persisted title %q, got %q", "Final", updated.Title)
	}
}

func TestWiki_DeletePage(t *testing.T) {
	t.Run("simple delete", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		page := mustCreateNode(t, w, nil, "Trash", "trash", pageKind())

		if err := w.DeletePage("system", page.ID, false); err != nil {
			t.Fatalf("DeletePage failed: %v", err)
		}

		if _, err := w.GetPage(page.ID); err == nil {
			t.Fatalf("expected page to be deleted")
		}
	})

	t.Run("delete with children non-recursive errors", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		parent := mustCreateNode(t, w, nil, "Parent", "parent", pageKind())
		_ = mustCreateNode(t, w, &parent.ID, "Child", "child", pageKind())

		err := w.DeletePage("system", parent.ID, false)
		if err == nil {
			t.Fatalf("expected error when deleting parent with children")
		}
	})

	t.Run("delete with children recursive succeeds", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		parent := mustCreateNode(t, w, nil, "Parent", "parent", pageKind())
		_ = mustCreateNode(t, w, &parent.ID, "Child", "child", pageKind())

		if err := w.DeletePage("system", parent.ID, true); err != nil {
			t.Fatalf("DeletePage recursive failed: %v", err)
		}
	})

	t.Run("root id is rejected", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		err := w.DeletePage("system", "root", false)
		if err == nil {
			t.Fatalf("expected error deleting root")
		}
		if err.Error() != "cannot delete root page" {
			t.Fatalf("unexpected error: %q", err.Error())
		}
	})

	t.Run("empty id is rejected", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		err := w.DeletePage("system", "", false)
		if err == nil {
			t.Fatalf("expected error deleting empty id")
		}
		if err.Error() != "cannot delete root page" {
			t.Fatalf("unexpected error: %q", err.Error())
		}
	})
}

func TestWiki_MovePage(t *testing.T) {
	t.Run("valid move", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		parent := mustCreateNode(t, w, nil, "Projects", "projects", pageKind())
		child := mustCreateNode(t, w, nil, "Old", "old", pageKind())

		if err := w.MovePage("system", child.ID, parent.ID); err != nil {
			t.Fatalf("MovePage failed: %v", err)
		}

		moved, err := w.GetPage(child.ID)
		if err != nil {
			t.Fatalf("GetPage failed: %v", err)
		}
		if moved.Parent == nil || moved.Parent.ID != parent.ID {
			t.Fatalf("expected parent ID %q, got %#v", parent.ID, moved.Parent)
		}
	})
}

func TestWiki_SuggestSlug(t *testing.T) {
	t.Run("unique at root", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		slug, err := w.SuggestSlug("root", "", "My Page")
		if err != nil {
			t.Fatalf("SuggestSlug failed: %v", err)
		}
		if slug != "my-page" {
			t.Fatalf("expected %q, got %q", "my-page", slug)
		}
	})

	t.Run("conflict at root", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_ = mustCreateNode(t, w, nil, "My Page", "my-page", pageKind())

		slug, err := w.SuggestSlug("root", "", "My Page")
		if err != nil {
			t.Fatalf("SuggestSlug failed: %v", err)
		}
		if slug != "my-page-1" {
			t.Fatalf("expected %q, got %q", "my-page-1", slug)
		}
	})

	t.Run("deep hierarchy", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		arch := mustCreateNode(t, w, nil, "Architecture", "architecture", pageKind())
		backend := mustCreateNode(t, w, &arch.ID, "Backend", "backend", pageKind())

		slug, err := w.SuggestSlug(backend.ID, "", "Data Layer")
		if err != nil {
			t.Fatalf("SuggestSlug failed: %v", err)
		}
		if slug != "data-layer" {
			t.Fatalf("expected %q, got %q", "data-layer", slug)
		}

		_ = mustCreateNode(t, w, &backend.ID, "Data Layer", "data-layer", pageKind())

		slug2, err := w.SuggestSlug(backend.ID, "", "Data Layer")
		if err != nil {
			t.Fatalf("SuggestSlug failed: %v", err)
		}
		if slug2 != "data-layer-1" {
			t.Fatalf("expected %q, got %q", "data-layer-1", slug2)
		}
	})
}

func TestWiki_FindByPath(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_ = mustCreateNode(t, w, nil, "Company", "company", pageKind())

		found, err := w.FindByPath("company")
		if err != nil {
			t.Fatalf("FindByPath failed: %v", err)
		}
		if found.Slug != "company" {
			t.Fatalf("expected slug %q, got %q", "company", found.Slug)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_, err := w.FindByPath("does/not/exist")
		if err == nil {
			t.Fatalf("expected error for invalid path")
		}
	})
}

func TestWiki_SortPages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	parent := mustCreateNode(t, w, nil, "Parent", "parent", pageKind())
	child1 := mustCreateNode(t, w, &parent.ID, "Child1", "child1", pageKind())
	child2 := mustCreateNode(t, w, &parent.ID, "Child2", "child2", pageKind())

	if err := w.SortPages(parent.ID, []string{child2.ID, child1.ID}); err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}

	updatedParent, err := w.GetPage(parent.ID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if len(updatedParent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(updatedParent.Children))
	}
	if updatedParent.Children[0].ID != child2.ID || updatedParent.Children[1].ID != child1.ID {
		t.Fatalf("expected order [child2, child1], got [%s, %s]", updatedParent.Children[0].Slug, updatedParent.Children[1].Slug)
	}
}

func TestWiki_CopyPage(t *testing.T) {
	t.Run("simple copy", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		original := mustCreateNode(t, w, nil, "Original", "original", pageKind())

		copied, err := w.CopyPage("system", original.ID, nil, "Copy of Original", "copy-of-original")
		if err != nil {
			t.Fatalf("CopyPage failed: %v", err)
		}

		if copied.Title != "Copy of Original" {
			t.Fatalf("expected title %q, got %q", "Copy of Original", copied.Title)
		}
		if copied.Slug != "copy-of-original" {
			t.Fatalf("expected slug %q, got %q", "copy-of-original", copied.Slug)
		}
		if copied.ID == original.ID {
			t.Fatalf("expected copied page to have different ID")
		}
	})

	t.Run("copy with parent", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		parent := mustCreateNode(t, w, nil, "Parent", "parent", pageKind())
		original := mustCreateNode(t, w, nil, "Original", "original", pageKind())

		copied, err := w.CopyPage("system", original.ID, &parent.ID, "Copy of Original", "copy-of-original")
		if err != nil {
			t.Fatalf("CopyPage failed: %v", err)
		}

		if copied.Parent == nil || copied.Parent.ID != parent.ID {
			t.Fatalf("expected parent ID %q, got %#v", parent.ID, copied.Parent)
		}
	})

	t.Run("non existent source", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		_, err := w.CopyPage("system", "non-existent-id", nil, "Copy", "copy")
		if err == nil {
			t.Fatalf("expected error for missing source")
		}
	})

	t.Run("copy with assets", func(t *testing.T) {
		w := createWikiTestInstance(t)
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		original := mustCreateNode(t, w, nil, "Original", "original", pageKind())

		originalNode := tree.PageNode{
			ID:    original.ID,
			Title: original.Title,
			Slug:  original.Slug,
		}

		file, _, err := test_utils.CreateMultipartFile("image.png", []byte("image content"))
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(file.Close, t)

		if _, err := w.GetAssetService().SaveAssetForPage(&originalNode, file, "image.png"); err != nil {
			t.Fatalf("Failed to save asset for original page: %v", err)
		}

		copied, err := w.CopyPage("system", original.ID, nil, "Copy of Original", "copy-of-original")
		if err != nil {
			t.Fatalf("CopyPage failed: %v", err)
		}

		copiedNode := tree.PageNode{
			ID:    copied.ID,
			Title: copied.Title,
			Slug:  copied.Slug,
		}

		copiedAssets, err := w.GetAssetService().ListAssetsForPage(&copiedNode)
		if err != nil {
			t.Fatalf("Failed to list assets for copied page: %v", err)
		}
		if len(copiedAssets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(copiedAssets))
		}
	})
}

func TestWiki_InitDefaultAdmin_UsesGivenPassword(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	_, err := w.GetUserService().GetUserByEmailOrUsernameAndPassword("admin", "admin")
	if err != nil {
		t.Fatalf("Admin user not found: %v", err)
	}
}

func TestWiki_Login_SuccessAndFailure(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	token, err := w.Login("admin", "admin")
	if err != nil || token == nil {
		t.Fatalf("expected login to succeed")
	}

	_, err = w.Login("admin", "wrong")
	if err == nil {
		t.Fatalf("expected login to fail with wrong password")
	}
}

func TestWiki_EnsurePath_HealsLinksForAllCreatedSegments(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	pageA := mustCreateNode(t, w, nil, "Page A", "a", pageKind())

	contentA := "Links: [X](/x) and [XY](/x/y)"
	_, err := w.UpdatePage("system", pageA.ID, pageA.Title, pageA.Slug, &contentA, pageKind())
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	out1, err := w.GetOutgoingLinks(pageA.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before ensure, got %d", out1.Count)
	}

	_, err = w.EnsurePath("system", "/x/y", "X Y", pageKind())
	if err != nil {
		t.Fatalf("EnsurePath failed: %v", err)
	}

	out2, err := w.GetOutgoingLinks(pageA.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks after ensure failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after ensure, got %d", out2.Count)
	}
}

func TestWiki_DeletePage_NonRecursive_MarksIncomingBroken(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	a := mustCreateNode(t, w, nil, "Page A", "a", pageKind())
	contentA := "Link to B: [Go](/b)"
	_, err := w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	b := mustCreateNode(t, w, nil, "Page B", "b", pageKind())
	contentB := "# Page B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	if err := w.DeletePage("system", b.ID, false); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	out, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", out.Count)
	}
	if out.Outgoings[0].ToPath != "/b" {
		t.Fatalf("expected ToPath /b, got %q", out.Outgoings[0].ToPath)
	}
	if !out.Outgoings[0].Broken {
		t.Fatalf("expected outgoing link to be broken")
	}
	if out.Outgoings[0].ToPageID != "" {
		t.Fatalf("expected empty ToPageID, got %q", out.Outgoings[0].ToPageID)
	}
}

func TestWiki_AuthDisabled(t *testing.T) {
	t.Run("auth service is nil", func(t *testing.T) {
		w, err := NewWiki(&WikiOptions{
			StorageDir:   t.TempDir(),
			AuthDisabled: true,
		})
		if err != nil {
			t.Fatalf("Failed to create wiki instance: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		if w.GetAuthService() != nil {
			t.Fatalf("expected auth service to be nil")
		}
	})

	t.Run("login logout refresh return ErrAuthDisabled", func(t *testing.T) {
		w, err := NewWiki(&WikiOptions{
			StorageDir:   t.TempDir(),
			AuthDisabled: true,
		})
		if err != nil {
			t.Fatalf("Failed to create wiki instance: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		tests := []struct {
			name string
			run  func() error
		}{
			{
				name: "login",
				run: func() error {
					_, err := w.Login("admin", "admin")
					return err
				},
			},
			{
				name: "logout",
				run: func() error {
					return w.Logout("some-token")
				},
			},
			{
				name: "refresh",
				run: func() error {
					_, err := w.RefreshToken("some-token")
					return err
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if err := tc.run(); err != ErrAuthDisabled {
					t.Fatalf("expected ErrAuthDisabled, got %v", err)
				}
			})
		}
	})

	t.Run("core functionality still works", func(t *testing.T) {
		w, err := NewWiki(&WikiOptions{
			StorageDir:   t.TempDir(),
			AuthDisabled: true,
		})
		if err != nil {
			t.Fatalf("Failed to create wiki instance: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

		page := mustCreateNode(t, w, nil, "Test Page", "test-page", pageKind())

		content := "# Content"
		updatedPage, err := w.UpdatePage("system", page.ID, "Updated Title", "updated-slug", &content, pageKind())
		if err != nil {
			t.Fatalf("UpdatePage failed: %v", err)
		}
		if updatedPage.Title != "Updated Title" {
			t.Fatalf("expected title %q, got %q", "Updated Title", updatedPage.Title)
		}

		retrievedPage, err := w.GetPage(page.ID)
		if err != nil {
			t.Fatalf("GetPage failed: %v", err)
		}
		if retrievedPage.ID != page.ID {
			t.Fatalf("expected ID %q, got %q", page.ID, retrievedPage.ID)
		}

		if err := w.DeletePage("system", page.ID, false); err != nil {
			t.Fatalf("DeletePage failed: %v", err)
		}
	})
}
