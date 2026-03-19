package links

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type MarkdownRefactorEngine struct {
	parser goldmark.Markdown
}

type RewriteWarning struct {
	Message string
}

type RewriteReplacement struct {
	Start    int
	End      int
	NewValue string
}

type RewriteResult struct {
	Content      string
	Replacements []RewriteReplacement
	Warnings     []RewriteWarning
}

type rewriteCandidate struct {
	Destination string
}

type inlineLinkOccurrence struct {
	RawDestination string
	Start          int
	End            int
}

func NewMarkdownRefactorEngine() *MarkdownRefactorEngine {
	return &MarkdownRefactorEngine{
		parser: goldmark.New(),
	}
}

func (r RewriteResult) Count() int {
	return len(r.Replacements)
}

func RewriteMarkdownLinks(content string, currentPath string, rules []RewriteRule) (string, int) {
	result := NewMarkdownRefactorEngine().Rewrite(content, currentPath, rules)
	return result.Content, result.Count()
}

func (e *MarkdownRefactorEngine) RewriteRelativeLinksForPathChange(content string, oldCurrentPath string, newCurrentPath string, rules []RewriteRule) RewriteResult {
	if content == "" || oldCurrentPath == newCurrentPath {
		return RewriteResult{Content: content}
	}

	candidates, excludedRanges := e.collectCandidatesAndExcludedRanges(content)
	if len(candidates) == 0 {
		return RewriteResult{Content: content}
	}

	occurrences := scanInlineLinkOccurrences(content, excludedRanges)
	replacements, warnings := buildPathChangeRewritePlan(oldCurrentPath, newCurrentPath, rules, candidates, occurrences)
	if len(replacements) == 0 {
		return RewriteResult{
			Content:  content,
			Warnings: warnings,
		}
	}

	return RewriteResult{
		Content:      applyReplacements(content, replacements),
		Replacements: replacements,
		Warnings:     warnings,
	}
}

func (e *MarkdownRefactorEngine) Rewrite(content string, currentPath string, rules []RewriteRule) RewriteResult {
	if len(rules) == 0 || content == "" {
		return RewriteResult{Content: content}
	}

	candidates, excludedRanges := e.collectCandidatesAndExcludedRanges(content)
	if len(candidates) == 0 {
		return RewriteResult{Content: content}
	}

	occurrences := scanInlineLinkOccurrences(content, excludedRanges)
	replacements, warnings := buildRewritePlan(currentPath, rules, candidates, occurrences)
	if len(replacements) == 0 {
		return RewriteResult{
			Content:  content,
			Warnings: warnings,
		}
	}

	return RewriteResult{
		Content:      applyReplacements(content, replacements),
		Replacements: replacements,
		Warnings:     warnings,
	}
}

func (e *MarkdownRefactorEngine) collectCandidatesAndExcludedRanges(content string) ([]rewriteCandidate, []textRange) {
	reader := text.NewReader([]byte(content))
	doc := e.parser.Parser().Parse(reader)

	var candidates []rewriteCandidate
	var excluded []textRange

	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.Link:
			candidates = append(candidates, rewriteCandidate{
				Destination: string(n.Destination),
			})
		case *ast.CodeSpan:
			excluded = append(excluded, collectTextNodeRanges(n)...)
		case *ast.FencedCodeBlock:
			excluded = append(excluded, collectBlockRanges(n)...)
		case *ast.CodeBlock:
			excluded = append(excluded, collectBlockRanges(n)...)
		}

		return ast.WalkContinue, nil
	})

	return candidates, mergeRanges(excluded)
}

func collectTextNodeRanges(parent ast.Node) []textRange {
	var ranges []textRange
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		if textNode, ok := child.(*ast.Text); ok {
			ranges = append(ranges, textRange{
				Start: textNode.Segment.Start,
				Stop:  textNode.Segment.Stop,
			})
		}
	}
	return ranges
}

