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
	sb.WriteString(withLineNumbers(p.Content))
	return sb.String()
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
