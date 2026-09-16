package graph

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"text/template"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

// Extractor extracts claims and relationships from agent arguments using an LLM.
type Extractor struct {
	client *llm.Client
	store  *Store
}

// NewExtractor creates a graph extractor.
func NewExtractor(client *llm.Client, store *Store) *Extractor {
	return &Extractor{client: client, store: store}
}

// SetClient updates the LLM client used by the extractor.
func (e *Extractor) SetClient(client *llm.Client) {
	e.client = client
}

// graphExtractResponse is the LLM's JSON output.
type graphExtractResponse struct {
	NewNodes []models.GraphNode `json:"new_nodes"`
	NewEdges []models.GraphEdge `json:"new_edges"`
}

const extractPromptTmpl = `You are a knowledge graph extractor. Extract claims from this debate argument.

Speaker: {{.AgentName}}
Argument: {{.ArgumentText}}
{{if .ExistingNodes}}
Existing nodes (reuse these IDs when same concept):
{{range .ExistingNodes}}
- "{{.ID}}": {{.Label}}
{{end}}
{{end}}
Return ONLY JSON: {"new_nodes":[{"id":"slug","label":"short label","type":"claim","content":"full claim text"}],"new_edges":[{"from":"id","to":"id","relation":"supports|contradicts"}]}
Max 5 nodes. Reuse existing IDs. Every node needs an edge.`

// ExtractAndMerge runs graph extraction on an argument and merges results into the store.
func (e *Extractor) ExtractAndMerge(ctx context.Context, entry models.TranscriptEntry, agentID string, searchResults []models.SearchResult) {
	// Step 1: Create a claim node from the argument
	claimNode := e.createClaimNode(entry, agentID, searchResults)
	e.store.AddNodes([]models.GraphNode{claimNode})

	// Step 2: Extract concept nodes from key entities in the argument
	conceptNodes := e.extractConceptNodes(entry, agentID)
	if len(conceptNodes) > 0 {
		e.store.AddNodes(conceptNodes)
	}

	// Step 3: Try LLM extraction for richer nodes + edges
	llmSucceeded := false
	tmpl, err := template.New("extract").Parse(extractPromptTmpl)
	if err == nil {
		var buf bytes.Buffer
		if err = tmpl.Execute(&buf, map[string]interface{}{
			"AgentName":     entry.AgentName,
			"AgentRole":     entry.AgentRole,
			"ArgumentText":  truncateArg(entry.Text, 800),
			"ExistingNodes": e.store.GetAllNodes(),
		}); err == nil {
			messages := []llm.Message{
				{Role: "system", Content: "Extract claims as JSON. Return ONLY JSON, no thinking, no explanation."},
				{Role: "user", Content: buf.String()},
			}

			rawJSON, err := e.client.Complete(ctx, messages, 0.2, 16384)
			if err == nil && rawJSON != "" {
				rawJSON = cleanJSON(rawJSON)
				var resp graphExtractResponse
				if err = json.Unmarshal([]byte(rawJSON), &resp); err == nil && len(resp.NewNodes) > 0 {
					llmSucceeded = true
					for i := range resp.NewEdges {
						resp.NewEdges[i].Provenance = agentID
					}
					for i := range resp.NewNodes {
						resp.NewNodes[i].AgentID = agentID
						resp.NewNodes[i].AgentName = entry.AgentName
						resp.NewNodes[i].Round = entry.Round
						resp.NewNodes[i].Phase = string(entry.Phase)
					}
					e.store.Merge(models.GraphUpdate{
						NewNodes: resp.NewNodes,
						NewEdges: resp.NewEdges,
					})
				}
			}
			if err != nil {
				log.Printf("GRAPH EXTRACTOR LLM ERROR: %v", err)
			}
		}
	}

	// Step 4: Auto-connect ALL nodes aggressively
	e.autoConnectAll(agentID, claimNode, conceptNodes, llmSucceeded)
}

// createClaimNode creates a claim node from the agent's argument, with source URLs attached.
func (e *Extractor) createClaimNode(entry models.TranscriptEntry, agentID string, searchResults []models.SearchResult) models.GraphNode {
	text := strings.TrimSpace(entry.Text)
	label := text
	if idx := strings.Index(text, ". "); idx > 0 && idx < 120 {
		label = text[:idx+1]
	} else if len(text) > 120 {
		label = text[:117] + "..."
	}

	nodeID := fmt.Sprintf("claim-%x", md5.Sum([]byte(fmt.Sprintf("%s%d%s", agentID, entry.Round, label))))[:16]

	// Build source URLs string from search results
	var sourceURLs []string
	for _, sr := range searchResults {
		if sr.URL != "" {
			sourceURLs = append(sourceURLs, sr.URL)
		}
	}

	return models.GraphNode{
		ID:        nodeID,
		Label:     label,
		Type:      "claim",
		Content:   text,
		AgentID:   agentID,
		AgentName: entry.AgentName,
		Round:     entry.Round,
		Phase:     string(entry.Phase),
		Refs:      1,
	}
}

