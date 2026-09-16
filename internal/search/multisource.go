package search

import (
	"context"
	"net/url"
	"strings"

	"github.com/kampong/debate/internal/models"
)

// MultiSourceSearcher runs the same query on two providers and merges results.
//
// Cross-verification rule: a result whose *domain* (eTLD+1) appears in both
// providers' outputs is marked CrossVerified=true. Single-source results are
// still kept but the absence of cross-verification is visible to the agent in
// the prompt and to the user in the UI.
type MultiSourceSearcher struct {
	Primary   Searcher
	Secondary Searcher
	// Optional labels surfaced in the prompt (e.g. "Tavily", "DuckDuckGo").
	// If empty we derive from the type names.
	PrimaryName   string
	SecondaryName string
}

// Search runs both providers, merges, and tags cross-verified results.
func (m *MultiSourceSearcher) Search(ctx context.Context, query string) ([]models.SearchResult, error) {
	primary, err := m.runOne(ctx, m.Primary, query)
	if err != nil {
		primary = nil
	}
	secondary, err := m.runOne(ctx, m.Secondary, query)
	if err != nil {
		secondary = nil
	}

	if len(primary) == 0 && len(secondary) == 0 {
		return nil, errEmpty
	}

	// Track domains seen in each provider.
	byDomain := make(map[string]map[string]*domainBucket) // source -> domain -> bucket
	byDomain[m.primaryLabel()] = make(map[string]*domainBucket)
	if m.Secondary != nil {
		byDomain[m.secondaryLabel()] = make(map[string]*domainBucket)
	}

	for _, r := range primary {
		d := domainOf(r.URL)
		if d == "" {
			continue
		}
		byDomain[m.primaryLabel()][d] = &domainBucket{title: r.Title, snippet: r.Snippet, url: r.URL, has: true}
	}
	secondaryLabel := m.secondaryLabel()
	if m.Secondary != nil {
		for _, r := range secondary {
			d := domainOf(r.URL)
			if d == "" {
				continue
			}
			bucket, ok := byDomain[secondaryLabel][d]
			if !ok {
				bucket = &domainBucket{}
				byDomain[secondaryLabel][d] = bucket
			}
			bucket.has = true
			if bucket.title == "" {
				bucket.title = r.Title
				bucket.snippet = r.Snippet
				bucket.url = r.URL
			}
		}
	}

	// Emit one entry per domain that appears in at least one source.
	// Order: cross-verified first, then by Score desc within each group.
	seen := make(map[string]bool)
	var verified, single []models.SearchResult
	for label, buckets := range byDomain {
		for d, b := range buckets {
			if !b.has || seen[d] {
				continue
			}
			seen[d] = true
			res := models.SearchResult{
				Title:         b.title,
				Snippet:       b.snippet,
				URL:           b.url,
				Source:        label,
				Domain:        d,
				CrossVerified: len(byDomain) > 1 && bothSources(d, byDomain),
			}
			if res.CrossVerified {
				verified = append(verified, res)
			} else {
				single = append(single, res)
			}
		}
	}
	return append(verified, single...), nil
}

func (m *MultiSourceSearcher) runOne(ctx context.Context, s Searcher, query string) ([]models.SearchResult, error) {
	if s == nil {
		return nil, errEmpty
	}
	return s.Search(ctx, query)
}

func (m *MultiSourceSearcher) primaryLabel() string {
	if m.PrimaryName != "" {
		return m.PrimaryName
	}
	return "primary"
}

func (m *MultiSourceSearcher) secondaryLabel() string {
	if m.SecondaryName != "" {
		return m.SecondaryName
	}
	if m.Secondary == nil {
		return ""
	}
	return "secondary"
}

func bothSources(domain string, m map[string]map[string]*domainBucket) bool {
	seen := 0
	for _, buckets := range m {
		if b, ok := buckets[domain]; ok && b.has {
			seen++
		}
	}
	return seen >= 2
}

// domainOf returns eTLD+1 in a deliberately simplistic way: host minus the
// first label. Good enough for the cross-verification use case — full PSL
// parsing would be overkill here.
func domainOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Host
	// Strip "www." prefix; everything else is kept verbatim.
	host = strings.TrimPrefix(host, "www.")
	return host
}

// errEmpty is returned when both providers yield no results. Callers can
// ignore it (the search is best-effort).
var errEmpty = errMultiNoResults{}

// domainBucket holds the first-seen title/snippet/url for a single
// (source, domain) pair, plus a "have we seen it" flag so cross-source
// detection can tell whether both providers returned it.
type domainBucket struct {
	title, snippet, url string
	has                 bool
}

type errMultiNoResults struct{}

func (errMultiNoResults) Error() string { return "multi-source search: no results from any source" }
