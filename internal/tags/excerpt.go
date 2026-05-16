package tags

import (
	"regexp"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
)

const excerptMaxRunes = 180

var (
	excerptFencedCodePattern = regexp.MustCompile("(?s)```.*?```")
	excerptImagePattern      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	excerptLinkPattern       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	excerptHTMLPattern       = regexp.MustCompile(`<[^>]+>`)
	excerptLinePrefixPattern = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s*|[-*+]\s+|\d+\.\s+|>\s?)`)
)

// ExtractExcerptFromContent parses markdown (including frontmatter) and returns a short plain-text excerpt.
func ExtractExcerptFromContent(raw string) string {
	_, body, _, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		body = raw
	}
	return buildExcerpt(body)
}

func buildExcerpt(body string) string {
	text := strings.ReplaceAll(body, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = excerptFencedCodePattern.ReplaceAllString(text, " ")
	text = excerptImagePattern.ReplaceAllString(text, "$1")
	text = excerptLinkPattern.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "`", "")
	text = excerptHTMLPattern.ReplaceAllString(text, " ")
	text = excerptLinePrefixPattern.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")

	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= excerptMaxRunes {
		return text
	}

	truncated := strings.TrimSpace(string(runes[:excerptMaxRunes]))
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace >= 120 {
		truncated = truncated[:lastSpace]
	}
	return strings.TrimSpace(truncated) + "..."
}
