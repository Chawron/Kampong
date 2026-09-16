package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kampong/debate/internal/models"
	"golang.org/x/net/html"
)

// DuckDuckGoSearcher implements Searcher using DuckDuckGo HTML search (actual web results).
type DuckDuckGoSearcher struct {
	client *http.Client
}

// NewDuckDuckGoSearcher creates a DuckDuckGo-backed searcher.
func NewDuckDuckGoSearcher(timeout time.Duration) *DuckDuckGoSearcher {
	return &DuckDuckGoSearcher{
		client: &http.Client{Timeout: timeout},
	}
}

// Search queries DuckDuckGo HTML search and parses actual web results.
func (d *DuckDuckGoSearcher) Search(ctx context.Context, query string) ([]models.SearchResult, error) {
	apiURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo status %d", resp.StatusCode)
	}

	return parseDDGHTML(resp.Body)
}

// parseDDGHTML extracts search results from DuckDuckGo HTML response.
func parseDDGHTML(r io.Reader) ([]models.SearchResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	var results []models.SearchResult

	// Walk the DOM looking for result links and snippets
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			// Check if this is a result link (class contains "result__a" or "result__url")
			isResultLink := false
			for _, attr := range n.Attr {
				if attr.Key == "class" && (strings.Contains(attr.Val, "result__a") || strings.Contains(attr.Val, "result__url")) {
					isResultLink = true
					break
				}
			}
			if isResultLink && len(results) < 8 {
				title := textContent(n)
				href := getAttr(n, "href")

				// DuckDuckGo wraps URLs in a redirect: /l/?kh=-1&uddg=ENCODED_URL
				if strings.HasPrefix(href, "/l/?") || strings.Contains(href, "uddg=") {
					if u, err := url.Parse(href); err == nil {
						if redir := u.Query().Get("uddg"); redir != "" {
							href = redir
						}
					}
				}

				if title != "" && href != "" && !strings.HasPrefix(href, "/") {
					// Look for the snippet in the parent's siblings
					snippet := findSnippet(n)
					results = append(results, models.SearchResult{
						Title:   cleanHTML(title),
						URL:     href,
						Snippet: cleanHTML(snippet),
						Score:   0.9 - float64(len(results))*0.05,
					})
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return results, nil
}

// textContent extracts all text from a node and its children.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

// getAttr returns the value of an HTML attribute.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// findSnippet looks for the snippet text near a result link.
func findSnippet(linkNode *html.Node) string {
	// Navigate up to the result container, then look for snippet class
	current := linkNode.Parent
	for i := 0; i < 5 && current != nil; i++ {
		snippet := findSnippetInSubtree(current)
		if snippet != "" {
			return snippet
		}
		current = current.Parent
	}
	return ""
}

// findSnippetInSubtree searches for a node with class containing "result__snippet".
func findSnippetInSubtree(n *html.Node) string {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, "result__snippet") {
				return textContent(n)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findSnippetInSubtree(c); result != "" {
			return result
		}
	}
	return ""
}

func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "<b>", "")
	s = strings.ReplaceAll(s, "</b>", "")
	s = strings.ReplaceAll(s, "<i>", "")
	s = strings.ReplaceAll(s, "</i>", "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.TrimSpace(s)
	return s
}
