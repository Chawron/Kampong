package search

import (
	"context"

	"github.com/kampong/debate/internal/models"
)

// Searcher is the interface for web search providers.
type Searcher interface {
	Search(ctx context.Context, query string) ([]models.SearchResult, error)
}
