package backup

import (
	"path/filepath"
	"reflect"
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

	tests := []struct {
		name         string
		base         map[string]plumbing.Hash
		remote       map[string]plumbing.Hash
		live         map[string]plumbing.Hash
		caseFold     bool
		unbornBranch bool
		want         contentChanges
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := planContentChanges(tc.base, tc.remote, tc.live, tc.caseFold, !tc.unbornBranch)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("planContentChanges() = %+v, want %+v", got, tc.want)
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
