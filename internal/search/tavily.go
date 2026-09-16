package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kampong/debate/internal/models"
)

// TavilySearcher implements Searcher using the Tavily Search API.
type TavilySearcher struct {
	apiKey string
	client *http.Client
}

// NewTavilySearcher creates a Tavily-backed searcher.
func NewTavilySearcher(apiKey string, timeout time.Duration) *TavilySearcher {
	return &TavilySearcher{
		apiKey: apiKey,
		client: &http.Client{Timeout: timeout},
	}
}

type tavilyRequest struct {
	Query             string `json:"query"`
	SearchDepth       string `json:"search_depth,omitempty"`
	MaxResults        int    `json:"max_results,omitempty"`
	IncludeAnswer     bool   `json:"include_answer,omitempty"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type tavilyResponse struct {
	Answer  string         `json:"answer,omitempty"`
	Results []tavilyResult `json:"results"`
}

// Search queries the Tavily Search API.
func (t *TavilySearcher) Search(ctx context.Context, query string) ([]models.SearchResult, error) {
	reqBody := tavilyRequest{
		Query:         query,
		SearchDepth:   "basic",
		MaxResults:    5,
		IncludeAnswer: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var tr tavilyResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	var results []models.SearchResult

	if tr.Answer != "" {
		results = append(results, models.SearchResult{
			Title:   "AI Answer",
			URL:     "",
			Snippet: tr.Answer,
			Score:   1.0,
		})
	}

	for _, r := range tr.Results {
		results = append(results, models.SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Score:   r.Score,
		})
	}

	return results, nil
}
