package backup

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

// TestPlanContentChanges_PerPathCases_PlansExpectedChanges covers how a pull treats one path,
// given its local HEAD (base), remote and live versions.
func TestPlanContentChanges_PerPathCases_PlansExpectedChanges(t *testing.T) {
	blob := func(content string) plumbing.Hash {
		return plumbing.ComputeHash(plumbing.BlobObject, []byte(content))
	}
	files := func(kv ...string) map[string]plumbing.Hash {
		out := map[string]plumbing.Hash{}
		for i := 0; i < len(kv); i += 2 {
			out[kv[i]] = blob(kv[i+1])
		}
		return out
	}
	write := func(rel, content string) []plannedWrite {
		return []plannedWrite{{rel: rel, blob: blob(content)}}
	}
	always := func(string) bool { return true }
	never := func(string) bool { return false }

	tests := []struct {
		name         string
		base         map[string]plumbing.Hash
		remote       map[string]plumbing.Hash
		live         map[string]plumbing.Hash
		caseFold     bool
		unbornBranch bool
		// equivalent, when set, answers the content-equivalence check for
		// every path; wantConsulted lists the paths it must be asked about.
		equivalent    func(rel string) bool
		wantConsulted []string
		want          contentChanges
	}{
		{
			name:   "remote unchanged, local edit is left for the backup",
			base:   files("p.md", "base"),
			remote: files("p.md", "base"),
			live:   files("p.md", "local"),
		},
		{
			name:   "remote adds a file that is absent locally",
			remote: files("new.md", "remote"),
			live:   files(),
			want:   contentChanges{writes: write("new.md", "remote")},
		},
		{
			name:   "remote adds a file that is already there with the same content",
			remote: files("new.md", "remote"),
			live:   files("new.md", "remote"),
		},
		{
			name:   "remote adds a file that exists locally with other content",
			remote: files("new.md", "remote"),
			live:   files("new.md", "local"),
			want:   contentChanges{conflicts: []string{"new.md"}},
		},
		{
			name:   "remote edits a file that is unchanged locally",
			base:   files("p.md", "base"),
			remote: files("p.md", "remote"),
			live:   files("p.md", "base"),
			want:   contentChanges{writes: write("p.md", "remote")},
		},
		{
			name:   "remote edit already applied locally",
			base:   files("p.md", "base"),
			remote: files("p.md", "remote"),
			live:   files("p.md", "remote"),
		},
		{
			name:   "remote and local edit the same file differently",
			base:   files("p.md", "base"),
			remote: files("p.md", "remote"),
			live:   files("p.md", "local"),
			want:   contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:   "remote edits a file deleted locally",
			base:   files("p.md", "base"),
			remote: files("p.md", "remote"),
			live:   files(),
			want:   contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:   "remote deletes a file that is unchanged locally",
			base:   files("p.md", "base"),
			remote: files(),
			live:   files("p.md", "base"),
			want:   contentChanges{deletes: []string{"p.md"}},
		},
		{
			name:   "remote delete already applied locally",
			base:   files("p.md", "base"),
			remote: files(),
			live:   files(),
		},
		{
			name:   "remote deletes a file edited locally",
			base:   files("p.md", "base"),
			remote: files(),
			live:   files("p.md", "local"),
			want:   contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:   "conflicts do not stop the plan for other paths",
			base:   files("p.md", "base", "old.md", "old"),
			remote: files("p.md", "remote", "new.md", "new"),
			live:   files("p.md", "local", "old.md", "old"),
			want: contentChanges{
				writes:    write("new.md", "new"),
				deletes:   []string{"old.md"},
				conflicts: []string{"p.md"},
			},
		},
		{
			name:         "unborn branch writes a remote file absent locally",
			remote:       files("p.md", "remote"),
			live:         files(),
			unbornBranch: true,
			want:         contentChanges{writes: write("p.md", "remote")},
		},
		{
			name:         "unborn branch keeps a local file with other content",
			remote:       files("p.md", "remote"),
			live:         files("p.md", "local"),
			unbornBranch: true,
		},
		{
			name:     "case fold finds an unchanged file renamed locally",
			base:     files("p.md", "base"),
			remote:   files("p.md", "remote"),
			live:     files("P.md", "base"),
			caseFold: true,
			want:     contentChanges{writes: write("p.md", "remote")},
		},
		{
			name:     "case fold finds the remote version under a different case",
			base:     files("p.md", "base"),
			remote:   files("p.md", "remote"),
			live:     files("P.md", "remote"),
			caseFold: true,
			// Rewritten with the same content, so the file survives a delete
			// of its old-case name in the same pull (next row).
			want: contentChanges{writes: write("p.md", "remote")},
		},
		{
			name:     "case fold applies a remote case-only rename",
			base:     files("Foo.md", "page"),
			remote:   files("foo.md", "page"),
			live:     files("Foo.md", "page"),
			caseFold: true,
			want: contentChanges{
				writes:  write("foo.md", "page"),
				deletes: []string{"Foo.md"},
			},
		},
		{
			name:   "without case fold a renamed file counts as deleted",
			base:   files("p.md", "base"),
			remote: files("p.md", "remote"),
			live:   files("P.md", "base"),
			want:   contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:   "every conflicting path is listed, not only the first",
			base:   files("a.md", "base", "b.md", "base"),
			remote: files("a.md", "remote", "b.md", "remote"),
			live:   files("a.md", "local", "b.md", "local"),
			want:   contentChanges{conflicts: []string{"a.md", "b.md"}},
		},
		{
			name:   "nested and Unicode paths are planned in sorted order",
			remote: files("z.md", "z", "a/ü/页.md", "u", "a/b.md", "b"),
			live:   files(),
			want: contentChanges{writes: []plannedWrite{
				{rel: "a/b.md", blob: blob("b")},
				{rel: "a/ü/页.md", blob: blob("u")},
				{rel: "z.md", blob: blob("z")},
			}},
		},
		{
			// Deletes run before writes, so the file is gone before the
			// directory of the same name is created.
			name:   "remote replaces a file with a directory of the same name",
			base:   files("sec", "old"),
			remote: files("sec/page.md", "new"),
			live:   files("sec", "old"),
			want: contentChanges{
				writes:  write("sec/page.md", "new"),
				deletes: []string{"sec"},
			},
		},
		{
			name:   "remote replaces a directory with a file of the same name",
			base:   files("sec/page.md", "old"),
			remote: files("sec", "new"),
			live:   files("sec/page.md", "old"),
			want: contentChanges{
				writes:  write("sec", "new"),
				deletes: []string{"sec/page.md"},
			},
		},
		{
			name:         "unborn branch skips a remote file already present with the same content",
			remote:       files("p.md", "remote"),
			live:         files("p.md", "remote"),
			unbornBranch: true,
		},
		{
			name:     "case fold: remote case-only rename of a locally edited file conflicts on both names",
			base:     files("Foo.md", "page"),
			remote:   files("foo.md", "page"),
			live:     files("Foo.md", "edited"),
			caseFold: true,
			want:     contentChanges{conflicts: []string{"Foo.md", "foo.md"}},
		},
		{
			name:          "remote and local edit the same file to equivalent content",
			base:          files("p.md", "base"),
			remote:        files("p.md", "remote"),
			live:          files("p.md", "local"),
			equivalent:    always,
			wantConsulted: []string{"p.md"},
		},
		{
			name:          "remote and local edit the same file to different content",
			base:          files("p.md", "base"),
			remote:        files("p.md", "remote"),
			live:          files("p.md", "local"),
			equivalent:    never,
			wantConsulted: []string{"p.md"},
			want:          contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:          "remote adds a file that exists locally with equivalent content",
			remote:        files("new.md", "remote"),
			live:          files("new.md", "local"),
			equivalent:    always,
			wantConsulted: []string{"new.md"},
		},
		{
			// Equivalence is only asked where a conflict would be recorded,
			// so the remote's save metadata still reaches an unedited file.
			name:       "live matches base, remote changes metadata only, still written",
			base:       files("p.md", "base"),
			remote:     files("p.md", "remote"),
			live:       files("p.md", "base"),
			equivalent: always,
			want:       contentChanges{writes: write("p.md", "remote")},
		},
		{
			name:       "equivalence is not asked for a remote delete of a locally edited file",
			base:       files("p.md", "base"),
			remote:     files(),
			live:       files("p.md", "local"),
			equivalent: always,
			want:       contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:       "equivalence is not asked for a remote edit of a locally deleted file",
			base:       files("p.md", "base"),
			remote:     files("p.md", "remote"),
			live:       files(),
			equivalent: always,
			want:       contentChanges{conflicts: []string{"p.md"}},
		},
		{
			name:         "equivalence is not asked on an unborn branch",
			remote:       files("p.md", "remote"),
			live:         files("p.md", "local"),
			unbornBranch: true,
			equivalent:   always,
		},
		{
			// Keeping a file found under another case would let a delete of
			// its old-case name in the same pull remove it (a remote
			// case-only rename plus a metadata change), so it stays a conflict.
			name:       "equivalence is not asked for a file found under another case",
			base:       files("Foo.md", "page"),
			remote:     files("foo.md", "remote"),
			live:       files("Foo.md", "page"),
			caseFold:   true,
			equivalent: always,
			want: contentChanges{
				deletes:   []string{"Foo.md"},
				conflicts: []string{"foo.md"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var consulted []string
			var equivalent func(rel string, live, remote plumbing.Hash) bool
			if tc.equivalent != nil {
				equivalent = func(rel string, live, remote plumbing.Hash) bool {
					consulted = append(consulted, rel)
					if live != tc.live[rel] || remote != tc.remote[rel] {
						t.Errorf("equivalent(%q) got live %s remote %s, want %s %s", rel, live, remote, tc.live[rel], tc.remote[rel])
					}
					return tc.equivalent(rel)
				}
			}
			got := planContentChanges(tc.base, tc.remote, tc.live, tc.caseFold, !tc.unbornBranch, equivalent)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("planContentChanges() = %+v, want %+v", got, tc.want)
			}
			if !reflect.DeepEqual(consulted, tc.wantConsulted) {
				t.Errorf("equivalence consulted for %q, want %q", consulted, tc.wantConsulted)
			}
		})
	}
}

