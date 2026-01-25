package importer

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

type fakeExecWiki struct {
	hash string

	ensureCalls int
	updateCalls int

	ensureFn func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error)
	updateFn func(userID, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error)

	lastUpdatedContent *string
}

func (f *fakeExecWiki) TreeHash() string { return f.hash }

func (f *fakeExecWiki) LookupPagePath(path string) (*tree.PathLookup, error) {
	panic("not used by Executor")
}

func (f *fakeExecWiki) EnsurePath(userID string, targetPath string, title string, kind *tree.NodeKind) (*tree.Page, error) {
	f.ensureCalls++
	if f.ensureFn != nil {
		return f.ensureFn(userID, targetPath, title, kind)
	}
	return &tree.Page{PageNode: &tree.PageNode{ID: "p1", Title: title, Slug: "slug", Kind: *kind}}, nil
}

func (f *fakeExecWiki) UpdatePage(userID string, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error) {
	f.updateCalls++
	f.lastUpdatedContent = content
	if f.updateFn != nil {
		return f.updateFn(userID, id, title, slug, content, kind)
	}
	// simulate tree change
	f.hash = f.hash + "-changed"
	return &tree.Page{PageNode: &tree.PageNode{ID: id, Title: title, Slug: slug, Kind: *kind}}, nil
}

func writeTmp(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestExecutor_StalePlan(t *testing.T) {
	w := &fakeExecWiki{hash: "new"}
	plan := &PlanResult{TreeHash: "old"}
	opts := &PlanOptions{SourceBasePath: t.TempDir()}
	ex := NewExecutor(plan, opts, w, slog.Default())

	got, err := ex.Execute("user1")
	if err == nil {
		t.Fatalf("expected stale plan error")
	}
	if got != nil {
		t.Fatalf("expected nil result on stale plan, got %#v", got)
	}
}

func TestExecutor_Create_HappyPath_StripsFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "a.md", "---\ntitle: X\n---\n\n# Heading\nBody")

	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Title: "A", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, w, slog.Default())

	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if res.ImportedCount != 1 || res.SkippedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if len(res.Items) != 1 || res.Items[0].Action != ExecutionActionCreated {
		t.Fatalf("item result: %#v", res.Items)
	}
	if w.ensureCalls != 1 || w.updateCalls != 1 {
		t.Fatalf("calls ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}

	if w.lastUpdatedContent == nil {
		t.Fatalf("expected content to be passed to UpdatePage")
	}
	if strings.Contains(*w.lastUpdatedContent, "title: X") || strings.Contains(*w.lastUpdatedContent, "---") {
		t.Fatalf("frontmatter was not stripped, got: %q", *w.lastUpdatedContent)
	}
	if !strings.Contains(*w.lastUpdatedContent, "# Heading") {
		t.Fatalf("expected body content, got: %q", *w.lastUpdatedContent)
	}

	if res.TreeHashBefore != "h1" {
		t.Fatalf("TreeHashBefore = %q", res.TreeHashBefore)
	}
	if res.TreeHash == "h1" {
		t.Fatalf("expected TreeHash to change (fake changes it), got %q", res.TreeHash)
	}
}

func TestExecutor_Skip_DoesNotCallWiki(t *testing.T) {
	tmp := t.TempDir()
	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Action: PlanActionSkip},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}

	if res.SkippedCount != 1 || res.ImportedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if w.ensureCalls != 0 || w.updateCalls != 0 {
		t.Fatalf("expected no wiki calls, got ensure=%d update=%d", w.ensureCalls, w.updateCalls)
	}
}

func TestExecutor_Create_EnsurePathError_SkipsItem(t *testing.T) {
	tmp := t.TempDir()
	writeTmp(t, tmp, "a.md", "Body")

	w := &fakeExecWiki{
		hash: "h1",
		ensureFn: func(userID, targetPath, title string, kind *tree.NodeKind) (*tree.Page, error) {
			return nil, errors.New("boom")
		},
	}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Title: "A", Kind: tree.NodeKindPage, Action: PlanActionCreate},
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if res.SkippedCount != 1 || res.ImportedCount != 0 {
		t.Fatalf("counts imported=%d skipped=%d", res.ImportedCount, res.SkippedCount)
	}
	if res.Items[0].Error == nil || *res.Items[0].Error == "" {
		t.Fatalf("expected error message")
	}
	if w.updateCalls != 0 {
		t.Fatalf("UpdatePage should not be called")
	}
}

func TestExecutor_UnknownAction_SkipsItem(t *testing.T) {
	tmp := t.TempDir()
	w := &fakeExecWiki{hash: "h1"}
	plan := &PlanResult{
		TreeHash: "h1",
		Items: []PlanItem{
			{SourcePath: "a.md", TargetPath: "docs/a", Action: PlanActionUpdate}, // not handled in switch
		},
	}
	opts := &PlanOptions{SourceBasePath: tmp}

	ex := NewExecutor(plan, opts, w, slog.Default())
	res, err := ex.Execute("user1")
	if err != nil {
		t.Fatalf("Execute err: %v", err)
	}
	if res.SkippedCount != 1 {
		t.Fatalf("SkippedCount=%d", res.SkippedCount)
	}
	if res.Items[0].Error == nil || *res.Items[0].Error != "unknown action" {
		t.Fatalf("Error=%#v", res.Items[0].Error)
	}
}
