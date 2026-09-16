package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/search"
)

// Runtime manages agent argument generation.
type Runtime struct {
	clientManager   *llm.ClientManager
	searcher        search.Searcher
	cache           *search.ResearchCache
	cfg             *config.Config
	OnResearchRound func(debateID string, agentID string, round ResearchRound) // callback for broadcasting research progress
}

// SetCache attaches a research cache to the runtime for deduplicating search API calls.
func (r *Runtime) SetCache(c *search.ResearchCache) {
	r.cache = c
}

// GetClient returns the default LLM client from the client manager.
func (r *Runtime) GetClient() *llm.Client {
	if r.clientManager == nil {
		return nil
	}
	pc := r.clientManager.GetBestClient()
	if pc == nil {
		return nil
	}
	return pc.Client
}

// FetchCitations is a public wrapper around the citation fetcher so the debate
// engine can call it after each argument (Phase 3 wiring).
func (r *Runtime) FetchCitations(ctx context.Context, results []models.SearchResult) []models.Citation {
	return r.fetchCitations(ctx, results)
}

// NewRuntime creates a new agent runtime.
func NewRuntime(clientManager *llm.ClientManager, searcher search.Searcher, cfg *config.Config) *Runtime {
	return &Runtime{
		clientManager: clientManager,
		searcher:      searcher,
		cfg:           cfg,
	}
}

// SearchAndGenerate searches for evidence then generates an argument with streaming.
// Returns the TranscriptEntry and streams tokens via onToken.
func (r *Runtime) SearchAndGenerate(
	ctx context.Context,
	agent models.Agent,
	session *models.DebateSession,
	phase string,
	onToken func(token string) error,
) (*models.TranscriptEntry, error) {
	// Step 1: Search for evidence (multi-query)
	searchQuery, searchResults := r.Search(ctx, agent, session, phase)

	// Step 2: Verify cited URLs (Phase 3 — real evidence)
	citations := r.fetchCitations(ctx, searchResults)

	// Step 3: Generate argument via LLM with streaming
	fullText, err := r.Generate(ctx, agent, session, phase, searchResults, onToken)
	if err != nil {
		return nil, fmt.Errorf("llm stream: %w", err)
	}

	entry := &models.TranscriptEntry{
		AgentID:       agent.ID,
		AgentName:     agent.Name,
		AgentRole:     agent.Role,
		Round:         session.Round,
		Phase:         models.DebateStatus(phase),
		Text:          fullText,
		SearchQuery:   searchQuery,
		SearchResults: searchResults,
		Citations:     citations,
	}

	return entry, nil
}