// TestPageContentEquivalent_Cases_FallsBackToNotEquivalent tests the check a
// pull asks before recording a conflict: pages are compared by content, and
// anything it cannot compare that way stays a conflict.
func TestPageContentEquivalent_Cases_FallsBackToNotEquivalent(t *testing.T) {
	const (
		remotePage = "---\nleafwiki_id: p1\nleafwiki_updated_at: 2026-01-01T00:00:00Z\n---\nBody\n"
		savedPage  = "---\nleafwiki_id: p1\nleafwiki_updated_at: 2026-02-02T00:00:00Z\n---\nBody\n"
		brokenPage = "---\nleafwiki_id: [invalid: yaml: structure\n---\nBody\n"
	)
	tests := []struct {
		name      string
		rel       string
		live      string
		remote    string
		hashedAs  string // what the live file held when it was hashed; defaults to live
		missing   bool   // the remote blob is not in the repository
		wantEqual bool
	}{
		{name: "page differing only in save metadata", rel: "p.md", live: savedPage, remote: remotePage, wantEqual: true},
		{name: "upper-case .MD extension", rel: "p.MD", live: savedPage, remote: remotePage, wantEqual: true},
		{name: "page with other body", rel: "p.md", live: strings.Replace(savedPage, "Body", "Other", 1), remote: remotePage},
		{name: "non-page file with the same text", rel: "p.txt", live: savedPage, remote: remotePage},
		{name: "unparsable frontmatter", rel: "p.md", live: brokenPage, remote: brokenPage + " "},
		{name: "live file changed since it was hashed", rel: "p.md", live: savedPage, remote: remotePage, hashedAs: "older\n"},
		{name: "live file missing", rel: "gone.md", remote: remotePage},
		{name: "remote blob missing", rel: "p.md", live: savedPage, remote: remotePage, missing: true},
		{name: "unicode and special characters in a nested file name", rel: "Über/日本 page & (1).md", live: savedPage, remote: remotePage, wantEqual: true},
		// The file exists next to the live directory, so only the path check keeps it out.
		{name: "path escaping the live directory", rel: "../p.md", live: savedPage, remote: remotePage},
		{name: "asset with the same text under another extension", rel: "img.md.png", live: savedPage, remote: remotePage},
		{name: "page whose frontmatter breaks only on the remote", rel: "p.md", live: savedPage, remote: brokenPage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, rootDir := newRepoWithRemote(t, initBareRemote(t))
			if tc.live != "" {
				writeFileHelper(t, rootDir, tc.rel, tc.live)
			}
			hashedAs := tc.live
			if tc.hashedAs != "" {
				hashedAs = tc.hashedAs
			}
			remote := plumbing.ComputeHash(plumbing.BlobObject, []byte(tc.remote))
			if !tc.missing {
				var err error
				if remote, err = repo.writeBlob([]byte(tc.remote)); err != nil {
					t.Fatalf("writeBlob: %v", err)
				}
			}

			live := plumbing.ComputeHash(plumbing.BlobObject, []byte(hashedAs))
			if got := repo.pageContentEquivalent(rootDir)(tc.rel, live, remote); got != tc.wantEqual {
				t.Errorf("pageContentEquivalent(%q) = %v, want %v", tc.rel, got, tc.wantEqual)
			}
		})
	}
}

