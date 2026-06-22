package markdown

import "strings"

// IsSystemKey reports whether a frontmatter key is managed by LeafWiki and
// must never be surfaced as a user-editable property.
//
// Reserved: "tags" and any key with the "leafwiki_" prefix.
// "title" is not reserved — it is always treated as a user-defined custom
// property and round-trips through the editor unchanged.
func IsSystemKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	return lower == "tags" || strings.HasPrefix(lower, "leafwiki_")
}
