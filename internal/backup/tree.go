package backup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// This file implements LeafWiki's git backup using git plumbing rather than the
// working tree: the live root/ and assets/ directories are turned into blob/tree
// objects and spliced into the repository tree at Config.ContentTreePaths(),
// then committed. Nothing is ever copied into the repository working tree, so a
// monorepo remote keeps files outside the configured prefix untouched.

// errPullConflict marks a remote content change that would clobber a dirty live
// file. materializeContent returns it wrapped in a *pullConflictError;
// pullBeforeBackup reports it as NeedsIntervention.
var errPullConflict = errors.New("pull conflict")

// writeBlob stores data as a git blob object and returns its hash. Content
// that is already in the object store (loose or packed) is not written again,
// so an unchanged backup run costs hashing only.
func (r *Repository) writeBlob(data []byte) (plumbing.Hash, error) {
	h := plumbing.ComputeHash(plumbing.BlobObject, data)
	if r.repo.Storer.HasEncodedObject(h) == nil {
		return h, nil
	}
	obj := r.repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	w, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return plumbing.ZeroHash, err
	}
	if err := w.Close(); err != nil {
		return plumbing.ZeroHash, err
	}
	return r.repo.Storer.SetEncodedObject(obj)
}

// writeTree stores a tree object built from entries and returns its hash. The
// entries are sorted into git's canonical order (directories as if they had a
// trailing slash) before encoding.
func (r *Repository) writeTree(entries []object.TreeEntry) (plumbing.Hash, error) {
	sort.Sort(object.TreeEntrySorter(entries))
	tree := &object.Tree{Entries: entries}
	obj := r.repo.Storer.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	if h := obj.Hash(); r.repo.Storer.HasEncodedObject(h) == nil {
		return h, nil
	}
	return r.repo.Storer.SetEncodedObject(obj)
}

// emptyTreeHash writes (or reuses) git's empty tree object.
func (r *Repository) emptyTreeHash() (plumbing.Hash, error) {
	return r.writeTree(nil)
}

// buildDirTree writes blob/tree objects for everything under dir and returns
// the resulting tree hash. present is false when dir is missing or holds no
// files at all — git cannot represent empty directories.
//
// Patterns from .gitignore files (the repository root, any intermediate
// directories down to dir, and nested files) are honoured, matching the old
// working-tree staging. rel is dir's path components relative to the repository
// root and patterns are the ignore rules accumulated for its ancestors, so a
// pattern's directory scope is preserved. This is what keeps transient atomic-
// write files (.tmp-*) and runtime files out of the backup. Nested ".git"
// directories are skipped. Symlinks are stored as git symlinks and executable
// bits are preserved; sockets/devices/FIFOs are ignored.
func (r *Repository) buildDirTree(dir string, rel []string, patterns []gitignore.Pattern) (hash plumbing.Hash, present bool, err error) {
	own, err := ignorePatternsIn(dir, rel)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	patterns = append(append([]gitignore.Pattern{}, patterns...), own...)
	matcher := gitignore.NewMatcher(patterns)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return plumbing.ZeroHash, false, nil
		}
		return plumbing.ZeroHash, false, fmt.Errorf("read %s: %w", dir, err)
	}

	var treeEntries []object.TreeEntry
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" {
			continue
		}
		childRel := append(append([]string{}, rel...), name)
		if matcher.Match(childRel, entry.IsDir()) {
			slog.Debug("backup: skipping ignored path", "path", strings.Join(childRel, "/"))
			continue
		}
		// The pull refuses such names, so committing one would make every
		// later pull of this backup fail; leave it out instead.
		if err := validateTreeEntryName(name); err != nil {
			r.warnSkippedName(strings.Join(childRel, "/"), err)
			continue
		}
		full := filepath.Join(dir, name)

		if entry.IsDir() {
			sub, subPresent, err := r.buildDirTree(full, childRel, patterns)
			if err != nil {
				return plumbing.ZeroHash, false, err
			}
			if !subPresent {
				continue
			}
			treeEntries = append(treeEntries, object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: sub})
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return plumbing.ZeroHash, false, fmt.Errorf("stat %s: %w", full, err)
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				return plumbing.ZeroHash, false, fmt.Errorf("readlink %s: %w", full, err)
			}
			h, err := r.writeBlob([]byte(target))
			if err != nil {
				return plumbing.ZeroHash, false, err
			}
			treeEntries = append(treeEntries, object.TreeEntry{Name: name, Mode: filemode.Symlink, Hash: h})

		case info.Mode().IsRegular():
			data, err := os.ReadFile(full)
			if err != nil {
				return plumbing.ZeroHash, false, fmt.Errorf("read %s: %w", full, err)
			}
			h, err := r.writeBlob(data)
			if err != nil {
				return plumbing.ZeroHash, false, err
			}
			mode := filemode.Regular
			if info.Mode().Perm()&0o111 != 0 {
				mode = filemode.Executable
			}
			treeEntries = append(treeEntries, object.TreeEntry{Name: name, Mode: mode, Hash: h})

		default:
			slog.Debug("backup: skipping non-regular file", "path", full, "mode", info.Mode().String())
		}
	}

	if len(treeEntries) == 0 {
		return plumbing.ZeroHash, false, nil
	}
	h, err := r.writeTree(treeEntries)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	return h, true, nil
}

