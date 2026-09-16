package analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

const complexityPromptTmpl = `You are a debate complexity assessor. Analyze this topic and return ONLY valid JSON.

Topic: %s

Return JSON with this exact structure:
{
  "score": "simple|moderate|complex|highly_complex",
  "recommended_mode": "quick|standard|deep|discussion",
  "reasons": ["brief reason 1", "brief reason 2"],
  "controversy_level": 0-100,
  "domain_count": number_of_distinct_domains
}

Scoring guide:
- simple: straightforward factual topic, single domain, low controversy → quick
- moderate: some debate possible, 1-2 domains, mild controversy → standard
- complex: multi-domain, requires expertise from several fields → deep
- highly_complex: controversial + multi-domain, strong opinions on multiple sides → deep or discussion

Consider:
- How many distinct academic/professional domains are involved?
- How emotionally or politically charged is the topic?
- Is there active disagreement among experts?
- Does the topic require specialized knowledge to evaluate?`

// AssessComplexity evaluates topic difficulty and recommends debate mode.
// Uses a lightweight LLM call (low temperature, few tokens) for efficiency.
func AssessComplexity(ctx context.Context, client *llm.Client, topic string) (*models.DebateComplexity, error) {
	prompt := fmt.Sprintf(complexityPromptTmpl, topic)

	messages := []llm.Message{
		{Role: "system", Content: "You are a debate complexity assessor. Return ONLY valid JSON, no markdown fences."},
		{Role: "user", Content: prompt},
	}

	rawJSON, err := client.Complete(ctx, messages, 0.1, 512)
	if err != nil {
		return nil, fmt.Errorf("complexity llm call: %w", err)
	}

	rawJSON = cleanJSON(rawJSON)

	var complexity models.DebateComplexity
	if err := json.Unmarshal([]byte(rawJSON), &complexity); err != nil {
		return nil, fmt.Errorf("parse complexity response: %w (raw: %.200s)", err, rawJSON)
	}

	// Validate and clamp fields
	switch complexity.Score {
	case "simple", "moderate", "complex", "highly_complex":
		// valid
	default:
		complexity.Score = "moderate"
	}

	switch complexity.RecommendedMode {
	case "quick", "standard", "deep", "discussion":
		// valid
	default:
		complexity.RecommendedMode = "standard"
	}

	if complexity.ControversyLevel < 0 {
		complexity.ControversyLevel = 0
	}
	if complexity.ControversyLevel > 100 {
		complexity.ControversyLevel = 100
	}
	if complexity.DomainCount < 1 {
		complexity.DomainCount = 1
	}

	return &complexity, nil
}
