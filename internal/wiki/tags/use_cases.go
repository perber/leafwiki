package tags

import (
	"context"
	"regexp"
	"strings"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	coretags "github.com/perber/wiki/internal/tags"
)

// ─── GetTagsUseCase ──────────────────────────────────────────────────────────

type GetTagsInput struct {
	Filter   string
	Selected []string
	Limit    int
}

type GetTagsOutput struct {
	Tags []coretags.TagCount
}

type GetTagsUseCase struct {
	svc *coretags.TagsService
}

func NewGetTagsUseCase(svc *coretags.TagsService) *GetTagsUseCase {
	return &GetTagsUseCase{svc: svc}
}

func (uc *GetTagsUseCase) Execute(_ context.Context, in GetTagsInput) (*GetTagsOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	filter := strings.ToLower(strings.TrimSpace(in.Filter))
	selected := normalizeTags(in.Selected)

	var (
		tags []coretags.TagCount
		err  error
	)
	if len(selected) == 0 {
		tags, err = uc.svc.GetAllTags(filter, limit)
	} else {
		tags, err = uc.svc.GetAllTagsForSelection(filter, selected, limit)
	}
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []coretags.TagCount{}
	}
	return &GetTagsOutput{Tags: tags}, nil
}

// ─── GetPagesByTagsUseCase ───────────────────────────────────────────────────

type GetPagesByTagsInput struct {
	Tags []string
}

type GetPagesByTagsOutput struct {
	Pages []*dto.TaggedPage
}

type GetPagesByTagsUseCase struct {
	svc          *coretags.TagsService
	treeService  *tree.TreeService
	userResolver *auth.UserResolver
}

const tagExcerptMaxRunes = 180

var (
	tagExcerptFencedCodePattern = regexp.MustCompile("(?s)```.*?```")
	tagExcerptImagePattern      = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	tagExcerptLinkPattern       = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	tagExcerptHTMLPattern       = regexp.MustCompile(`<[^>]+>`)
	tagExcerptLinePrefixPattern = regexp.MustCompile(`(?m)^\s{0,3}(#{1,6}\s*|[-*+]\s+|\d+\.\s+|>\s?)`)
)

func NewGetPagesByTagsUseCase(svc *coretags.TagsService, treeService *tree.TreeService, userResolver *auth.UserResolver) *GetPagesByTagsUseCase {
	return &GetPagesByTagsUseCase{svc: svc, treeService: treeService, userResolver: userResolver}
}

func (uc *GetPagesByTagsUseCase) Execute(_ context.Context, in GetPagesByTagsInput) (*GetPagesByTagsOutput, error) {
	normalized := normalizeTags(in.Tags)
	if len(normalized) == 0 {
		return &GetPagesByTagsOutput{Pages: []*dto.TaggedPage{}}, nil
	}

	pageIDs, err := uc.svc.GetPageIDsByTags(normalized)
	if err != nil {
		return nil, err
	}
	if len(pageIDs) == 0 {
		return &GetPagesByTagsOutput{Pages: []*dto.TaggedPage{}}, nil
	}

	tagsPerPage, err := uc.svc.GetTagsForPages(pageIDs)
	if err != nil {
		return nil, err
	}

	pages := make([]*dto.TaggedPage, 0, len(pageIDs))
	for _, id := range pageIDs {
		node, err := uc.treeService.FindPageByID(id)
		if err != nil || node == nil {
			continue
		}

		excerpt := ""
		raw, err := uc.treeService.ReadPageRaw(id)
		if err == nil {
			excerpt = buildTagExcerpt(raw)
		}

		pages = append(pages, dto.ToTaggedPage(node, tagsPerPage[id], excerpt, uc.userResolver))
	}

	return &GetPagesByTagsOutput{Pages: pages}, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		n := strings.ToLower(strings.TrimSpace(t))
		if n == "" {
			continue
		}
		if _, exists := seen[n]; exists {
			continue
		}
		seen[n] = struct{}{}
		result = append(result, n)
	}
	return result
}

func buildTagExcerpt(raw string) string {
	_, body, _, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		body = raw
	}

	text := strings.ReplaceAll(body, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = tagExcerptFencedCodePattern.ReplaceAllString(text, " ")
	text = tagExcerptImagePattern.ReplaceAllString(text, "$1")
	text = tagExcerptLinkPattern.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "`", "")
	text = tagExcerptHTMLPattern.ReplaceAllString(text, " ")
	text = tagExcerptLinePrefixPattern.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")

	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= tagExcerptMaxRunes {
		return text
	}

	truncated := strings.TrimSpace(string(runes[:tagExcerptMaxRunes]))
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace >= 120 {
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated) + "..."
}