// warnSkippedName logs once per path that buildDirTree left it out of the
// backup. The path is quoted so control characters stay readable in the log.
func (r *Repository) warnSkippedName(path string, reason error) {
	if _, seen := r.skippedNames[path]; seen {
		return
	}
	if r.skippedNames == nil {
		r.skippedNames = map[string]struct{}{}
	}
	r.skippedNames[path] = struct{}{}
	slog.Warn("backup: skipping a file name the backup cannot store on this system; rename it to include it in the backup",
		"path", strconv.Quote(path), "reason", reason.Error())
}

// ignorePatternsIn reads dir/.gitignore if present and parses its patterns with
// domain set to dir's path components relative to the repository root.
func ignorePatternsIn(dir string, domain []string) ([]gitignore.Pattern, error) {
	f, err := os.Open(filepath.Join(dir, ".gitignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		patterns = append(patterns, gitignore.ParsePattern(line, domain))
	}
	return patterns, scanner.Err()
}

// ancestorIgnorePatterns returns target's path components relative to the
// repository root together with the ignore patterns declared by .gitignore files
// from the repository root down to (but excluding) target.
func (r *Repository) ancestorIgnorePatterns(target string) ([]string, []gitignore.Pattern, error) {
	rel, err := filepath.Rel(r.repoDir, target)
	if err != nil {
		return nil, nil, err
	}
	var parts []string
	if slash := filepath.ToSlash(rel); slash != "." && slash != "" {
		parts = strings.Split(slash, "/")
	}

	var patterns []gitignore.Pattern
	for i := 0; i < len(parts); i++ {
		dir := r.repoDir
		if i > 0 {
			dir = filepath.Join(r.repoDir, filepath.FromSlash(strings.Join(parts[:i], "/")))
		}
		ps, err := ignorePatternsIn(dir, parts[:i])
		if err != nil {
			return nil, nil, err
		}
		patterns = append(patterns, ps...)
	}
	return parts, patterns, nil
}

// buildContentReplacements builds the tree for the live root/ and assets/
// directories and maps each to the repository-relative path it belongs at.
func (r *Repository) buildContentReplacements() (map[string]plumbing.Hash, error) {
	targets := r.cfg.contentTargets()

	replacements := make(map[string]plumbing.Hash, len(targets))
	for _, t := range targets {
		rel, patterns, err := r.ancestorIgnorePatterns(t.liveDir)
		if err != nil {
			return nil, err
		}
		hash, _, err := r.buildDirTree(t.liveDir, rel, patterns)
		if err != nil {
			return nil, err
		}
		// hash is the zero hash when present is false (dir missing or empty) —
		// a zero value here means "remove this path", matching spliceTree.
		replacements[t.treePath] = hash
	}
	return replacements, nil
}

// spliceTree returns a tree based on base but with every path in replacements
// created, replaced or removed. A zero base is treated as an empty tree. A
// zero hash in replacements removes that path; every other value is a
// directory hash. The returned bool is false when the result would be an
// empty tree.
func (r *Repository) spliceTree(base plumbing.Hash, replacements map[string]plumbing.Hash) (plumbing.Hash, bool, error) {
	direct := map[string]plumbing.Hash{}
	groups := map[string]map[string]plumbing.Hash{}
	for p, hash := range replacements {
		seg, rest, found := strings.Cut(p, "/")
		if !found {
			direct[seg] = hash
			continue
		}
		if groups[seg] == nil {
			groups[seg] = map[string]plumbing.Hash{}
		}
		groups[seg][rest] = hash
	}

	var entries []object.TreeEntry
	if !base.IsZero() {
		tree, err := object.GetTree(r.repo.Storer, base)
		if err != nil {
			if !errors.Is(err, plumbing.ErrObjectNotFound) {
				return plumbing.ZeroHash, false, fmt.Errorf("read tree %s: %w", base, err)
			}
		} else {
			entries = append(entries, tree.Entries...)
		}
	}

	for name, hash := range direct {
		if e := findTreeEntry(entries, name); e != nil && e.Mode != filemode.Dir {
			return plumbing.ZeroHash, false, fmt.Errorf(
				"backup path segment %q collides with an existing file in the repository; "+
					"choose a --git-backup-path that does not overlap with existing content", name)
		}
		entries = upsertTreeEntry(entries, name, hash)
	}
	for seg, subs := range groups {
		var childBase plumbing.Hash
		if e := findTreeEntry(entries, seg); e != nil {
			if e.Mode != filemode.Dir {
				return plumbing.ZeroHash, false, fmt.Errorf(
					"backup path segment %q collides with an existing file in the repository; "+
						"choose a --git-backup-path that does not overlap with existing content", seg)
			}
			childBase = e.Hash
		}
		hash, _, err := r.spliceTree(childBase, subs)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		entries = upsertTreeEntry(entries, seg, hash)
	}

	if len(entries) == 0 {
		return plumbing.ZeroHash, false, nil
	}
	h, err := r.writeTree(entries)
	if err != nil {
		return plumbing.ZeroHash, false, err
	}
	return h, true, nil
}

func findTreeEntry(entries []object.TreeEntry, name string) *object.TreeEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

// upsertTreeEntry replaces the entry named name, or removes it when hash is
// zero. Every spliced target is a directory.
func upsertTreeEntry(entries []object.TreeEntry, name string, hash plumbing.Hash) []object.TreeEntry {
	out := entries[:0]
	for _, e := range entries {
		if e.Name != name {
			out = append(out, e)
		}
	}
	if !hash.IsZero() {
		out = append(out, object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: hash})
	}
	return out
}

