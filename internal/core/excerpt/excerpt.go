package excerpt

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/perber/wiki/internal/core/markdown"
	"github.com/yuin/goldmark"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

const MaxRunes = 180

var mdRenderer = goldmark.New(
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

var (
	sanitize = bluemonday.StrictPolicy()

	fencedCodePattern    = regexp.MustCompile("(?s)```.*?```")
	imagePattern         = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	linkPattern          = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	htmlPattern          = regexp.MustCompile(`<[^>]+>`)
	linePrefixPattern    = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s*|[-*+]\s+|\d+\.\s+|>\s?)`)
	wikiImagePattern     = regexp.MustCompile(`!\[\[[^\]\n]*\]\]`)
	wikiLinkAliasPattern = regexp.MustCompile(`\[\[[^\]|\n]+\|([^\]\n]+)\]\]`)
	wikiLinkPattern      = regexp.MustCompile(`\[\[([^\]\n]+)\]\]`)

	shoutoutOpenPattern  = regexp.MustCompile(`^ {0,3}:::\s*(?P<type>[A-Za-z][\w-]*)\s*$`)
	shoutoutClosePattern = regexp.MustCompile(`^ {0,3}:::\s*$`)
	fencePattern         = regexp.MustCompile(`^ {0,3}(?P<marker>` + "`{3,}|~{3,}" + `).*$`)

	shoutoutTypeIdx = shoutoutOpenPattern.SubexpIndex("type")
	fenceMarkerIdx  = fencePattern.SubexpIndex("marker")
)

type fenceState struct {
	markerChar   byte
	markerLength int
}

// FromContent parses markdown, skips frontmatter, and returns a short plain-text excerpt.
func FromContent(raw string) string {
	_, body, _, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		body = raw
	}
	return FromBody(body)
}

// FromBody converts markdown or HTML-ish body content into a short plain-text excerpt.
func FromBody(body string) string {
	text := PlainTextFromMarkdown(body)

	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= MaxRunes {
		return text
	}

	truncated := strings.TrimSpace(string(runes[:MaxRunes]))
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace >= 120 {
		truncated = truncated[:lastSpace]
	}
	return strings.TrimSpace(truncated) + "..."
}

// NormalizeMarkdownBody removes non-content markdown constructs that should not leak into excerpts or search text.
func NormalizeMarkdownBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	output := make([]string, 0, len(lines))
	var outerFence *fenceState

	for _, line := range lines {
		if outerFence != nil {
			output = append(output, line)
			outerFence = getFenceState(line, outerFence)
			continue
		}

		if match := shoutoutOpenPattern.FindStringSubmatch(line); match != nil {
			output = append(output, match[shoutoutTypeIdx])
			continue
		}

		if shoutoutClosePattern.MatchString(line) {
			continue
		}

		output = append(output, line)
		outerFence = getFenceState(line, outerFence)
	}

	return strings.Join(output, "\n")
}

// PlainTextFromMarkdown converts markdown body content into plain text compatible with search snippets and tag excerpts.
func PlainTextFromMarkdown(body string) string {
	normalized := NormalizeMarkdownBody(body)
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = fencedCodePattern.ReplaceAllString(normalized, " ")
	normalized = imagePattern.ReplaceAllString(normalized, "$1")
	normalized = wikiImagePattern.ReplaceAllString(normalized, " ")
	normalized = wikiLinkAliasPattern.ReplaceAllString(normalized, "$1")
	normalized = wikiLinkPattern.ReplaceAllString(normalized, "$1")
	var htmlBuf bytes.Buffer
	if err := mdRenderer.Convert([]byte(normalized), &htmlBuf); err != nil {
		htmlBuf.Reset()
		htmlBuf.Write([]byte(normalized))
	}
	text := sanitize.Sanitize(htmlBuf.String())
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = fencedCodePattern.ReplaceAllString(text, " ")
	text = imagePattern.ReplaceAllString(text, "$1")
	text = linkPattern.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "`", "")
	text = htmlPattern.ReplaceAllString(text, " ")
	text = linePrefixPattern.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(text), " ")
}

func getFenceState(line string, currentFence *fenceState) *fenceState {
	match := fencePattern.FindStringSubmatch(line)
	if match == nil {
		return currentFence
	}

	marker := match[fenceMarkerIdx]
	if marker == "" {
		return currentFence
	}

	if currentFence == nil {
		return &fenceState{
			markerChar:   marker[0],
			markerLength: len(marker),
		}
	}

	if marker[0] == currentFence.markerChar && len(marker) >= currentFence.markerLength {
		return nil
	}

	return currentFence
}
