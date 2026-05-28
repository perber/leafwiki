package tags

import (
	"context"
	"strings"

	"github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
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

func NewGetPagesByTagsUseCase(svc *coretags.TagsService, treeService *tree.TreeService, userResolver *auth.UserResolver) *GetPagesByTagsUseCase {
	return &GetPagesByTagsUseCase{svc: svc, treeService: treeService, userResolver: userResolver}
}

func ValidatePagesByTagsInput(tags []string) ([]string, error) {
	normalized := normalizeTags(tags)
	if len(normalized) == 0 {
		return nil, sharederrors.NewLocalizedError(
			ErrCodeTagsMissingParam,
			"Query parameter 'tags' is required",
			"query parameter tags is required",
			nil,
		)
	}
	return normalized, nil
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

	excerptsPerPage, err := uc.svc.GetExcerptsForPages(pageIDs)
	if err != nil {
		return nil, err
	}

	pages := make([]*dto.TaggedPage, 0, len(pageIDs))
	for _, id := range pageIDs {
		node, err := uc.treeService.FindPageByID(id)
		if err != nil || node == nil {
			continue
		}
		pages = append(pages, dto.ToTaggedPage(node, tagsPerPage[id], excerptsPerPage[id], uc.userResolver))
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