// commitTree writes a commit object for treeHash with the given parents.
func (r *Repository) commitTree(treeHash plumbing.Hash, parents []plumbing.Hash, message string) (plumbing.Hash, error) {
	sig := object.Signature{Name: r.cfg.AuthorName, Email: r.cfg.AuthorEmail, When: time.Now()}
	commit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      message,
		TreeHash:     treeHash,
		ParentHashes: parents,
	}
	obj := r.repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return r.repo.Storer.SetEncodedObject(obj)
}

// headCommit returns the current HEAD commit, or (nil, nil) when the branch is
// unborn (a freshly initialised repository).
func (r *Repository) headCommit() (*object.Commit, error) {
	head, err := r.repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil
		}
		return nil, err
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return commit, nil
}

// setHead points the configured branch (and HEAD) at hash.
func (r *Repository) setHead(hash plumbing.Hash) error {
	ref := plumbing.NewBranchReferenceName(r.cfg.Branch)
	if err := r.repo.Storer.SetReference(plumbing.NewHashReference(ref, hash)); err != nil {
		return err
	}
	return r.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, ref))
}

// readBlob returns the contents of a blob object.
func (r *Repository) readBlob(hash plumbing.Hash) ([]byte, error) {
	obj, err := r.repo.Storer.EncodedObject(plumbing.BlobObject, hash)
	if err != nil {
		return nil, err
	}
	reader, err := obj.Reader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

// subtreeAt resolves dirPath (slash-separated) inside treeHash. present is
// false when treeHash is zero or dirPath is not a directory there.
func (r *Repository) subtreeAt(treeHash plumbing.Hash, dirPath string) (plumbing.Hash, bool, error) {
	if treeHash.IsZero() {
		return plumbing.ZeroHash, false, nil
	}
	if dirPath == "" {
		return treeHash, true, nil
	}
	current := treeHash
	for _, seg := range strings.Split(dirPath, "/") {
		tree, err := object.GetTree(r.repo.Storer, current)
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				return plumbing.ZeroHash, false, nil
			}
			return plumbing.ZeroHash, false, err
		}
		entry := findTreeEntry(tree.Entries, seg)
		if entry == nil || entry.Mode != filemode.Dir {
			return plumbing.ZeroHash, false, nil
		}
		current = entry.Hash
	}
	return current, true, nil
}