func collectBlockRanges(node interface{ Lines() *text.Segments }) []textRange {
	lines := node.Lines()
	if lines == nil {
		return nil
	}

	ranges := make([]textRange, 0, lines.Len())
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		ranges = append(ranges, textRange{
			Start: segment.Start,
			Stop:  segment.Stop,
		})
	}
	return ranges
}

type textRange struct {
	Start int
	Stop  int
}

func mergeRanges(ranges []textRange) []textRange {
	if len(ranges) < 2 {
		return ranges
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start == ranges[j].Start {
			return ranges[i].Stop < ranges[j].Stop
		}
		return ranges[i].Start < ranges[j].Start
	})

	merged := make([]textRange, 0, len(ranges))
	current := ranges[0]
	for i := 1; i < len(ranges); i++ {
		next := ranges[i]
		if next.Start <= current.Stop {
			if next.Stop > current.Stop {
				current.Stop = next.Stop
			}
			continue
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)
	return merged
}

func scanInlineLinkOccurrences(content string, excluded []textRange) []inlineLinkOccurrence {
	var occurrences []inlineLinkOccurrence
	for i := 0; i < len(content); i++ {
		if isExcludedOffset(i, excluded) {
			continue
		}
		if content[i] != '[' {
			continue
		}
		if i > 0 && content[i-1] == '!' {
			continue
		}

		labelEnd := findClosingBracket(content, i)
		if labelEnd == -1 {
			continue
		}

		j := labelEnd + 1
		for j < len(content) && (content[j] == ' ' || content[j] == '\n' || content[j] == '\t') {
			j++
		}
		if j >= len(content) || content[j] != '(' {
			continue
		}

		occurrence, ok := parseInlineLinkOccurrence(content, j+1)
		if !ok {
			continue
		}
		occurrences = append(occurrences, occurrence)
		i = occurrence.End
	}
	return occurrences
}

func parseInlineLinkOccurrence(content string, start int) (inlineLinkOccurrence, bool) {
	i := start
	for i < len(content) && (content[i] == ' ' || content[i] == '\n' || content[i] == '\t') {
		i++
	}
	if i >= len(content) {
		return inlineLinkOccurrence{}, false
	}

	destStart := i
	destEnd := -1

	if content[i] == '<' {
		i++
		destStart = i
		for i < len(content) {
			if content[i] == '>' {
				destEnd = i
				break
			}
			if content[i] == '\\' {
				i++
			}
			i++
		}
	} else {
		depth := 0
		for i < len(content) {
			switch content[i] {
			case '\\':
				i += 2
				continue
			case '(':
				depth++
			case ')':
				if depth == 0 {
					destEnd = i
					goto done
				}
				depth--
			case ' ', '\n', '\t':
				destEnd = i
				goto done
			}
			i++
		}
	}

done:
	if destEnd == -1 || destEnd <= destStart {
		return inlineLinkOccurrence{}, false
	}

	closingParen := findClosingParen(content, start)
	if closingParen == -1 {
		return inlineLinkOccurrence{}, false
	}

	return inlineLinkOccurrence{
		RawDestination: content[destStart:destEnd],
		Start:          destStart,
		End:            destEnd,
	}, true
}

func buildRewritePlan(currentPath string, rules []RewriteRule, candidates []rewriteCandidate, occurrences []inlineLinkOccurrence) ([]RewriteReplacement, []RewriteWarning) {
	var replacements []RewriteReplacement
	var warnings []RewriteWarning

	occurrenceIndex := 0
	for _, candidate := range candidates {
		mapped := false
		for occurrenceIndex < len(occurrences) {
			occurrence := occurrences[occurrenceIndex]
			occurrenceIndex++

			if normalizeCandidateDestination(occurrence.RawDestination) != normalizeCandidateDestination(candidate.Destination) {
				continue
			}

			newDest, changed, warning := rewriteLinkDestination(currentPath, occurrence.RawDestination, rules)
			if warning != nil {
				warnings = append(warnings, *warning)
			}
			if changed {
				replacements = append(replacements, RewriteReplacement{
					Start:    occurrence.Start,
					End:      occurrence.End,
					NewValue: newDest,
				})
			}
			mapped = true
			break
		}

		if !mapped {
			warnings = append(warnings, RewriteWarning{
				Message: fmt.Sprintf("Skipped unsupported link syntax for destination %q", candidate.Destination),
			})
		}
	}

	return replacements, dedupeWarnings(warnings)
}

