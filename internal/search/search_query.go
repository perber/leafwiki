package search

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// minSubstringTermRunes is the shortest literal word that also counts as a
// substring term: a trigram index cannot match anything shorter.
const minSubstringTermRunes = 3

// searchQuery is raw user search input parsed into an FTS5 MATCH argument
// plus the literal words it was built from.
type searchQuery struct {
	// fts is the FTS5 MATCH argument; "" when there is nothing to match.
	fts string
	// literalOnly is true when every field went through the literal path:
	// no trusted operator, keyword, quoted phrase, column filter or NEAR
	// call. An empty query is not literalOnly.
	literalOnly bool
	// substringTerms are the literal words of at least
	// minSubstringTermRunes runes, in query order.
	substringTerms []string
	// shortTerms are the literal words shorter than that, in query order.
	shortTerms []string
}

// buildFuzzyQuery turns raw user search input into an FTS5 MATCH argument.
func buildFuzzyQuery(q string) string {
	return parseSearchQuery(q).fts
}

// parseSearchQuery turns raw user search input into a searchQuery.
//
// It classifies each whitespace-separated field independently rather than
// making one all-or-nothing trust decision for the whole query string. The
// earlier whole-query approach (github.com/perber/leafwiki/issues/1577)
// trusted the *entire* query as deliberate FTS5 syntax the moment it saw an
// operator character (*, (, ), :, AND, OR) anywhere in it — so a query like
// `foo* bar,baz` or `it's* thing` still reached SQLite with an unescaped
// comma or apostrophe in the other field and failed with an FTS5 syntax
// error. Classifying field-by-field means one field's operator syntax can
// no longer vouch for a different field's stray punctuation.
func parseSearchQuery(q string) searchQuery {
	q = strings.TrimSpace(q)
	if q == "" {
		return searchQuery{}
	}

	// A NEAR(...) proximity call is the one FTS5 construct whose syntax
	// genuinely spans multiple whitespace-separated fields tied together
	// by punctuation (NEAR(term1 term2, distance)) — per-field
	// classification below would treat "term2," and "distance)" as two
	// unrelated fields and strip the comma the call's own grammar requires,
	// silently breaking it instead of erroring. Trusting the whole call
	// verbatim when it's shaped like one preserves this narrow,
	// already-supported case exactly as it worked before this function
	// existed.
	if isNearFunctionCall(q) {
		return searchQuery{fts: q}
	}

	parsed := searchQuery{literalOnly: true}
	fields := splitFTS5AwareFields(q)
	sanitized := make([]string, len(fields))
	for i := range fields {
		if trusted, ok := trustedFTS5Field(fields, i); ok {
			sanitized[i] = trusted
			parsed.literalOnly = false
			continue
		}

		// Literal text: split it into the same "words" the pages table's
		// tokenizer would (see splitIntoIndexWords) and AND together a
		// quoted-prefix match for each one, rather than quoting the whole
		// field as a single phrase. A single quoted phrase would need FTS5 to
		// treat the words on either side of the dropped punctuation as
		// consecutive token positions, which does not hold for every
		// separator character — SQLite's unicode61 tokenizer does not leave
		// "it" and "s" (from "it's") at adjacent positions the way it does
		// for, say, a comma, so a `"it's"` phrase query fails to match
		// content containing "it's" even though both tokenize the same way.
		// AND-ing independent terms doesn't depend on that adjacency at all.
		words := splitIntoIndexWords(fields[i])
		for _, w := range words {
			if utf8.RuneCountInString(w) >= minSubstringTermRunes {
				parsed.substringTerms = append(parsed.substringTerms, w)
			} else {
				parsed.shortTerms = append(parsed.shortTerms, w)
			}
		}
		sanitized[i] = quoteWordsAsPrefixTerms(words)
	}
	parsed.fts = strings.Join(sanitized, " ")
	return parsed
}

// isNearFunctionCall reports whether q, trimmed, is shaped like a complete
// FTS5 NEAR(...) proximity call ("NEAR" then "(", ending in ")") — not a
// syntax guarantee, just enough to single out this specific construct from
// ordinary text before per-field classification would otherwise take it
// apart.
func isNearFunctionCall(q string) bool {
	upper := strings.ToUpper(q)
	if !strings.HasPrefix(upper, "NEAR") {
		return false
	}
	rest := strings.TrimSpace(upper[len("NEAR"):])
	return strings.HasPrefix(rest, "(") && strings.HasSuffix(q, ")")
}

