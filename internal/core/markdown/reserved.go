package markdown

import "strings"

// IsSystemKey reports whether a frontmatter key is managed by LeafWiki and
// must never be surfaced as a user-editable property.
//
// Currently reserved: "tags" and any key with the "leafwiki_" prefix.
// "title" is intentionally NOT reserved here: when leafwiki_title is
// explicitly set (Frontmatter.HasLeafWikiTitle == true), a coexisting "title"
// key is treated as a user-defined custom property and must round-trip
// unchanged through the editor.
func IsSystemKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return lower == "tags" || strings.HasPrefix(lower, "leafwiki_")
}
