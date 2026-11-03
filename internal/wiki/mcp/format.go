package mcp

import (
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