// filesUnder returns a map from path relative to treeHash's dirPath to the blob
// hash of each file beneath it.
func (r *Repository) filesUnder(treeHash plumbing.Hash, dirPath string) (map[string]plumbing.Hash, error) {
	out := map[string]plumbing.Hash{}
	subtree, present, err := r.subtreeAt(treeHash, dirPath)
	if err != nil {
		return nil, err
	}
	if !present {
		return out, nil
	}
	if err := r.flattenTree(subtree, "", out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) flattenTree(treeHash plumbing.Hash, prefix string, out map[string]plumbing.Hash) error {
	tree, err := object.GetTree(r.repo.Storer, treeHash)
	if err != nil {
		return err
	}
	for _, entry := range tree.Entries {
		if err := validateTreeEntryName(entry.Name); err != nil {
			if prefix != "" {
				return fmt.Errorf("%w (in %q)", err, prefix)
			}
			return err
		}
		name := entry.Name
		if prefix != "" {
			name = prefix + "/" + name
		}
		if entry.Mode == filemode.Dir {
			if err := r.flattenTree(entry.Hash, name, out); err != nil {
				return err
			}
			continue
		}
		out[name] = entry.Hash
	}
	return nil
}

// liveFiles returns a map from path relative to dir to the git blob hash of
// each file, matching the key space of filesUnder so the two can be compared.
// A missing dir yields an empty map.
func liveFiles(dir string) (map[string]plumbing.Hash, error) {
	out := map[string]plumbing.Hash{}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var data []byte
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			data = []byte(target)
		case info.Mode().IsRegular():
			data, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		default:
			return nil
		}
		out[filepath.ToSlash(rel)] = plumbing.ComputeHash(plumbing.BlobObject, data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// syncContentFromRemote copies files that exist in commitHash's content subtree
// into the live content directories. It is additive: files already present
// locally are kept as-is (local wins) and nothing is deleted.
//
// It runs once, on the first contact with a remote that already has history,
// before the wiki loads its tree. That way this instance starts in sync with the
// backup instead of treating every remote-only file as a local deletion and
// wiping the remote down to whatever it happens to contain. Because the content
// is on disk before the wiki boots, the wiki also sees those pages and skips
// seeding its default welcome page.
func (r *Repository) syncContentFromRemote(commitHash plumbing.Hash) (int, error) {
	commit, err := r.repo.CommitObject(commitHash)
	if err != nil {
		return 0, err
	}

	targets := r.cfg.contentTargets()

	synced := 0
	for _, t := range targets {
		files, err := r.filesUnder(commit.TreeHash, t.treePath)
		if err != nil {
			return synced, fmt.Errorf("read remote %s tree: %w", t.treePath, err)
		}
		for rel, blobHash := range files {
			livePath, err := resolveLivePath(t.liveDir, rel)
			if err != nil {
				return synced, err
			}
			if _, err := os.Stat(livePath); err == nil {
				continue // keep the local copy; never overwrite
			} else if !os.IsNotExist(err) {
				return synced, err
			}
			content, err := r.readBlob(blobHash)
			if err != nil {
				return synced, fmt.Errorf("read remote blob for %s: %w", rel, err)
			}
			if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
				return synced, err
			}
			if err := os.WriteFile(livePath, content, 0o644); err != nil {
				return synced, err
			}
			synced++
			slog.Debug("backup: synced remote content", "path", t.treePath+"/"+rel)
		}
	}
	if synced > 0 {
		slog.Info("backup: synced content from remote into the local data directory", "files", synced)
	}
	return synced, nil
}

// materializeContent writes the remote tree's content subtree into the live
// root/ and assets/ directories so external edits pushed to the remote show up
// in the running wiki. It only touches the configured content directories;
// sibling files in a monorepo prefix are left alone.
//
// It plans every content directory first and writes nothing while any path
// conflicts, so a conflict never leaves remote changes half-applied on disk.
// A conflict is returned as a *pullConflictError; the caller reports it. An
// I/O failure during the apply pass can still leave some files written, but
// the next pull converges: a file that already holds the remote version counts
// as applied, not as a conflict.
func (r *Repository) materializeContent(baseCommit, remoteCommit *object.Commit) error {
	plans, err := r.planMaterialize(baseCommit, remoteCommit)
	if err != nil {
		return err
	}
	var conflicts []string
	for _, p := range plans {
		for _, rel := range p.conflicts {
			conflicts = append(conflicts, p.target.treePath+"/"+rel)
		}
	}
	if len(conflicts) > 0 {
		return &pullConflictError{Paths: conflicts}
	}
	return r.applyMaterialize(plans)
}

// pullConflictError lists the content paths (repository-relative) that changed
// on the remote and deviate locally. It matches errPullConflict.
type pullConflictError struct {
	Paths []string
}

func (e *pullConflictError) Error() string {
	return fmt.Sprintf("%v: %s", errPullConflict, strings.Join(e.Paths, ", "))
}

func (e *pullConflictError) Unwrap() error { return errPullConflict }

// targetPlan is what a pull does to one content target's live directory.
type targetPlan struct {
	target contentTarget
	contentChanges
}

// contentChanges holds paths relative to a content target, each list sorted.
type contentChanges struct {
	writes    []plannedWrite
	deletes   []string
	conflicts []string
}

type plannedWrite struct {
	rel  string
	blob plumbing.Hash
}

// planMaterialize reads the base and remote trees and the live directories of
// every content target and decides what the pull has to do. It writes nothing.
func (r *Repository) planMaterialize(baseCommit, remoteCommit *object.Commit) ([]targetPlan, error) {
	var baseTree plumbing.Hash
	if baseCommit != nil {
		baseTree = baseCommit.TreeHash
	}
	var plans []targetPlan
	for _, t := range r.cfg.contentTargets() {
		baseFiles, err := r.filesUnder(baseTree, t.treePath)
		if err != nil {
			return nil, fmt.Errorf("read base %s tree: %w", t.treePath, err)
		}
		remoteFiles, err := r.filesUnder(remoteCommit.TreeHash, t.treePath)
		if err != nil {
			return nil, fmt.Errorf("read remote %s tree: %w", t.treePath, err)
		}
		live, err := liveFiles(t.liveDir)
		if err != nil {
			return nil, fmt.Errorf("read live %s: %w", t.liveDir, err)
		}
		plans = append(plans, targetPlan{
			target:         t,
			contentChanges: planContentChanges(baseFiles, remoteFiles, live, r.liveFSCaseInsensitive, baseCommit != nil),
		})
	}
	return plans, nil
}

// planContentChanges decides, per path, how to bring the live files (live) from
// the local HEAD version (base) to the remote version (remote). All three map
// paths relative to one content target to blob hashes.
//
//   - The remote did not change the path → nothing to do.
//   - The live file already equals the remote version → already applied.
//   - checkConflicts is false (unborn local branch, no HEAD to compare
//     against) → additive and local-wins like syncContentFromRemote: write
//     only paths that are absent locally, never overwrite or delete.
//   - The live file deviates from base, edited or deleted → conflict, as
//     applying the remote change would discard a local change.
//   - Otherwise the remote version is written, or the file deleted.
//
// With caseFold, a live file recorded under a different case than the tree
// path (a local rename on a case-insensitive filesystem) is the same physical
// file the OS would write over, so it is found through a case-folded lookup.
func planContentChanges(base, remote, live map[string]plumbing.Hash, caseFold, checkConflicts bool) contentChanges {
	var liveFolded map[string]plumbing.Hash
	if caseFold {
		liveFolded = make(map[string]plumbing.Hash, len(live))
		for k, v := range live {
			liveFolded[strings.ToLower(k)] = v
		}
	}

	var out contentChanges
	paths := unionKeys(base, remote)
	sort.Strings(paths)
	for _, rel := range paths {
		baseHash, baseOK := base[rel]
		remoteHash, remoteOK := remote[rel]
		if baseOK == remoteOK && baseHash == remoteHash {
			continue
		}
		liveHash, liveOK := live[rel]
		folded := false
		if !liveOK && liveFolded != nil {
			liveHash, liveOK = liveFolded[strings.ToLower(rel)]
			folded = liveOK
		}
		if liveOK == remoteOK && liveHash == remoteHash {
			if folded {
				// Same content under another case: rewrite it anyway, so a
				// delete of the old-case name in this pull (a remote
				// case-only rename) does not remove the physical file.
				out.writes = append(out.writes, plannedWrite{rel: rel, blob: remoteHash})
			}
			continue
		}
		if !checkConflicts {
			if !liveOK && remoteOK {
				out.writes = append(out.writes, plannedWrite{rel: rel, blob: remoteHash})
			}
			continue
		}
		if liveOK != baseOK || liveHash != baseHash {
			out.conflicts = append(out.conflicts, rel)
			continue
		}
		if remoteOK {
			out.writes = append(out.writes, plannedWrite{rel: rel, blob: remoteHash})
		} else {
			out.deletes = append(out.deletes, rel)
		}
	}
	return out
}

// applyMaterialize carries out plans: deletes first, so a write that lands on
// the same physical file under a different case is not removed again. Every
// path is resolved before anything changes, so a path outside the live
// directory leaves the plan entirely unapplied.
func (r *Repository) applyMaterialize(plans []targetPlan) error {
	type resolved struct {
		deletes []string
		writes  []string
	}
	paths := make([]resolved, len(plans))
	for i, p := range plans {
		for _, rel := range p.deletes {
			livePath, err := resolveLivePath(p.target.liveDir, rel)
			if err != nil {
				return err
			}
			paths[i].deletes = append(paths[i].deletes, livePath)
		}
		for _, w := range p.writes {
			livePath, err := resolveLivePath(p.target.liveDir, w.rel)
			if err != nil {
				return err
			}
			paths[i].writes = append(paths[i].writes, livePath)
		}
	}

	for i, p := range plans {
		for _, livePath := range paths[i].deletes {
			if err := os.Remove(livePath); err != nil && !os.IsNotExist(err) {
				return err
			}
			removeEmptyParents(filepath.Dir(livePath), p.target.liveDir)
		}
		for j, w := range p.writes {
			if err := r.writeLiveBlob(paths[i].writes[j], w.blob); err != nil {
				return fmt.Errorf("read remote blob for %s: %w", w.rel, err)
			}
		}
	}
	return nil
}

// writeLiveBlob writes blobHash's content to livePath, creating parent
// directories as needed.
func (r *Repository) writeLiveBlob(livePath string, blobHash plumbing.Hash) error {
	content, err := r.readBlob(blobHash)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(livePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(livePath, content, 0o644)
}

func unionKeys(a, b map[string]plumbing.Hash) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys
}

// removeEmptyParents removes now-empty directories left behind after a file was
// deleted, stopping at (and never removing) stop.
func removeEmptyParents(dir, stop string) {
	stop = filepath.Clean(stop)
	for {
		if filepath.Clean(dir) == stop || !strings.HasPrefix(filepath.Clean(dir)+string(os.PathSeparator), stop+string(os.PathSeparator)) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
