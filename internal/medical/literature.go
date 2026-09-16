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

// LiteratureSearcher provides real-time medical literature search capabilities.
type LiteratureSearcher struct {
	pubmed    *PubMedSearcher
	client    *http.Client
}

// LiteratureSearchResult contains comprehensive literature search results.
type LiteratureSearchResult struct {
	Query           string              `json:"query"`
	TotalResults    int                 `json:"total_results"`
	Articles        []LiteratureArticle `json:"articles"`
	ClinicalTrials  []ClinicalTrial     `json:"clinical_trials,omitempty"`
	Guidelines      []ClinicalGuideline `json:"guidelines,omitempty"`
	SearchTimestamp time.Time           `json:"search_timestamp"`
}

// LiteratureArticle represents a medical literature article with enhanced metadata.
type LiteratureArticle struct {
	PMID          string   `json:"pmid"`
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Journal       string   `json:"journal"`
	Year          int      `json:"year"`
	Abstract      string   `json:"abstract"`
	DOI           string   `json:"doi"`
	Keywords      []string `json:"keywords"`
	MeSH          []string `json:"mesh_terms"`
	EvidenceLevel int      `json:"evidence_level"`
	StudyType     string   `json:"study_type"`
	SampleSize    int      `json:"sample_size"`
	CitationCount int      `json:"citation_count"`
	ImpactFactor  float64  `json:"impact_factor"`
	RelevanceScore float64 `json:"relevance_score"`
}