// splitFTS5AwareFields splits q on whitespace like strings.Fields, except
// whitespace inside a "double-quoted span" doesn't split the field, so a
// quoted phrase (balanced or not) reaches trustedFTS5Field as one field to
// classify as a whole instead of being cut apart mid-phrase.
func splitFTS5AwareFields(q string) []string {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for _, r := range q {
		switch {
		case r == '"':
			cur.WriteRune(r)
			inQuote = !inQuote
		case !inQuote && (r == ' ' || r == '\t' || r == '\n' || r == '\r'):
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}

// trustedFTS5Field decides whether fields[i] is trustworthy, deliberate
// FTS5 syntax (an operator, a boolean keyword used between two real
// operands, a column filter naming a real column, an already-quoted
// phrase) and returns it in the form to send to FTS5. ok is false for
// literal search text that needs quoting so it can't be misread as syntax.
//
// A comma or apostrophe — never valid FTS5 syntax on their own — and an
// unbalanced double quote always disqualify a field from the "trust it"
// path, even if the same field also contains an operator character like *
// or : (that combination is exactly what #1577's follow-up reports found:
// "sometimes # and \"" alongside other punctuation).
func trustedFTS5Field(fields []string, i int) (string, bool) {
	f := fields[i]
	if isBalancedQuotedField(f) {
		return f, true
	}

	// A column filter is always rewritten (never trusted verbatim) into a
	// safe, per-word-quoted form scoped to that column — see
	// rewriteColumnFilterField for why: the raw value can contain anything
	// (a pasted URL's second ":", an apostrophe, ...) without needing its
	// own escaping rules, because it's built from the same safe
	// splitIntoIndexWords primitive as the plain-literal-text path.
	if rewritten, ok := rewriteColumnFilterField(f); ok {
		return rewritten, true
	}

	// An operator field ("foo*", "(foo", "bar)") is only trusted when it is
	// well-formed FTS5 syntax on its own and the query's parentheses balance
	// overall. Anything else ("foo-bar*", "report.v2*", "(foo" without a
	// partner, a bare "*") is literal text.
	if strings.ContainsAny(f, "*()") {
		if isWellFormedOperatorField(f) && (!strings.ContainsAny(f, "()") || operatorParensBalanced(fields)) {
			return f, true
		}
	} else if isFTS5Keyword(f) && hasRealOperandNeighbors(fields, i) {
		return f, true
	}
	return "", false
}

// isBarewordRune reports whether r may appear in an unquoted FTS5 bareword.
// The tokenizer's searchTokenChars (- . / # +) are deliberately not
// included: they are word characters for indexing but not valid query
// syntax.
func isBarewordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isWellFormedOperatorField reports whether f consists only of barewords,
// prefix markers and parentheses in a shape FTS5 accepts: "(" only at the
// start or after another "(", "*" only directly after a word, ")" only after
// a word, "*" or ")", and nothing following ")" except another ")".
func isWellFormedOperatorField(f string) bool {
	var prev rune // 0 = start of field
	for _, r := range f {
		switch {
		case r == '*':
			if !isBarewordRune(prev) {
				return false
			}
		case r == '(':
			if prev != 0 && prev != '(' {
				return false
			}
		case r == ')':
			if !isBarewordRune(prev) && prev != '*' && prev != ')' {
				return false
			}
		case isBarewordRune(r):
			if prev == '*' || prev == ')' {
				return false
			}
		default:
			return false
		}
		prev = r
	}
	return true
}

// operatorParensBalanced reports whether the parentheses in the query's
// well-formed operator fields pair up, so a group may span several fields
// ("(apple OR cherry) AND banana") but a lone "(foo" is not trusted.
func operatorParensBalanced(fields []string) bool {
	depth := 0
	for _, f := range fields {
		if !isWellFormedOperatorField(f) {
			continue
		}
		for _, r := range f {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}
	return depth == 0
}

// quoteWordsAsPrefixTerms wraps each word as its own quoted-phrase-prefix
// match ("word"*) and joins them with a space (FTS5's implicit AND). Since
// each word came from splitIntoIndexWords, none of them can contain a
// quote, comma, or apostrophe, so no escaping is needed here.
func quoteWordsAsPrefixTerms(words []string) string {
	for i, w := range words {
		words[i] = `"` + w + `"*`
	}
	return strings.Join(words, " ")
}

// hasRealOperandNeighbors reports whether fields[i] sits strictly between
// two other fields that aren't themselves boolean keywords. FTS5's
// AND/OR/NOT are binary operators: a dangling "AND"/"OR"/"NOT" with no
// operand on one side (or with another keyword standing in for the
// operand, e.g. a doubled "foo AND AND bar") is itself a syntax error, not
// valid-but-unusual syntax.
func hasRealOperandNeighbors(fields []string, i int) bool {
	if i <= 0 || i >= len(fields)-1 {
		return false
	}
	return !isFTS5Keyword(fields[i-1]) && !isFTS5Keyword(fields[i+1])
}

// searchTokenChars is the extra tokenchars given to the pages table's
// unicode61 tokenizer (see ensureSchema) — the punctuation that's part of a
// word rather than a separator (so "report-final.v2" indexes as one token,
// not three). splitIntoIndexWords uses the same alphabet so a literal
// search field is split into exactly the "words" the tokenizer would
// itself produce; both must stay in sync with the schema's tokenize
// argument or search can silently miss content that's plainly there.
const searchTokenChars = "-_/+#."

// splitIntoIndexWords splits a raw field into the same run-of-word-chars
// units the pages table's tokenizer (see searchTokenChars) would treat as
// tokens, dropping any other character (comma, apostrophe, a stray ") as a
// separator instead of embedding it as literal phrase content — since
// split words can never contain a quote, comma, or apostrophe, none of
// them need escaping.
func splitIntoIndexWords(f string) []string {
	var words []string
	var cur strings.Builder
	for _, r := range f {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(searchTokenChars, r) {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

// isFTS5Keyword reports whether f is a boolean-operator keyword. Callers
// must also confirm it sits between two real operands (see
// hasRealOperandNeighbors) before trusting it — FTS5's AND/OR/NOT are binary
// operators, so a dangling "AND"/"OR"/"NOT" with no left or right neighbor
// is itself a syntax error, not valid-but-unusual syntax.
func isFTS5Keyword(f string) bool {
	switch strings.ToUpper(f) {
	case "AND", "OR", "NOT":
		return true
	default:
		return false
	}
}

// searchColumns are the pages FTS5 table's real column names, lowercased,
// against which a field's "col:" prefix is validated before trusting it as
// an FTS5 column filter (see rewriteColumnFilterField). FTS5 column-name
// matching is case-insensitive, so column names are compared lowercased.
var searchColumns = map[string]bool{
	"path":     true,
	"filepath": true,
	"pageid":   true,
	"kind":     true,
	"title":    true,
	"headings": true,
	"content":  true,
}

// rewriteColumnFilterField checks whether f looks like a deliberate FTS5
// column filter (e.g. "content:search") naming one of the pages table's
// real columns, as opposed to incidental punctuation with a colon in it —
// a pasted URL ("http://example.com"), a Windows path ("c:\Users\foo"), or
// a time ("10:30") all contain ":" but aren't column filters, and trusting
// any colon-bearing field unconditionally (the pre-#1577 behavior) sends
// SQLite a "no such column" error for all three.
//
// When the prefix does name a real column, the *value* after the colon is
// never trusted verbatim either — it's rewritten into a parenthesized,
// per-word quoted-prefix group scoped to that column
// (`col:("word1"* "word2"*)`), built with the same splitIntoIndexWords
// primitive as the plain-literal-text path. That handles a value
// containing anything (another ":" from a pasted URL, an apostrophe, ...)
// safely without a separate "is this value simple enough to trust"
// predicate, and — unlike simply falling back to an unscoped literal
// search — still honors the column scope the user asked for instead of
// silently searching every column.
func rewriteColumnFilterField(f string) (string, bool) {
	idx := strings.Index(f, ":")
	if idx <= 0 {
		return "", false
	}
	col := f[:idx]
	if !searchColumns[strings.ToLower(col)] {
		return "", false
	}
	words := splitIntoIndexWords(f[idx+1:])
	if len(words) == 0 {
		return "", false
	}
	return col + ":(" + quoteWordsAsPrefixTerms(words) + ")", true
}

// isBalancedQuotedField reports whether f is a complete, balanced quoted
// FTS5 phrase — e.g. `"foo bar"` or `"foo bar"*` — as opposed to a
// lone/unbalanced " that only looks like the start of one.
func isBalancedQuotedField(f string) bool {
	body := strings.TrimSuffix(f, "*")
	if len(body) < 2 || !strings.HasPrefix(body, `"`) || !strings.HasSuffix(body, `"`) {
		return false
	}
	inner := body[1 : len(body)-1]
	return strings.Count(inner, `"`)%2 == 0
}

func buildSearchWhereClause(ftsQuery string, pageIDs []string) (string, []interface{}) {
	clauses := make([]string, 0, 2)
	args := make([]interface{}, 0, 1+len(pageIDs))

	if ftsQuery != "" {
		clauses = append(clauses, "pages MATCH ?")
		args = append(args, ftsQuery)
	}

	if len(pageIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(pageIDs)), ",")
		clauses = append(clauses, fmt.Sprintf("pageID IN (%s)", placeholders))
		for _, pageID := range pageIDs {
			args = append(args, pageID)
		}
	}

	return strings.Join(clauses, " AND "), args
}

func searchTitleExpr(hasQuery bool) string {
	if hasQuery {
		return "highlight(pages, 4, char(2), char(3))"
	}
	return "title"
}

func searchExcerptExpr(hasQuery bool) string {
	if hasQuery {
		return "snippet(pages, 6, '<b>', '</b>', '...', 16)"
	}
	return "''"
}

func searchRankExpr(hasQuery bool) string {
	if hasQuery {
		return `bm25(pages,
				0.0,  -- path
				0.0,  -- filepath
				0.0,  -- pageID
				0.0,  -- kind
				20.0, -- title
				5.0,   -- headings
				1.0    -- content
			)`
	}
	return "0.0"
}

func searchOrderByExpr(hasQuery bool) string {
	if hasQuery {
		return "bm25_score ASC"
	}
	return "title COLLATE NOCASE ASC, path COLLATE NOCASE ASC"
}
