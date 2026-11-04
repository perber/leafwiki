package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// renderPage renders a page-read result as the text a page tool returns: a
// short header carrying the page's identifying metadata, then the markdown body
// with 4-digit line numbers so an agent can reference lines precisely. It is
// shared by get_page and get_page_by_path so their output stays consistent.
func renderPage(p *Page) string {
	return pageHeader(p) + withLineNumbers(p.Content)
}

// pageHeader renders the identifying-metadata half of a page-read result: the
// title line, the metadata lines, and the blank line that separates the header
// from the body. Shared by renderPage and renderPageWindow so the header is
// byte-identical in the full and the windowed rendering.
func pageHeader(p *Page) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n", p.Title)
	fmt.Fprintf(&sb, "ID: %s\n", p.ID)
	if p.Path != "" {
		fmt.Fprintf(&sb, "Path: %s\n", p.Path)
	}
	fmt.Fprintf(&sb, "Kind: %s\n", p.Kind)
	if !p.CreatedAt.IsZero() {
		fmt.Fprintf(&sb, "Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	if !p.UpdatedAt.IsZero() {
		fmt.Fprintf(&sb, "Updated: %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
	if len(p.Tags) > 0 {
		fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	if len(p.Properties) > 0 {
		sb.WriteString("Properties:\n")
		for _, key := range sortedKeys(p.Properties) {
			fmt.Fprintf(&sb, "  %s: %s\n", key, p.Properties[key])
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// renderPageWindow renders the page-read result windowed into a slice of the
// body: the same header as renderPage, then the lines [offset, end) of the
// content. offset is the zero-based index of the first line to read; lines is
// how many lines to return, a non-positive value reading all remaining, and
// end clamps to the page's last line. The body's line numbers stay absolute
// 1-based over the whole document — the same line model withLineNumbers and
// ApplyPatch use — so patch_page's ranges keep referencing the numbers the
// agent read, even in a mid-page window. A window that ends before the page's
// last line is followed by a truncation note stating the shown range and the
// page's total; an offset past the end is an error the caller reports as
// invalid input. With offset 0 and no lines the output is exactly
// renderPage's.
func renderPageWindow(p *Page, offset, lines int) (string, error) {
	body := strings.Split(p.Content, "\n") // same split as withLineNumbers and ApplyPatch
	total := len(body)
	if offset >= total {
		return "", fmt.Errorf("offset %d beyond end of page (page has %d lines)", offset, total)
	}
	end := total
	if lines > 0 {
		// Clamped before the add: lines is unbounded client input, and
		// offset+lines would wrap negative for a lines past the page's end
		// (a wrapped end renders an empty window and a nonsense note).
		if remaining := total - offset; lines > remaining {
			lines = remaining
		}
		end = offset + lines
	}
	var sb strings.Builder
	for i := offset; i < end; i++ {
		fmt.Fprintf(&sb, "%04d | %s", i+1, body[i])
		if i < end-1 {
			sb.WriteByte('\n')
		}
	}
	if end < total {
		fmt.Fprintf(&sb, "\n\n(Showing lines %d–%d of %d)", offset+1, end, total)
	}
	return pageHeader(p) + sb.String(), nil
}

// withLineNumbers prefixes every line of the content with a 4-digit, 1-based
// line number and a " | " separator, so an agent can quote a line unambiguously.
func withLineNumbers(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%04d | %s", i+1, line)
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// renderRevisions renders a page's revision history, newest first (the order
// the store returns it): one line per revision carrying the ID the agent can
// hand to get_page_revision and restore_page_revision, the type, the author's
// ID, the creation time and an optional summary. An empty history is reported
// plainly, like an empty search.
func renderRevisions(id string, res *ListRevisionsResult) string {
	if res == nil || len(res.Revisions) == 0 {
		return "No revisions found for page " + id + "."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Revisions for page %s (newest first):\n", id)
	for _, r := range res.Revisions {
		fmt.Fprintf(&sb, "- %s (%s) by %s at %s", r.ID, r.Type, r.AuthorID, r.CreatedAt.Format("2006-01-02 15:04:05"))
		if r.Summary != "" {
			fmt.Fprintf(&sb, " — %s", r.Summary)
		}
		sb.WriteByte('\n')
	}
	if res.NextCursor != "" {
		fmt.Fprintf(&sb, "\nNext page cursor: %s", res.NextCursor)
	}
	return sb.String()
}

// renderRevision renders one revision (the get_page_revision output): its
// metadata, the content with line numbers, and the assets (omitted when the
// revision carries none).
func renderRevision(snap *RevisionSnapshot) string {
	var sb strings.Builder
	if rev := snap.Revision; rev != nil {
		fmt.Fprintf(&sb, "# Revision %s\n", rev.ID)
		fmt.Fprintf(&sb, "Type: %s\n", rev.Type)
		if rev.AuthorID != "" {
			fmt.Fprintf(&sb, "Author: %s\n", rev.AuthorID)
		}
		if !rev.CreatedAt.IsZero() {
			fmt.Fprintf(&sb, "Created: %s\n", rev.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		if rev.Title != "" {
			fmt.Fprintf(&sb, "Title: %s\n", rev.Title)
		}
		if rev.Path != "" {
			fmt.Fprintf(&sb, "Path: %s\n", rev.Path)
		}
		if rev.Summary != "" {
			fmt.Fprintf(&sb, "Summary: %s\n", rev.Summary)
		}
		sb.WriteString("\n## Content\n\n")
	}
	sb.WriteString(withLineNumbers(snap.Content))
	if len(snap.Assets) > 0 {
		sb.WriteString("\n\n## Assets\n\n")
		for _, a := range snap.Assets {
			fmt.Fprintf(&sb, "- %s (%d bytes", a.Name, a.SizeBytes)
			if a.MIMEType != "" {
				fmt.Fprintf(&sb, ", %s", a.MIMEType)
			}
			sb.WriteString(")\n")
		}
	}
	return sb.String()
}

// renderComparison renders the compare_page_revisions output: the content-change
// flag, the asset differences and both contents with line numbers. No diff is
// computed here — the comparison already decided what changed (the REST
// comparison shape, not a re-implementation).
func renderComparison(cmp *RevisionComparison) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Comparison: %s → %s\n\n", cmp.Base.Revision.ID, cmp.Target.Revision.ID)
	if cmp.ContentChanged {
		sb.WriteString("Content changed: yes\n")
	} else {
		sb.WriteString("Content changed: no\n")
	}
	if len(cmp.AssetChanges) > 0 {
		parts := make([]string, 0, len(cmp.AssetChanges))
		for _, c := range cmp.AssetChanges {
			parts = append(parts, c.Name+" "+c.Status)
		}
		fmt.Fprintf(&sb, "Asset changes: %s\n", strings.Join(parts, "; "))
	} else {
		sb.WriteString("Asset changes: none\n")
	}
	fmt.Fprintf(&sb, "\n## Base (%s)\n\n%s\n\n## Target (%s)\n\n%s",
		cmp.Base.Revision.ID, withLineNumbers(cmp.Base.Content),
		cmp.Target.Revision.ID, withLineNumbers(cmp.Target.Content))
	return sb.String()
}

// sortedKeys returns a map's keys in sorted order so property rendering is
// deterministic across calls.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parsePatch decodes the patch_page patch argument, a JSON array of range
// objects ({"start","end","replace"}, see PatchRange), and returns the parsed
// ranges. It rejects the wire shape (non-array, empty array, malformed
// objects) and the range shape (start below 1, end before start); line
// bounds against the page are checked in ApplyPatch, where the content is
// known.
func parsePatch(raw string) ([]PatchRange, error) {
	var ranges []PatchRange
	if err := json.Unmarshal([]byte(raw), &ranges); err != nil {
		return nil, fmt.Errorf("patch must be a JSON array of range objects: %v", err)
	}
	if len(ranges) == 0 {
		return nil, fmt.Errorf("patch must contain at least one range")
	}
	for i, r := range ranges {
		if r.Start < 1 {
			return nil, fmt.Errorf("patch range %d: start must be >= 1, got %d", i+1, r.Start)
		}
		if r.End < r.Start {
			return nil, fmt.Errorf("patch range %d: end %d is before start %d", i+1, r.End, r.Start)
		}
	}
	return ranges, nil
}

// ApplyPatch replaces each range's lines with its replacement text and
// returns the new content. Line numbers are 1-based and inclusive; an empty
// replacement deletes the range's lines; a replacement may span multiple
// lines. The ranges are applied after sorting, must not overlap, and must
// stay within the content: any violation is a descriptive error (the caller
// reports it to the agent, who can re-read the page and retry).
func ApplyPatch(content string, ranges []PatchRange) (string, error) {
	if len(ranges) == 0 {
		return "", fmt.Errorf("patch must contain at least one range")
	}
	lines := strings.Split(content, "\n")
	total := len(lines)

	sorted := make([]PatchRange, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Start <= sorted[i-1].End {
			return "", fmt.Errorf("patch ranges overlap at line %d (%d-%d and %d-%d)",
				sorted[i].Start, sorted[i-1].Start, sorted[i-1].End, sorted[i].Start, sorted[i].End)
		}
	}
	for _, r := range sorted {
		switch {
		case r.Start < 1:
			return "", fmt.Errorf("patch range starts at line %d, which is below the first line", r.Start)
		case r.End < r.Start:
			return "", fmt.Errorf("patch range ends at line %d, before its start line %d", r.End, r.Start)
		case r.Start > total:
			return "", fmt.Errorf("patch range starts at line %d, beyond the end of the page (page has %d lines)", r.Start, total)
		case r.End > total:
			return "", fmt.Errorf("patch range ends at line %d, beyond the end of the page (page has %d lines)", r.End, total)
		}
	}

	result := make([]string, 0, len(lines))
	cursor := 0 // index of the next untouched line
	for _, r := range sorted {
		result = append(result, lines[cursor:r.Start-1]...)
		cursor = r.End
		if r.Replace != "" {
			result = append(result, strings.Split(r.Replace, "\n")...)
		}
	}
	result = append(result, lines[cursor:]...)
	return strings.Join(result, "\n"), nil
}
