package debate

import (
	"fmt"
	"strings"

	"github.com/kampong/debate/internal/models"
)

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"and": true, "but": true, "or": true, "nor": true, "not": true,
	"so": true, "yet": true, "both": true, "each": true, "every": true,
	"all": true, "any": true, "few": true, "more": true, "most": true,
	"other": true, "some": true, "such": true, "no": true, "only": true,
	"own": true, "same": true, "than": true, "too": true, "very": true,
	"just": true, "because": true, "if": true, "when": true, "where": true,
	"how": true, "what": true, "which": true, "who": true, "whom": true,
	"this": true, "that": true, "these": true, "those": true, "it": true,
	"its": true, "we": true, "they": true, "you": true, "he": true,
	"she": true, "me": true, "him": true, "her": true, "us": true,
	"them": true, "my": true, "your": true, "his": true, "their": true,
	"our": true, "i": true, "also": true,
}

func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	seen := make(map[string]bool)
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()-")
		if len(w) <= 3 || stopWords[w] {
			continue
		}
		if !seen[w] {
			seen[w] = true
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func keywordOverlap(a, b []string) float64 {
	setA := make(map[string]bool, len(a))
	for _, k := range a {
		setA[k] = true
	}
	setB := make(map[string]bool, len(b))
	for _, k := range b {
		setB[k] = true
	}
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	intersection := 0
	for k := range setA {
		if setB[k] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// CheckConsistency analyzes each agent's claims across rounds for contradictions.
func CheckConsistency(transcript []models.TranscriptEntry) []models.ConsistencyCheck {
	type agentData struct {
		name    string
		entries []models.TranscriptEntry
	}
	agentMap := make(map[string]*agentData)
	var agentOrder []string

	for _, entry := range transcript {
		if entry.IsUserInject || entry.IsChallenge {
			continue
		}
		ad, ok := agentMap[entry.AgentID]
		if !ok {
			ad = &agentData{name: entry.AgentName}
			agentMap[entry.AgentID] = ad
			agentOrder = append(agentOrder, entry.AgentID)
		}
		ad.entries = append(ad.entries, entry)
	}

	var results []models.ConsistencyCheck

	for _, agentID := range agentOrder {
		ad := agentMap[agentID]

		type roundKW struct {
			round    int
			text     string
			keywords []string
		}
		var rounds []roundKW
		for _, e := range ad.entries {
			rounds = append(rounds, roundKW{
				round:    e.Round,
				text:     e.Text,
				keywords: extractKeywords(e.Text),
			})
		}

		if len(rounds) < 2 {
			var claims []string
			for _, e := range ad.entries {
				claims = append(claims, truncateText(e.Text, 120))
			}
			results = append(results, models.ConsistencyCheck{
				AgentID:        agentID,
				AgentName:      ad.name,
				Claims:         claims,
				ConsistencyPct: 100,
			})
			continue
		}

		var contradictions []string
		totalPairs := 0
		consistentPairs := 0

		for i := 0; i < len(rounds); i++ {
			for j := i + 1; j < len(rounds); j++ {
				totalPairs++
				overlap := keywordOverlap(rounds[i].keywords, rounds[j].keywords)
				if overlap < 0.15 && len(rounds[i].keywords) > 3 && len(rounds[j].keywords) > 3 {
					contradictions = append(contradictions,
						fmt.Sprintf("Low overlap (%.0f%%) between Round %d and Round %d",
							overlap*100, rounds[i].round, rounds[j].round))
				} else {
					consistentPairs++
				}
			}
		}

		consistencyPct := 100
		if totalPairs > 0 {
			consistencyPct = int(float64(consistentPairs) / float64(totalPairs) * 100)
		}

		var claims []string
		for _, e := range ad.entries {
			claims = append(claims, truncateText(e.Text, 120))
		}

		results = append(results, models.ConsistencyCheck{
			AgentID:        agentID,
			AgentName:      ad.name,
			Claims:         claims,
			Contradictions: contradictions,
			ConsistencyPct: consistencyPct,
		})
	}

	return results
}

func truncateText(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