// extractConceptNodes extracts key concepts/entities from the argument as concept nodes.
func (e *Extractor) extractConceptNodes(entry models.TranscriptEntry, agentID string) []models.GraphNode {
	// Extract important multi-word phrases and single entities
	entities := extractEntities(entry.Text)
	var nodes []models.GraphNode

	for _, entity := range entities {
		nodeID := fmt.Sprintf("concept-%x", md5.Sum([]byte(strings.ToLower(entity))))[:16]
		nodes = append(nodes, models.GraphNode{
			ID:        nodeID,
			Label:     entity,
			Type:      "concept",
			Content:   entity + " — discussed by " + entry.AgentName,
			AgentID:   agentID,
			AgentName: entry.AgentName,
			Round:     entry.Round,
			Phase:     string(entry.Phase),
			Refs:      1,
		})
	}
	return nodes
}

// extractEntities extracts key entities/phrases from text.
func extractEntities(text string) []string {
	// Common domain-specific multi-word phrases to look for
	// This is a simple approach — extract capitalized multi-word phrases and key terms
	words := strings.Fields(text)
	entityCount := make(map[string]int)

	// Extract bigrams and trigrams that contain capitalized words or key terms
	for i := 0; i < len(words)-1; i++ {
		w1 := strings.Trim(words[i], ".,;:!?\"'()[]{}")
		w2 := strings.Trim(words[i+1], ".,;:!?\"'()[]{}")

		// Bigram: if first word is capitalized (proper noun) or both are meaningful
		if isCapitalized(w1) && len(w2) > 2 {
			bigram := w1 + " " + w2
			if !isStopPhrase(bigram) {
				entityCount[bigram]++
			}
		}

		// Trigram
		if i < len(words)-2 {
			w3 := strings.Trim(words[i+2], ".,;:!?\"'()[]{}")
			if isCapitalized(w1) && len(w2) > 2 && len(w3) > 2 {
				trigram := w1 + " " + w2 + " " + w3
				if !isStopPhrase(trigram) {
					entityCount[trigram]++
				}
			}
		}
	}

	// Also extract single key terms (lowercase, meaningful)
	keyTerms := extractKeyTerms(text)
	for _, t := range keyTerms {
		entityCount[t]++
	}

	// Sort by frequency, take top 5
	type entityFreq struct {
		entity string
		count  int
	}
	var sorted []entityFreq
	for e, c := range entityCount {
		sorted = append(sorted, entityFreq{e, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	var entities []string
	seen := make(map[string]bool)
	for _, ef := range sorted {
		if len(entities) >= 5 {
			break
		}
		// Deduplicate: skip if this entity is a substring of an existing one
		isDup := false
		for existing := range seen {
			if strings.Contains(existing, ef.entity) || strings.Contains(ef.entity, existing) {
				isDup = true
				break
			}
		}
		if !isDup {
			entities = append(entities, ef.entity)
			seen[ef.entity] = true
		}
	}
	return entities
}

// extractKeyTerms extracts important single-word terms from text.
func extractKeyTerms(text string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"about": true, "that": true, "this": true, "it": true, "and": true,
		"or": true, "but": true, "not": true, "no": true, "if": true,
		"than": true, "then": true, "so": true, "also": true, "more": true,
		"which": true, "their": true, "there": true, "what": true, "when": true,
		"where": true, "who": true, "how": true, "why": true, "its": true,
		"they": true, "them": true, "our": true, "we": true,
		"you": true, "your": true, "my": true, "his": true, "her": true,
		"very": true, "just": true, "some": true, "any": true, "all": true,
		"each": true, "every": true, "both": true, "few": true, "most": true,
		"other": true, "such": true, "only": true, "own": true, "same": true,
	}
	wordCount := make(map[string]int)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) > 3 && !stopWords[w] {
			wordCount[w]++
		}
	}

	type wordFreq struct {
		word  string
		count int
	}
	var sorted []wordFreq
	for w, c := range wordCount {
		sorted = append(sorted, wordFreq{w, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	var terms []string
	for i, wf := range sorted {
		if i >= 8 {
			break
		}
		terms = append(terms, wf.word)
	}
	return terms
}

func isCapitalized(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'A' && s[0] <= 'Z'
}

func isStopPhrase(s string) bool {
	lower := strings.ToLower(s)
	stops := []string{"the ", "a ", "an ", "is ", "are ", "was ", "were ", "has ", "have ", "had ",
		"will ", "would ", "could ", "should ", "may ", "might ", "can ", "that ", "this ",
		"it ", "and ", "but ", "for ", "not ", "with ", "from ", "into ", "about "}
	for _, stop := range stops {
		if strings.HasPrefix(lower, stop) {
			return true
		}
	}
	return false
}

// autoConnectAll connects claim nodes to concept nodes and to each other.
func (e *Extractor) autoConnectAll(agentID string, claimNode models.GraphNode, conceptNodes []models.GraphNode, llmSucceeded bool) {
	allNodes := e.store.GetAllNodes()
	allEdges := e.store.GetAllEdges()

	hasEdge := func(from, to string) bool {
		for _, e := range allEdges {
			if e.From == from && e.To == to {
				return true
			}
		}
		return false
	}

	var newEdges []models.GraphEdge

	// 1. Connect claim → its own concept nodes
	for _, cn := range conceptNodes {
		if !hasEdge(claimNode.ID, cn.ID) {
			newEdges = append(newEdges, models.GraphEdge{
				From:       claimNode.ID,
				To:         cn.ID,
				Relation:   "cites",
				Provenance: agentID,
			})
		}
	}

	// 2. Connect to concept nodes from OTHER agents that share the same concept
	for _, cn := range conceptNodes {
		for _, n := range allNodes {
			if n.ID == cn.ID || n.Type != "concept" {
				continue
			}
			// Same label (case-insensitive) = same concept
			if strings.EqualFold(n.Label, cn.Label) && !hasEdge(claimNode.ID, n.ID) {
				newEdges = append(newEdges, models.GraphEdge{
					From:       claimNode.ID,
					To:         n.ID,
					Relation:   "supports",
					Provenance: agentID,
				})
			}
		}
	}

	// 3. Connect claim to other claims from different agents by word overlap
	if !llmSucceeded {
		for _, n := range allNodes {
			if n.ID == claimNode.ID || n.Type != "claim" || n.AgentID == agentID {
				continue
			}
			if n.Content == "" || claimNode.Content == "" {
				continue
			}
			similarity := wordOverlap(claimNode.Content, n.Content)
			if similarity > 0.08 && !hasEdge(claimNode.ID, n.ID) && !hasEdge(n.ID, claimNode.ID) {
				relation := "supports"
				// Check for contradiction signals
				if hasContradictionSignal(claimNode.Content, n.Content) {
					relation = "contradicts"
				}
				newEdges = append(newEdges, models.GraphEdge{
					From:       claimNode.ID,
					To:         n.ID,
					Relation:   relation,
					Provenance: agentID,
				})
			}
		}
	}

	// 4. Connect same-agent claims chronologically
	for _, n := range allNodes {
		if n.ID == claimNode.ID || n.Type != "claim" || n.AgentID != agentID {
			continue
		}
		if !hasEdge(claimNode.ID, n.ID) && !hasEdge(n.ID, claimNode.ID) {
			newEdges = append(newEdges, models.GraphEdge{
				From:       claimNode.ID,
				To:         n.ID,
				Relation:   "supports",
				Provenance: agentID,
			})
		}
	}

	if len(newEdges) > 0 {
		e.store.AddEdges(newEdges)
	}
}

// hasContradictionSignal checks if two texts contain contradiction indicators.
func hasContradictionSignal(a, b string) bool {
	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)
	contradictionWords := []string{
		"disagree", "wrong", "incorrect", "false", "not true",
		"however", "but", "contrary", "opposite", "myth",
		"overrated", "overblown", "exaggerated", "misleading",
	}
	for _, w := range contradictionWords {
		if strings.Contains(aLower, w) || strings.Contains(bLower, w) {
			return true
		}
	}
	return false
}

// wordOverlap calculates simple word overlap ratio between two texts.
func wordOverlap(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}
	overlap := 0
	for w := range wordsA {
		if wordsB[w] {
			overlap++
		}
	}
	return float64(overlap) / float64(min(len(wordsA), len(wordsB)))
}

// tokenize splits text into lowercase words, filtering short/common words.
func tokenize(text string) map[string]bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"about": true, "that": true, "this": true, "it": true, "and": true,
		"or": true, "but": true, "not": true, "no": true, "if": true,
		"than": true, "then": true, "so": true, "also": true, "more": true,
	}
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) > 2 && !stopWords[w] {
			words[w] = true
		}
	}
	return words
}

func truncateArg(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// GetStore returns the underlying graph store.
func (e *Extractor) GetStore() *Store {
	return e.store
}