// TestApplyMaterialize_DeleteAndWriteSamePath_KeepsWrittenFile tests that the
// apply pass runs deletes before writes. On a case-insensitive file system a
// remote case-only rename plans a delete of "Foo.md" and a write of "foo.md"
// that hit the same physical file; written in the other order, the page is
// deleted. The same path for both stands in for that on every host.
func TestApplyMaterialize_DeleteAndWriteSamePath_KeepsWrittenFile(t *testing.T) {
	repo, rootDir := newRepoWithRemote(t, initBareRemote(t))
	writeFileHelper(t, rootDir, "Foo.md", "old\n")
	blob, err := repo.writeBlob([]byte("new\n"))
	if err != nil {
		t.Fatalf("writeBlob: %v", err)
	}

	err = repo.applyMaterialize([]targetPlan{{
		target: contentTarget{treePath: "root", liveDir: rootDir},
		contentChanges: contentChanges{
			deletes: []string{"Foo.md"},
			writes:  []plannedWrite{{rel: "Foo.md", blob: blob}},
		},
	}})
	if err != nil {
		t.Fatalf("applyMaterialize: %v", err)
	}
	readFileEqual(t, filepath.Join(rootDir, "Foo.md"), "new\n")
}

// TestApplyMaterialize_PathOutsideLiveDir_RefusesWithoutTouchingIt tests the
// apply pass's own containment check, independent of tree entry validation: a
// planned write or delete that resolves outside the live directory fails and
// leaves the file there alone.
func TestApplyMaterialize_PathOutsideLiveDir_RefusesWithoutTouchingIt(t *testing.T) {
	tests := []struct {
		name    string
		changes func(blob plumbing.Hash) contentChanges
	}{
		{
			name: "write",
			changes: func(blob plumbing.Hash) contentChanges {
				return contentChanges{writes: []plannedWrite{{rel: "../existing.txt", blob: blob}}}
			},
		},
		{
			name: "delete",
			changes: func(plumbing.Hash) contentChanges {
				return contentChanges{deletes: []string{"../existing.txt"}}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, rootDir := newRepoWithRemote(t, initBareRemote(t))
			existing := filepath.Join(filepath.Dir(rootDir), "existing.txt")
			writeFileHelper(t, filepath.Dir(rootDir), "existing.txt", "original")
			blob, err := repo.writeBlob([]byte("overwritten"))
			if err != nil {
				t.Fatalf("writeBlob: %v", err)
			}

			err = repo.applyMaterialize([]targetPlan{{
				target:         contentTarget{treePath: "root", liveDir: rootDir},
				contentChanges: tc.changes(blob),
			}})
			if err == nil {
				t.Fatal("expected applyMaterialize to refuse a path outside the live directory")
			}
			readFileEqual(t, existing, "original")
		})
	}
}