// ClinicalTrial represents a clinical trial from ClinicalTrials.gov.
type ClinicalTrial struct {
	NCTNumber     string   `json:"nct_number"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Phase         string   `json:"phase"`
	Conditions    []string `json:"conditions"`
	Interventions []string `json:"interventions"`
	Enrollment    int      `json:"enrollment"`
	StartDate     string   `json:"start_date"`
	CompletionDate string  `json:"completion_date"`
	Sponsor       string   `json:"sponsor"`
}

// NewLiteratureSearcher creates a new literature searcher.
func NewLiteratureSearcher(pubmedAPIKey string) *LiteratureSearcher {
	return &LiteratureSearcher{
		pubmed: NewPubMedSearcher(pubmedAPIKey),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchLiterature performs a comprehensive literature search.
func (l *LiteratureSearcher) SearchLiterature(ctx context.Context, query string, options SearchOptions) (*LiteratureSearchResult, error) {
	result := &LiteratureSearchResult{
		Query:           query,
		SearchTimestamp: time.Now(),
		Articles:        []LiteratureArticle{},
		ClinicalTrials:  []ClinicalTrial{},
		Guidelines:      []ClinicalGuideline{},
	}

	// Search PubMed
	pubmedResults, err := l.searchPubMed(ctx, query, options)
	if err == nil && pubmedResults != nil {
		result.Articles = pubmedResults
		result.TotalResults += len(pubmedResults)
	}

	// Search ClinicalTrials.gov if requested
	if options.IncludeTrials {
		trials, err := l.searchClinicalTrials(ctx, query, options)
		if err == nil && trials != nil {
			result.ClinicalTrials = trials
			result.TotalResults += len(trials)
		}
	}

	return result, nil
}

// SearchOptions configures literature search behavior.
type SearchOptions struct {
	MaxResults      int      `json:"max_results"`
	MinYear         int      `json:"min_year"`
	MaxYear         int      `json:"max_year"`
	StudyTypes      []string `json:"study_types"` // rct, meta_analysis, cohort, etc.
	MinEvidenceLevel int     `json:"min_evidence_level"`
	IncludeTrials   bool     `json:"include_trials"`
	SortBy          string   `json:"sort_by"` // relevance, date, citations
}

// searchPubMed searches PubMed with enhanced filtering.
func (l *LiteratureSearcher) searchPubMed(ctx context.Context, query string, options SearchOptions) ([]LiteratureArticle, error) {
	// Build enhanced query
	enhancedQuery := l.buildEnhancedQuery(query, options)

	// Search PubMed
	pubmedResult, err := l.pubmed.Search(ctx, enhancedQuery, options.MaxResults)
	if err != nil {
		return nil, err
	}

	// Convert to literature articles
	articles := make([]LiteratureArticle, 0, len(pubmedResult.Articles))
	for _, article := range pubmedResult.Articles {
		litArticle := LiteratureArticle{
			PMID:          article.PMID,
			Title:         article.Title,
			Authors:       article.Authors,
			Journal:       article.Journal,
			Abstract:      article.Abstract,
			DOI:           article.DOI,
			Keywords:      article.MeshTerms,
			MeSH:          article.MeshTerms,
			EvidenceLevel: article.EvidenceLevel,
			StudyType:     l.determineStudyType(article),
			RelevanceScore: l.calculateRelevance(article, query),
		}

		// Filter by year if specified
		if options.MinYear > 0 || options.MaxYear > 0 {
			year := l.extractYear(article.PubDate)
			if options.MinYear > 0 && year < options.MinYear {
				continue
			}
			if options.MaxYear > 0 && year > options.MaxYear {
				continue
			}
			litArticle.Year = year
		}

		// Filter by evidence level if specified
		if options.MinEvidenceLevel > 0 && litArticle.EvidenceLevel > options.MinEvidenceLevel {
			continue
		}

		// Filter by study type if specified
		if len(options.StudyTypes) > 0 && !l.matchesStudyType(litArticle.StudyType, options.StudyTypes) {
			continue
		}

		articles = append(articles, litArticle)
	}

	// Sort results
	l.sortArticles(articles, options.SortBy)

	return articles, nil
}

// searchClinicalTrials searches ClinicalTrials.gov.
func (l *LiteratureSearcher) searchClinicalTrials(ctx context.Context, query string, options SearchOptions) ([]ClinicalTrial, error) {
	// Build ClinicalTrials.gov API URL
	baseURL := "https://clinicaltrials.gov/api/v2/studies"
	params := url.Values{}
	params.Add("query.term", query)
	params.Add("pageSize", fmt.Sprintf("%d", options.MaxResults))
	
	if options.MinYear > 0 {
		params.Add("filter.advanced", fmt.Sprintf("AREA[StudyFirstPostDate]RANGE[%d0101,MAX]", options.MinYear))
	}

	apiURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clinicaltrials.gov API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response
	var ctResp struct {
		Studies []struct {
			ProtocolSection struct {
				IdentificationModule struct {
					NCTID        string `json:"NCTId"`
					OfficialTitle string `json:"OfficialTitle"`
				} `json:"IdentificationModule"`
				StatusModule struct {
					OverallStatus string `json:"OverallStatus"`
				} `json:"StatusModule"`
				DesignModules struct {
					DesignInfo struct {
						Phase string `json:"Phase"`
					} `json:"DesignInfo"`
				} `json:"DesignModules"`
				ConditionsModule struct {
					Conditions []string `json:"Conditions"`
				} `json:"ConditionsModule"`
				ArmsInterventionsModule struct {
					Interventions []struct {
						Name string `json:"InterventionName"`
					} `json:"Interventions"`
				} `json:"ArmsInterventionsModule"`
				EnrollmentModule struct {
					Count int `json:"EnrollCount"`
				} `json:"EnrollmentModule"`
				SponsorCollaboratorsModule struct {
					LeadSponsor struct {
						Name string `json:"Name"`
					} `json:"LeadSponsor"`
				} `json:"SponsorCollaboratorsModule"`
			} `json:"ProtocolSection"`
		} `json:"studies"`
	}

	if err := json.Unmarshal(body, &ctResp); err != nil {
		return nil, err
	}

	// Convert to clinical trials
	trials := make([]ClinicalTrial, 0, len(ctResp.Studies))
	for _, study := range ctResp.Studies {
		interventions := make([]string, 0)
		for _, intervention := range study.ProtocolSection.ArmsInterventionsModule.Interventions {
			interventions = append(interventions, intervention.Name)
		}

		trial := ClinicalTrial{
			NCTNumber:     study.ProtocolSection.IdentificationModule.NCTID,
			Title:         study.ProtocolSection.IdentificationModule.OfficialTitle,
			Status:        study.ProtocolSection.StatusModule.OverallStatus,
			Phase:         study.ProtocolSection.DesignModules.DesignInfo.Phase,
			Conditions:    study.ProtocolSection.ConditionsModule.Conditions,
			Interventions: interventions,
			Enrollment:    study.ProtocolSection.EnrollmentModule.Count,
			Sponsor:       study.ProtocolSection.SponsorCollaboratorsModule.LeadSponsor.Name,
		}
		trials = append(trials, trial)
	}

	return trials, nil
}

// buildEnhancedQuery builds an enhanced PubMed query with filters.
func (l *LiteratureSearcher) buildEnhancedQuery(query string, options SearchOptions) string {
	enhanced := query

	// Add study type filters
	if len(options.StudyTypes) > 0 {
		studyTypeFilters := []string{}
		for _, st := range options.StudyTypes {
			switch strings.ToLower(st) {
			case "rct", "randomized":
				studyTypeFilters = append(studyTypeFilters, `"randomized controlled trial"[Publication Type]`)
			case "meta_analysis", "meta-analysis":
				studyTypeFilters = append(studyTypeFilters, `"meta-analysis"[Publication Type]`)
			case "systematic_review":
				studyTypeFilters = append(studyTypeFilters, `"systematic review"[Publication Type]`)
			case "cohort":
				studyTypeFilters = append(studyTypeFilters, `"cohort studies"[Publication Type]`)
			case "case_control":
				studyTypeFilters = append(studyTypeFilters, `"case-control studies"[Publication Type]`)
			}
		}
		if len(studyTypeFilters) > 0 {
			enhanced += " AND (" + strings.Join(studyTypeFilters, " OR ") + ")"
		}
	}

	// Add year filter
	if options.MinYear > 0 {
		enhanced += fmt.Sprintf(" AND %d:%d[dp]", options.MinYear, 2030)
	}

	// Add human studies filter
	enhanced += " AND humans[filter]"

	return enhanced
}

// determineStudyType determines the study type from a PubMed article.
func (l *LiteratureSearcher) determineStudyType(article PubMedArticle) string {
	abstractLower := strings.ToLower(article.Abstract)
	
	if strings.Contains(abstractLower, "meta-analysis") || strings.Contains(abstractLower, "meta analysis") {
		return "meta-analysis"
	}
	if strings.Contains(abstractLower, "systematic review") {
		return "systematic review"
	}
	if strings.Contains(abstractLower, "randomized") || strings.Contains(abstractLower, "rct") {
		return "rct"
	}
	if strings.Contains(abstractLower, "cohort") {
		return "cohort"
	}
	if strings.Contains(abstractLower, "case-control") || strings.Contains(abstractLower, "case control") {
		return "case-control"
	}
	if strings.Contains(abstractLower, "case series") || strings.Contains(abstractLower, "case report") {
		return "case report"
	}
	if strings.Contains(abstractLower, "review") {
		return "review"
	}
	
	return "other"
}

// calculateRelevance calculates a relevance score for an article.
func (l *LiteratureSearcher) calculateRelevance(article PubMedArticle, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)
	
	// Title match (highest weight)
	if strings.Contains(strings.ToLower(article.Title), queryLower) {
		score += 0.5
	}
	
	// Abstract match
	if strings.Contains(strings.ToLower(article.Abstract), queryLower) {
		score += 0.3
	}
	
	// MeSH terms match
	for _, mesh := range article.MeshTerms {
		if strings.Contains(strings.ToLower(mesh), queryLower) {
			score += 0.2
			break
		}
	}
	
	// Evidence level bonus
	if article.EvidenceLevel <= 2 {
		score += 0.2
	}
	
	return score
}

// extractYear extracts the publication year from a date string.
func (l *LiteratureSearcher) extractYear(dateStr string) int {
	// Simple extraction - look for 4-digit year
	for i := 0; i < len(dateStr)-3; i++ {
		if dateStr[i] >= '0' && dateStr[i] <= '9' {
			year := 0
			for j := 0; j < 4 && i+j < len(dateStr); j++ {
				if dateStr[i+j] >= '0' && dateStr[i+j] <= '9' {
					year = year*10 + int(dateStr[i+j]-'0')
				}
			}
			if year >= 1900 && year <= 2030 {
				return year
			}
		}
	}
	return 0
}

// matchesStudyType checks if a study type matches the filter.
func (l *LiteratureSearcher) matchesStudyType(studyType string, filters []string) bool {
	studyTypeLower := strings.ToLower(studyType)
	for _, filter := range filters {
		if strings.Contains(studyTypeLower, strings.ToLower(filter)) {
			return true
		}
	}
	return false
}

// sortArticles sorts articles by the specified criteria.
func (l *LiteratureSearcher) sortArticles(articles []LiteratureArticle, sortBy string) {
	// Simple bubble sort for now
	for i := 0; i < len(articles); i++ {
		for j := i + 1; j < len(articles); j++ {
			shouldSwap := false
			switch sortBy {
			case "relevance":
				shouldSwap = articles[i].RelevanceScore < articles[j].RelevanceScore
			case "date":
				shouldSwap = articles[i].Year < articles[j].Year
			case "evidence":
				shouldSwap = articles[i].EvidenceLevel > articles[j].EvidenceLevel
			}
			if shouldSwap {
				articles[i], articles[j] = articles[j], articles[i]
			}
		}
	}
}
