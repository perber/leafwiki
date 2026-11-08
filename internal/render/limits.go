package render

import (
	"bytes"
	"errors"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ErrTooComplex means the page is too large or shaped so that rendering it
// would cost far more than its size suggests. The caller serves no rendered
// body then; it is not a page error.
var ErrTooComplex = errors.New("render: page too large or too complex to render")

// Limits keep one render bounded in time and memory. RenderHTML runs per
// crawler request and any anonymous client can claim to be a crawler, so a
// small page must not make it allocate gigabytes. Ordinary pages stay far
// below all of them.
const (
	maxContentBytes = 256 << 10
	// maxOutputBytes caps goldmark's HTML before sanitizing and balancing.
	maxOutputBytes = 4 << 20
	// maxTableCells: goldmark pads every table row to the header's width, so
	// n header cells and n short rows render n² cells.
	maxTableCells = 50_000
	// maxEmphasisDelimiters: goldmark's delimiter matching gets superlinear
	// on long runs of "*" and "_".
	maxEmphasisDelimiters = 20_000
	// maxNestingDepth: list and block quote markers or indentation steps
	// before a line's text, and block nesting in the parsed document. The
	// HTML parser refuses more than 512 open elements anyway; parsing that
	// deep is the cost to avoid.
	maxNestingDepth = 64
	// maxBlocks: block nodes in the document; many deeply nested lines cost
	// far more than their bytes.
	maxBlocks = 60_000
	// maxInlineDelimiters: "[", "<", "*", "_", "~" and "`" in one inline
	// container (a paragraph, a heading, a table row). goldmark matches them
	// within the container in quadratic time.
	maxInlineDelimiters = 400
	// maxInlineBytes bounds one inline container, so a quadratic shape built
	// from a character not counted above stays small too.
	maxInlineBytes = 64 << 10
	// maxPageTags: "<" on the whole page, in raw HTML blocks (closing line
	// included) and in all inline containers. x/net/html appends text that is
	// separated by end tags it ignores ("x</td>x</td>…") to one growing text
	// node, quadratic in the number of pieces, also across table rows.
	maxPageTags = 15_000
	// maxConsecutiveDefinitions: link reference definitions without a blank
	// line between them are one paragraph, which goldmark's definition parser
	// reads in quadratic time, already in the pre-pass below.
	maxConsecutiveDefinitions = 5000
)

// blockParser parses blocks only: no inline parsers and no table extension, so
// it is linear and pads no table. Its paragraphs are the inline containers the
// real render parses, without block quote or list prefixes.
var blockParser = parser.NewParser(
	parser.WithBlockParsers(parser.DefaultBlockParsers()...),
	parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	parser.WithInlineParsers([]util.PrioritizedValue{}...),
)

// checkComplexity refuses content that would break the limits. It counts
// generously, so it can only refuse a page, never let a costly one through.
func checkComplexity(content string) error {
	if len(content) > maxContentBytes {
		return ErrTooComplex
	}
	if strings.Count(content, "*")+strings.Count(content, "_") > maxEmphasisDelimiters {
		return ErrTooComplex
	}
	// Cheap per-line check first: one line of 50,000 list markers is already
	// expensive for the block parser below.
	definitions := 0
	for _, line := range strings.Split(content, "\n") {
		if nestingDepth(line) > maxNestingDepth {
			return ErrTooComplex
		}
		switch trimmed := strings.TrimLeft(line, " \t>"); {
		case strings.TrimSpace(trimmed) == "":
			definitions = 0
		case strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]:"):
			if definitions++; definitions > maxConsecutiveDefinitions {
				return ErrTooComplex
			}
		}
	}
	source := []byte(content)
	doc := blockParser.Parse(text.NewReader(source))
	blocks, depth, cells, tags := 0, 0, 0, 0
	var tooComplex bool
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		if !entering {
			depth--
			return ast.WalkContinue, nil
		}
		blocks++
		depth++
		if blocks > maxBlocks || depth > maxNestingDepth {
			tooComplex = true
			return ast.WalkStop, nil
		}
		switch n.Kind() {
		case ast.KindCodeBlock, ast.KindFencedCodeBlock:
			// Not inline-parsed.
			return ast.WalkContinue, nil
		case ast.KindHTMLBlock:
			lines := blockLines(n, source)
			if block, ok := n.(*ast.HTMLBlock); ok && block.HasClosure() {
				// Types 1, 3, 4 and 5 keep their last line here, not in Lines.
				lines = append(lines, string(block.ClosureLine.Value(source)))
			}
			for _, line := range lines {
				tags += strings.Count(line, "<")
			}
			if tags > maxPageTags {
				tooComplex = true
				return ast.WalkStop, nil
			}
			return ast.WalkContinue, nil
		}
		lines := blockLines(n, source)
		for _, line := range lines {
			tags += strings.Count(line, "<")
		}
		if tags > maxPageTags {
			tooComplex = true
			return ast.WalkStop, nil
		}
		if table := tableStart(lines); table >= 0 {
			// The table extension takes the line before the first delimiter
			// row as the header and pads every row to its width. Count the
			// widest line, so the estimate never comes out lower.
			widest := 0
			for _, line := range lines {
				widest = max(widest, strings.Count(line, "|")+1)
			}
			if cells += widest * len(lines); cells > maxTableCells {
				tooComplex = true
				return ast.WalkStop, nil
			}
			// Each row is its own inline container.
			for _, line := range lines {
				if !inlineContainerOK(line) {
					tooComplex = true
					return ast.WalkStop, nil
				}
			}
			return ast.WalkContinue, nil
		}
		if !inlineContainerOK(strings.Join(lines, "")) {
			tooComplex = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if tooComplex {
		return ErrTooComplex
	}
	return nil
}

// blockLines returns the text lines of a block without container prefixes.
func blockLines(n ast.Node, source []byte) []string {
	segments := n.Lines()
	lines := make([]string, segments.Len())
	for i := range lines {
		segment := segments.At(i)
		lines[i] = string(segment.Value(source))
	}
	return lines
}

// tableStart returns the index of the first table delimiter row that has a
// line before it, or -1.
func tableStart(lines []string) int {
	for i := 1; i < len(lines); i++ {
		if isTableDelimiterRow(lines[i]) {
			return i
		}
	}
	return -1
}

func inlineContainerOK(s string) bool {
	if len(s) > maxInlineBytes {
		return false
	}
	delimiters := 0
	for _, c := range []string{"[", "<", "*", "_", "~", "`"} {
		delimiters += strings.Count(s, c)
	}
	return delimiters <= maxInlineDelimiters
}

// isTableDelimiterRow reports whether line looks like "|---|:--:|".
func isTableDelimiterRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "-") && strings.Trim(line, "|-: \t") == ""
}

// nestingDepth counts the block markers (">", "-", "*", "+", "1." and "1)")
// and the two-space indentation steps in front of a line's text.
func nestingDepth(line string) int {
	depth := 0
	for {
		spaces := len(line) - len(strings.TrimLeft(line, " \t"))
		rest := line[spaces:]
		marker := 0
		switch {
		case strings.HasPrefix(rest, ">"):
			marker = 1
		case len(rest) >= 2 && strings.ContainsRune("-*+", rune(rest[0])) && (rest[1] == ' ' || rest[1] == '\t'):
			marker = 2
		default:
			digits := len(rest) - len(strings.TrimLeft(rest, "0123456789"))
			if digits > 0 && digits < len(rest)-1 && (rest[digits] == '.' || rest[digits] == ')') && rest[digits+1] == ' ' {
				marker = digits + 2
			}
		}
		if marker == 0 {
			return depth
		}
		depth += 1 + spaces/2
		line = rest[marker:]
	}
}

// cappedWriter buffers up to limit bytes and fails once a write would pass it.
type cappedWriter struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.exceeded || w.buf.Len()+len(p) > w.limit {
		w.exceeded = true
		return 0, ErrTooComplex
	}
	return w.buf.Write(p)
}