// TestApplyMaterialize_LastWriteOutsideLiveDir_TouchesNothing tests that the
// apply pass resolves every path before it changes anything, so a path that
// leaves the live directory late in the plan does not leave earlier deletes
// and writes applied.
func TestApplyMaterialize_LastWriteOutsideLiveDir_TouchesNothing(t *testing.T) {
	repo, rootDir := newRepoWithRemote(t, initBareRemote(t))
	writeFileHelper(t, rootDir, "old.md", "old\n")
	blob, err := repo.writeBlob([]byte("new\n"))
	if err != nil {
		t.Fatalf("writeBlob: %v", err)
	}

	err = repo.applyMaterialize([]targetPlan{{
		target: contentTarget{treePath: "root", liveDir: rootDir},
		contentChanges: contentChanges{
			deletes: []string{"old.md"},
			writes:  []plannedWrite{{rel: "a.md", blob: blob}, {rel: "../z.md", blob: blob}},
		},
	}})
	if err == nil {
		t.Fatal("expected applyMaterialize to refuse a path outside the live directory")
	}
	readFileEqual(t, filepath.Join(rootDir, "old.md"), "old\n")
	for _, p := range []string{filepath.Join(rootDir, "a.md"), filepath.Join(filepath.Dir(rootDir), "z.md")} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s exists after a refused apply (err %v)", p, err)
		}
	}
}