func buildPathChangeRewritePlan(oldCurrentPath string, newCurrentPath string, rules []RewriteRule, candidates []rewriteCandidate, occurrences []inlineLinkOccurrence) ([]RewriteReplacement, []RewriteWarning) {
	var replacements []RewriteReplacement
	var warnings []RewriteWarning

	occurrenceIndex := 0
	for _, candidate := range candidates {
		mapped := false
		for occurrenceIndex < len(occurrences) {
			occurrence := occurrences[occurrenceIndex]
			occurrenceIndex++

			if normalizeCandidateDestination(occurrence.RawDestination) != normalizeCandidateDestination(candidate.Destination) {
				continue
			}

			newDest, changed, warning := rewriteRelativeLinkForPathChange(oldCurrentPath, newCurrentPath, occurrence.RawDestination, rules)
			if warning != nil {
				warnings = append(warnings, *warning)
			}
			if changed {
				replacements = append(replacements, RewriteReplacement{
					Start:    occurrence.Start,
					End:      occurrence.End,
					NewValue: newDest,
				})
			}
			mapped = true
			break
		}

		if !mapped {
			warnings = append(warnings, RewriteWarning{
				Message: fmt.Sprintf("Skipped unsupported link syntax for destination %q", candidate.Destination),
			})
		}
	}

	return replacements, dedupeWarnings(warnings)
}

func normalizeCandidateDestination(destination string) string {
	if destination == "" {
		return ""
	}
	destination = strings.TrimSpace(destination)
	destination = strings.TrimPrefix(destination, "<")
	destination = strings.TrimSuffix(destination, ">")
	return destination
}

func rewriteLinkDestination(currentPath string, destination string, rules []RewriteRule) (string, bool, *RewriteWarning) {
	baseDest, suffix := splitLinkDestination(destination)
	if baseDest == "" || isExternalLinkDestination(baseDest) || isAssetLinkDestination(baseDest) {
		return destination, false, nil
	}

	resolvedPath, err := resolveURLPath(currentPath, baseDest)
	if err != nil || resolvedPath == "" {
		return destination, false, &RewriteWarning{
			Message: fmt.Sprintf("Skipped unresolved link destination %q", destination),
		}
	}

	newResolvedPath, ok := applyRewriteRules(resolvedPath, rules)
	if !ok || newResolvedPath == resolvedPath {
		return destination, false, nil
	}

	var rewrittenBase string
	if strings.HasPrefix(baseDest, "/") {
		rewrittenBase = newResolvedPath
	} else {
		currentPathForRelative := currentPath
		if nextCurrentPath, rewrittenCurrentPath := applyRewriteRules(normalizeWikiPath(currentPath), rules); rewrittenCurrentPath {
			currentPathForRelative = nextCurrentPath
		}
		rewrittenBase = relativeWikiLinkPath(currentPathForRelative, newResolvedPath)
	}

	if rewrittenBase == "" {
		return destination, false, &RewriteWarning{
			Message: fmt.Sprintf("Skipped empty rewritten destination for %q", destination),
		}
	}

	return rewrittenBase + suffix, true, nil
}

