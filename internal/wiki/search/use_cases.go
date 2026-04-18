package search

import (
	"context"
	"fmt"

	coresearch "github.com/perber/wiki/internal/search"
)

// ─── SearchUseCase ───────────────────────────────────────────────────────────

type SearchInput struct {
	Query  string
	Offset int
	Limit  int
}

type SearchOutput struct {
	Result *coresearch.SearchResult
}

type SearchUseCase struct {
	index *coresearch.SQLiteIndex
}

func NewSearchUseCase(idx *coresearch.SQLiteIndex) *SearchUseCase {
	return &SearchUseCase{index: idx}
}

func (uc *SearchUseCase) Execute(_ context.Context, in SearchInput) (*SearchOutput, error) {
	if uc.index == nil {
		return nil, fmt.Errorf("search index not available")
	}
	result, err := uc.index.Search(in.Query, in.Offset, in.Limit)
	if err != nil {
		return nil, err
	}
	return &SearchOutput{Result: result}, nil
}

// ─── GetIndexingStatusUseCase ────────────────────────────────────────────────

type GetIndexingStatusOutput struct {
	Status *coresearch.IndexingStatus
}

type GetIndexingStatusUseCase struct {
	status *coresearch.IndexingStatus
}

func NewGetIndexingStatusUseCase(s *coresearch.IndexingStatus) *GetIndexingStatusUseCase {
	return &GetIndexingStatusUseCase{status: s}
}

func (uc *GetIndexingStatusUseCase) Execute(_ context.Context) *GetIndexingStatusOutput {
	if uc.status == nil {
		return &GetIndexingStatusOutput{Status: nil}
	}
	return &GetIndexingStatusOutput{Status: uc.status.Snapshot()}
}