// fetchCitations HEAD-checks each URL returned by the searcher, recording
// reachability and the domain. Capped to 5 fetches per agent argument to
// keep tail latency bounded.
//
// Network errors, timeouts, and DNS failures all flip ReachabilityOK to false
// but do NOT cause the search to fail — a single bad URL should never block a
// debate.
func (r *Runtime) fetchCitations(ctx context.Context, results []models.SearchResult) []models.Citation {
	if len(results) == 0 {
		return nil
	}
	const maxFetches = 5
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	out := make([]models.Citation, 0, maxFetches)
	fetched := 0
	for _, sr := range results {
		if fetched >= maxFetches {
			break
		}
		if sr.URL == "" {
			continue
		}
		c := models.Citation{
			URL:           sr.URL,
			Title:         sr.Title,
			Snippet:       sr.Snippet,
			Domain:        sr.Domain,
			SourceFamily:  sr.Source,
			CrossVerified: sr.CrossVerified,
			FetchedAt:     time.Now().UTC(),
		}
		if c.Domain == "" {
			if u, err := url.Parse(sr.URL); err == nil {
				c.Domain = strings.TrimPrefix(u.Host, "www.")
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, sr.URL, nil)
		if err != nil {
			c.ReachabilityOK = false
			out = append(out, c)
			fetched++
			continue
		}
		req.Header.Set("User-Agent", "KampongBot/1.0 (+https://github.com/kampong)")
		resp, err := client.Do(req)
		if err != nil {
			c.ReachabilityOK = false
			out = append(out, c)
			fetched++
			continue
		}
		// Some servers reject HEAD with 4xx but a GET would succeed; treat any
		// 2xx or 3xx as reachable. Drain + close so the connection can be reused.
		_ = resp.Body.Close()
		c.ReachabilityOK = resp.StatusCode < 400
		out = append(out, c)
		fetched++
	}
	return out
}

// Search performs agentic iterative research: search → evaluate → search more → until sufficient.
// Returns the combined query string, all accumulated results, and research round details.
func (r *Runtime) Search(ctx context.Context, agent models.Agent, session *models.DebateSession, phase string) (string, []models.SearchResult) {
	const maxRounds = 3

	allResults := make([]models.SearchResult, 0)
	seenURLs := make(map[string]bool)
	var allFindings []string
	var queryParts []string
	var rounds []ResearchRound

	// Round 1: Initial search with multiple queries
	initialQueries := r.buildSearchQueries(agent, session, phase)
	round1Results := r.executeSearches(ctx, initialQueries, seenURLs)
	queryParts = append(queryParts, initialQueries...)
	allResults = append(allResults, round1Results...)

	round1 := ResearchRound{
		Round:   1,
		Queries: initialQueries,
		Results: round1Results,
	}
	rounds = append(rounds, round1)

	// Broadcast round 1
	if r.OnResearchRound != nil {
		r.OnResearchRound(session.ID, agent.ID, round1)
	}

	// Rounds 2+: LLM evaluates findings and decides if more research is needed
	for round := 2; round <= maxRounds; round++ {
		// Ask LLM to evaluate current research
		plan := r.evaluateResearch(ctx, agent, session, allResults, allFindings, round-1)

		if plan.Sufficient || len(plan.FollowUpQueries) == 0 {
			log.Printf("RESEARCH [%s] agent=%s round=%d: sufficient=%v (findings=%d)",
				session.ID, agent.Name, round-1, plan.Sufficient, len(plan.KeyFindings))
			break
		}

		log.Printf("RESEARCH [%s] agent=%s round=%d: doing follow-up (%s)",
			session.ID, agent.Name, round, plan.Reasoning)

		// Execute follow-up searches
		followUpResults := r.executeSearches(ctx, plan.FollowUpQueries, seenURLs)
		if len(followUpResults) == 0 {
			break // No new results, stop researching
		}

		queryParts = append(queryParts, plan.FollowUpQueries...)
		allResults = append(allResults, followUpResults...)
		allFindings = append(allFindings, plan.KeyFindings...)

		roundData := ResearchRound{
			Round:     round,
			Queries:   plan.FollowUpQueries,
			Results:   followUpResults,
			Findings:  plan.KeyFindings,
			Reasoning: plan.Reasoning,
		}
		rounds = append(rounds, roundData)

		// Broadcast this round
		if r.OnResearchRound != nil {
			r.OnResearchRound(session.ID, agent.ID, roundData)
		}
	}

	// Cap at 10 results total
	if len(allResults) > 10 {
		allResults = allResults[:10]
	}

	return strings.Join(queryParts, " | "), allResults
}

// executeSearches runs multiple search queries and deduplicates results.
func (r *Runtime) executeSearches(ctx context.Context, queries []string, seenURLs map[string]bool) []models.SearchResult {
	var results []models.SearchResult
	for _, q := range queries {
		// Check cache first to avoid duplicate API calls
		if r.cache != nil {
			if cached, ok := r.cache.Get(q); ok {
				log.Printf("SEARCH CACHE HIT: query=%q results=%d", q, len(cached))
				for _, item := range cached {
					if item.URL != "" && seenURLs[item.URL] {
						continue
					}
					if item.URL != "" {
						seenURLs[item.URL] = true
					}
					results = append(results, item)
				}
				continue
			}
		}

		if r.searcher == nil {
			continue
		}
		res, err := r.searcher.Search(ctx, q)
		if err != nil {
			log.Printf("SEARCH ERROR: query=%q err=%v", q, err)
			continue
		}

		// Store results in cache for future reuse
		if r.cache != nil {
			r.cache.Set(q, res)
		}

		for _, item := range res {
			if item.URL != "" && seenURLs[item.URL] {
				continue
			}
			if item.URL != "" {
				seenURLs[item.URL] = true
			}
			results = append(results, item)
		}
	}
	return results
}

// evaluateResearch asks the LLM to evaluate current research and plan follow-ups.
func (r *Runtime) evaluateResearch(ctx context.Context, agent models.Agent, session *models.DebateSession, results []models.SearchResult, findings []string, roundNum int) ResearchPlanResponse {
	roleGuidance := getRoleGuidance(agent.RoleType)

	tmpl, err := template.New("research_plan").Parse(ResearchPlannerPrompt)
	if err != nil {
		log.Printf("RESEARCH PLAN TEMPLATE ERROR: %v", err)
		return ResearchPlanResponse{Sufficient: true}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ResearchPlanData{
		Topic:         session.Topic,
		RoleName:      agent.Name,
		RoleType:      string(agent.RoleType),
		Expertise:     agent.Expertise,
		RoleGuidance:  roleGuidance,
		ResearchRound: roundNum,
		SearchResults: results,
		KeyFindings:   findings,
	}); err != nil {
		log.Printf("RESEARCH PLAN EXEC ERROR: %v", err)
		return ResearchPlanResponse{Sufficient: true}
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a research assistant planning searches for a debate. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	// Get the client for this agent's provider
	client, err := r.clientManager.GetClient(agent.ProviderName)
	if err != nil {
		log.Printf("RESEARCH PLAN CLIENT ERROR: %v", err)
		return ResearchPlanResponse{Sufficient: true}
	}

	rawJSON, err := client.Complete(ctx, messages, 0.3, 16384)
	if err != nil {
		log.Printf("RESEARCH PLAN LLM ERROR: %v", err)
		return ResearchPlanResponse{Sufficient: true}
	}

	// Clean response
	rawJSON = strings.TrimSpace(rawJSON)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var plan ResearchPlanResponse
	if err := json.Unmarshal([]byte(rawJSON), &plan); err != nil {
		log.Printf("RESEARCH PLAN PARSE ERROR: %v (raw: %s)", err, rawJSON[:min(len(rawJSON), 200)])
		return ResearchPlanResponse{Sufficient: true}
	}

	// Cap follow-up queries at 2
	if len(plan.FollowUpQueries) > 2 {
		plan.FollowUpQueries = plan.FollowUpQueries[:2]
	}

	return plan
}

func getRoleGuidance(roleType models.AgentRoleType) string {
	switch roleType {
	case models.RoleCore:
		return "Bring deep technical knowledge. Cite specific data, studies, and established findings."
	case models.RoleAdjacent:
		return "Connect ideas across domains. Show how your field reveals insights core experts miss."
	case models.RolePractitioner:
		return "Ground the debate in real-world reality. Share what actually happens in practice."
	case models.RoleLayperson:
		return "Ask the obvious questions. Challenge assumptions that only make sense inside a specialist bubble."
	default:
		return "Provide a well-researched perspective."
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Generate produces the agent's argument via LLM with streaming.
func (r *Runtime) Generate(
	ctx context.Context,
	agent models.Agent,
	session *models.DebateSession,
	phase string,
	searchResults []models.SearchResult,
	onToken func(token string) error,
) (string, error) {
	// Build system prompt with graph context
	systemPrompt, err := AgentSystemPrompt(PromptData{
		Topic:           session.Topic,
		RoleName:        agent.Name,
		RoleType:        string(agent.RoleType),
		Expertise:       agent.Expertise,
		Phase:           phase,
		Round:           session.Round,
		TotalRounds:     session.TotalRounds,
		SearchResults:   searchResults,
		RecentArguments: session.GetRecentTranscript(r.cfg.Debate.Context.FullDetailRounds * len(session.Agents)),
		GraphClaims:     r.getGraphClaims(session),
		Citations:       r.fetchCitations(ctx, searchResults),
	})
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Present your %s argument on: %s", phase, session.Topic)},
	}

	// Get the client for this agent's provider
	client, err := r.clientManager.GetClient(agent.ProviderName)
	if err != nil {
		return "", fmt.Errorf("get client for provider %q: %w", agent.ProviderName, err)
	}

	// Stream with the assigned provider; on hard failure BEFORE any token is
	// emitted, fail over to other enabled providers (different families first).
	emitted := 0
	out, err := client.Stream(ctx, messages, r.cfg.LLM.Temperature, r.cfg.LLM.MaxTokens, func(token string) error {
		emitted++
		return onToken(token)
	})
	if err != nil && emitted == 0 && ctx.Err() == nil {
		for _, alt := range r.clientManager.FailoverCandidates(agent.ProviderName) {
			log.Printf("FAILOVER [%s] provider %q -> %q: %v", session.ID, agent.ProviderName, alt.Name, err)
			out, err = alt.Stream(ctx, messages, r.cfg.LLM.Temperature, r.cfg.LLM.MaxTokens, func(token string) error {
				emitted++
				return onToken(token)
			})
			if err == nil {
				break
			}
		}
	}

	return out, err
}

// getGraphClaims extracts recent claims from the knowledge graph for context injection.
func (r *Runtime) getGraphClaims(session *models.DebateSession) []GraphClaim {
	var claims []GraphClaim
	for _, n := range session.Graph.Nodes {
		if n.Type == "claim" && n.Content != "" {
			claims = append(claims, GraphClaim{
				Label:     n.Label,
				Content:   n.Content,
				AgentName: n.AgentName,
				Type:      n.Type,
			})
		}
	}
	// Cap at 10 claims to avoid prompt bloat
	if len(claims) > 10 {
		claims = claims[len(claims)-10:]
	}
	return claims
}

// buildSearchQueries constructs multiple context-aware search queries.
func (r *Runtime) buildSearchQueries(agent models.Agent, session *models.DebateSession, phase string) []string {
	var queries []string

	// Primary query: topic + expertise
	queries = append(queries, session.Topic+" "+agent.Expertise)

	// Secondary query: in rebuttal phases, search for counter-evidence
	if phase == "rebuttal" || phase == "final_rebuttal" {
		recent := session.GetRecentTranscript(3)
		for _, entry := range recent {
			if entry.AgentID != agent.ID {
				counterQuery := session.Topic + " counter evidence " + truncateText(entry.Text, 80)
				queries = append(queries, counterQuery)
				break
			}
		}
	} else if phase == "opening" {
		// In opening, search for foundational data/statistics
		queries = append(queries, session.Topic+" statistics data research")
	}

	return queries
}

// GraphClaim is a lightweight claim from the knowledge graph for prompt injection.
type GraphClaim struct {
	Label     string
	Content   string
	AgentName string
	Type      string
}

// ResearchPlanData is the template data for the research planner prompt.
type ResearchPlanData struct {
	Topic         string
	RoleName      string
	RoleType      string
	Expertise     string
	RoleGuidance  string
	ResearchRound int
	SearchResults []models.SearchResult
	KeyFindings   []string
}

// ResearchPlanResponse is the LLM's structured response to the research planner.
type ResearchPlanResponse struct {
	Sufficient      bool     `json:"sufficient"`
	KeyFindings     []string `json:"key_findings"`
	FollowUpQueries []string `json:"follow_up_queries"`
	Reasoning       string   `json:"reasoning"`
}

// ResearchRound holds results from one round of research.
type ResearchRound struct {
	Round       int
	Queries     []string
	Results     []models.SearchResult
	Findings    []string
	Reasoning   string
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
