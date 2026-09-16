package medical

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PubMedSearcher provides access to PubMed/MEDLINE literature.
type PubMedSearcher struct {
	client  *http.Client
	baseURL string
	apiKey  string // Optional NCBI API key for higher rate limits
}

// PubMedArticle represents a PubMed article.
type PubMedArticle struct {
	PMID        string   `json:"pmid"`
	Title       string   `json:"title"`
	Abstract    string   `json:"abstract"`
	Authors     []string `json:"authors"`
	Journal     string   `json:"journal"`
	PubDate     string   `json:"pub_date"`
	DOI         string   `json:"doi"`
	MeshTerms   []string `json:"mesh_terms"`
	ArticleType string   `json:"article_type"`
	EvidenceLevel int    `json:"evidence_level"` // 1-5 Oxford level
}

// PubMedSearchResult contains the search results.
type PubMedSearchResult struct {
	Query      string           `json:"query"`
	TotalCount int              `json:"total_count"`
	Articles   []PubMedArticle  `json:"articles"`
}

// NewPubMedSearcher creates a new PubMed searcher.
func NewPubMedSearcher(apiKey string) *PubMedSearcher {
	return &PubMedSearcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils",
		apiKey:  apiKey,
	}
}

// Search searches PubMed for medical literature.
func (p *PubMedSearcher) Search(ctx context.Context, query string, maxResults int) (*PubMedSearchResult, error) {
	// Step 1: Search for PMIDs
	searchURL := fmt.Sprintf("%s/esearch.fcgi?db=pubmed&term=%s&retmax=%d&retmode=json",
		p.baseURL, url.QueryEscape(query), maxResults)
	
	if p.apiKey != "" {
		searchURL += "&api_key=" + p.apiKey
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Parse search results
	var searchResp struct {
		ESearchResult struct {
			Count     string   `json:"count"`
			IDList    []string `json:"idlist"`
		} `json:"esearchresult"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}

	if len(searchResp.ESearchResult.IDList) == 0 {
		return &PubMedSearchResult{
			Query:      query,
			TotalCount: 0,
			Articles:   []PubMedArticle{},
		}, nil
	}

	// Step 2: Fetch article details
	articles, err := p.fetchArticles(ctx, searchResp.ESearchResult.IDList)
	if err != nil {
		return nil, fmt.Errorf("fetch articles: %w", err)
	}

	totalCount := 0
	fmt.Sscanf(searchResp.ESearchResult.Count, "%d", &totalCount)

	return &PubMedSearchResult{
		Query:      query,
		TotalCount: totalCount,
		Articles:   articles,
	}, nil
}

// fetchArticles fetches detailed information for a list of PMIDs.
func (p *PubMedSearcher) fetchArticles(ctx context.Context, pmids []string) ([]PubMedArticle, error) {
	ids := strings.Join(pmids, ",")
	fetchURL := fmt.Sprintf("%s/efetch.fcgi?db=pubmed&id=%s&retmode=xml", p.baseURL, ids)

	if p.apiKey != "" {
		fetchURL += "&api_key=" + p.apiKey
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create fetch request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read fetch response: %w", err)
	}

	// Parse XML response (simplified - in production use proper XML parsing)
	articles := p.parsePubMedXML(string(body))

	return articles, nil
}

// parsePubMedXML parses PubMed XML response into articles.
// This is a simplified parser - in production, use encoding/xml properly.
func (p *PubMedSearcher) parsePubMedXML(xmlData string) []PubMedArticle {
	// Simplified parsing - extract key fields
	// In production, use proper XML parsing with encoding/xml
	
	articles := []PubMedArticle{}
	
	// Split by article markers
	parts := strings.Split(xmlData, "<PubmedArticle>")
	for i := 1; i < len(parts); i++ {
		article := PubMedArticle{}
		part := parts[i]
		
		// Extract PMID
		if pmid := extractXMLValue(part, "PMID"); pmid != "" {
			article.PMID = pmid
		}
		
		// Extract title
		if title := extractXMLValue(part, "ArticleTitle"); title != "" {
			article.Title = title
		}
		
		// Extract abstract
		if abstract := extractXMLValue(part, "AbstractText"); abstract != "" {
			article.Abstract = abstract
		}
		
		// Extract journal
		if journal := extractXMLValue(part, "Title"); journal != "" {
			article.Journal = journal
		}
		
		// Extract publication date
		if year := extractXMLValue(part, "Year"); year != "" {
			article.PubDate = year
		}
		
		// Extract DOI
		if doi := extractXMLValue(part, "ELocationID"); doi != "" {
			article.DOI = doi
		}
		
		// Determine evidence level based on article type
		article.EvidenceLevel = determineEvidenceLevelFromXML(part)
		
		articles = append(articles, article)
	}
	
	return articles
}

// extractXMLValue extracts a value from XML by tag name (simplified).
func extractXMLValue(xml, tagName string) string {
	startTag := "<" + tagName
	endTag := "</" + tagName + ">"
	
	startIdx := strings.Index(xml, startTag)
	if startIdx == -1 {
		return ""
	}
	
	// Find the end of the start tag
	tagEnd := strings.Index(xml[startIdx:], ">")
	if tagEnd == -1 {
		return ""
	}
	
	valueStart := startIdx + tagEnd + 1
	endIdx := strings.Index(xml[valueStart:], endTag)
	if endIdx == -1 {
		return ""
	}
	
	return strings.TrimSpace(xml[valueStart : valueStart+endIdx])
}

// determineEvidenceLevelFromXML determines evidence level from article type.
func determineEvidenceLevelFromXML(xml string) int {
	xmlLower := strings.ToLower(xml)
	
	// Level 1: Systematic reviews, meta-analyses
	if strings.Contains(xmlLower, "systematic review") || strings.Contains(xmlLower, "meta-analysis") {
		return 1
	}
	
	// Level 2: Randomized controlled trials
	if strings.Contains(xmlLower, "randomized controlled trial") || strings.Contains(xmlLower, "rct") {
		return 2
	}
	
	// Level 3: Cohort studies, case-control
	if strings.Contains(xmlLower, "cohort") || strings.Contains(xmlLower, "case-control") {
		return 3
	}
	
	// Level 4: Case series, case reports
	if strings.Contains(xmlLower, "case series") || strings.Contains(xmlLower, "case report") {
		return 4
	}
	
	// Level 5: Expert opinion, guidelines, reviews
	return 5
}

// SearchByClinicalQuestion searches PubMed using PICO format.
func (p *PubMedSearcher) SearchByClinicalQuestion(ctx context.Context, population, intervention, comparison, outcome string, maxResults int) (*PubMedSearchResult, error) {
	query := ""
	
	if population != "" {
		query += population + "[Title/Abstract]"
	}
	
	if intervention != "" {
		if query != "" {
			query += " AND "
		}
		query += intervention + "[Title/Abstract]"
	}
	
	if comparison != "" {
		if query != "" {
			query += " AND "
		}
		query += comparison + "[Title/Abstract]"
	}
	
	if outcome != "" {
		if query != "" {
			query += " AND "
		}
		query += outcome + "[Title/Abstract]"
	}
	
	// Filter for high-quality studies
	query += " AND (\"randomized controlled trial\"[Publication Type] OR \"systematic review\"[Publication Type] OR \"meta-analysis\"[Publication Type])"
	
	return p.Search(ctx, query, maxResults)
}