func rewriteRelativeLinkForPathChange(oldCurrentPath string, newCurrentPath string, destination string, rules []RewriteRule) (string, bool, *RewriteWarning) {
	baseDest, suffix := splitLinkDestination(destination)
	if baseDest == "" || strings.HasPrefix(baseDest, "/") || isExternalLinkDestination(baseDest) || isAssetLinkDestination(baseDest) {
		return destination, false, nil
	}

	resolvedPath, err := resolveURLPath(oldCurrentPath, baseDest)
	if err != nil || resolvedPath == "" {
		return destination, false, &RewriteWarning{
			Message: fmt.Sprintf("Skipped unresolved link destination %q", destination),
		}
	}

	targetPath := resolvedPath
	if rewrittenTarget, ok := applyRewriteRules(resolvedPath, rules); ok {
		targetPath = rewrittenTarget
	}

	rewrittenBase := relativeWikiLinkPath(newCurrentPath, targetPath)
	if rewrittenBase == "" {
		return destination, false, &RewriteWarning{
			Message: fmt.Sprintf("Skipped empty rewritten destination for %q", destination),
		}
	}

	if rewrittenBase == baseDest {
		return destination, false, nil
	}

	return rewrittenBase + suffix, true, nil
}

func applyReplacements(content string, replacements []RewriteReplacement) string {
	if len(replacements) == 0 {
		return content
	}

	var builder strings.Builder
	last := 0
	for _, replacement := range replacements {
		builder.WriteString(content[last:replacement.Start])
		builder.WriteString(replacement.NewValue)
		last = replacement.End
	}
	builder.WriteString(content[last:])
	return builder.String()
}

func dedupeWarnings(warnings []RewriteWarning) []RewriteWarning {
	if len(warnings) < 2 {
		return warnings
	}

	seen := make(map[string]struct{}, len(warnings))
	deduped := make([]RewriteWarning, 0, len(warnings))
	for _, warning := range warnings {
		if _, ok := seen[warning.Message]; ok {
			continue
		}
		seen[warning.Message] = struct{}{}
		deduped = append(deduped, warning)
	}
	return deduped
}

func isExcludedOffset(offset int, ranges []textRange) bool {
	for _, current := range ranges {
		if offset >= current.Start && offset < current.Stop {
			return true
		}
		if offset < current.Start {
			return false
		}
	}
	return false
}

func splitLinkDestination(destination string) (string, string) {
	if destination == "" {
		return "", ""
	}
	if idx := strings.IndexAny(destination, "?#"); idx != -1 {
		return destination[:idx], destination[idx:]
	}
	return destination, ""
}

func isExternalLinkDestination(destination string) bool {
	lower := strings.ToLower(destination)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "#")
}

func applyRewriteRules(resolvedPath string, rules []RewriteRule) (string, bool) {
	for _, rule := range rules {
		if resolvedPath == rule.OldPath {
			return rule.NewPath, true
		}
		if strings.HasPrefix(resolvedPath, rule.OldPath+"/") {
			return rule.NewPath + strings.TrimPrefix(resolvedPath, rule.OldPath), true
		}
	}
	return "", false
}

func relativeWikiLinkPath(currentPath string, targetPath string) string {
	base := normalizeWikiPath(currentPath)
	target := normalizeWikiPath(targetPath)
	if target == "" {
		return ""
	}

	baseParts := splitWikiPathSegments(base)
	targetParts := splitWikiPathSegments(target)

	common := 0
	for common < len(baseParts) && common < len(targetParts) && baseParts[common] == targetParts[common] {
		common++
	}

	var relParts []string
	for i := common; i < len(baseParts); i++ {
		relParts = append(relParts, "..")
	}
	relParts = append(relParts, targetParts[common:]...)

	rel := strings.Join(relParts, "/")
	if rel == "" {
		return ""
	}
	return rel
}

func splitWikiPathSegments(value string) []string {
	normalized := strings.Trim(normalizeWikiPath(value), "/")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "/")
}

func findClosingBracket(content string, start int) int {
	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		case '\\':
			i++
		}
	}
	return -1
}

func findClosingParen(content string, start int) int {
	depth := 1
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '\\':
			i++
		}
	}
	return -1
}
