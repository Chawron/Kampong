package social

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

// DefaultPersonas returns the default set of 12 social reaction personas.
func DefaultPersonas() []models.SocialPersona {
	return []models.SocialPersona{
		{ID: "concerned_parent", Name: "Concerned Parent", Persona: "concerned_parent", Bias: "skeptical", KnowledgeLvl: "casual"},
		{ID: "science_journalist", Name: "Science Journalist", Persona: "science_journalist", Bias: "neutral", KnowledgeLvl: "informed"},
		{ID: "corporate_executive", Name: "Corporate Executive", Persona: "corporate_executive", Bias: "supportive", KnowledgeLvl: "casual"},
		{ID: "patient_advocate", Name: "Patient Advocate", Persona: "patient_advocate", Bias: "supportive", KnowledgeLvl: "informed"},
		{ID: "conspiracy_skeptic", Name: "Conspiracy Skeptic", Persona: "conspiracy_skeptic", Bias: "hostile", KnowledgeLvl: "uninformed"},
		{ID: "policy_maker", Name: "Policy Maker", Persona: "policy_maker", Bias: "neutral", KnowledgeLvl: "informed"},
		{ID: "social_media_influencer", Name: "Social Media Influencer", Persona: "social_media_influencer", Bias: "neutral", KnowledgeLvl: "uninformed"},
		{ID: "healthcare_worker", Name: "Healthcare Worker", Persona: "healthcare_worker", Bias: "supportive", KnowledgeLvl: "expert"},
		{ID: "elderly_citizen", Name: "Elderly Citizen", Persona: "elderly_citizen", Bias: "skeptical", KnowledgeLvl: "casual"},
		{ID: "university_student", Name: "University Student", Persona: "university_student", Bias: "neutral", KnowledgeLvl: "informed"},
		{ID: "insurance_analyst", Name: "Insurance Analyst", Persona: "insurance_analyst", Bias: "skeptical", KnowledgeLvl: "expert"},
		{ID: "community_leader", Name: "Community Leader", Persona: "community_leader", Bias: "supportive", KnowledgeLvl: "casual"},
	}
}

// personaReactionJSON is the expected LLM output for a single persona reaction.
type personaReactionJSON struct {
	Reaction   string `json:"reaction"`
	Sentiment  string `json:"sentiment"`
	Amplify    string `json:"amplify"`
	Distort    string `json:"distort"`
	TrustScore int    `json:"trust_score"`
}

// narrativeAnalysisJSON is the expected LLM output for the overall narrative analysis.
type narrativeAnalysisJSON struct {
	NarrativeSummary  string   `json:"narrative_summary"`
	DominantNarrative string   `json:"dominant_narrative"`
	RiskAreas         []string `json:"risk_areas"`
	OverallSentiment  string   `json:"overall_sentiment"`
}

// RunSimulation runs the social reaction simulation.
// For each persona, generate a reaction to the debate verdict/synthesis.
// Then analyze the overall narrative landscape.
func RunSimulation(ctx context.Context, client *llm.Client, topic string, verdict string, synthesis string) (*models.SocialSimulation, error) {
	personas := DefaultPersonas()

	// Build the verdict/synthesis context for the prompt
	verdictOrSynthesis := verdict
	if synthesis != "" {
		if verdict != "" {
			verdictOrSynthesis = verdict + "\n\nSynthesis:\n" + synthesis
		} else {
			verdictOrSynthesis = synthesis
		}
	}

	// Generate reactions for each persona
	reactions := make([]models.SocialReaction, 0, len(personas))
	for _, persona := range personas {
		reaction, err := generatePersonaReaction(ctx, client, topic, persona, verdictOrSynthesis)
		if err != nil {
			log.Printf("SOCIAL: Error generating reaction for persona %s: %v", persona.ID, err)
			// Add a placeholder reaction so we still have all personas represented
			reactions = append(reactions, models.SocialReaction{
				PersonaID:   persona.ID,
				PersonaName: persona.Name,
				PersonaType: persona.Persona,
				Reaction:    "Unable to generate reaction.",
				Sentiment:   "neutral",
				TrustScore:  50,
			})
			continue
		}
		reactions = append(reactions, *reaction)
	}

	// Generate overall narrative analysis
	narrative, err := generateNarrativeAnalysis(ctx, client, reactions)
	if err != nil {
		log.Printf("SOCIAL: Error generating narrative analysis: %v", err)
		narrative = &narrativeAnalysisJSON{
			NarrativeSummary:  "Narrative analysis unavailable.",
			DominantNarrative: "Unknown",
			RiskAreas:         []string{},
			OverallSentiment:  "neutral",
		}
	}

	return &models.SocialSimulation{
		Personas:          personas,
		Reactions:         reactions,
		NarrativeSummary:  narrative.NarrativeSummary,
		DominantNarrative: narrative.DominantNarrative,
		RiskAreas:         narrative.RiskAreas,
		OverallSentiment:  narrative.OverallSentiment,
	}, nil
}

// generatePersonaReaction calls the LLM to produce a single persona's reaction.
func generatePersonaReaction(ctx context.Context, client *llm.Client, topic string, persona models.SocialPersona, verdictOrSynthesis string) (*models.SocialReaction, error) {
	prompt := fmt.Sprintf(`You are simulating: %s — a %s.
Bias: %s. Knowledge level: %s.

The topic of the expert debate was: %s

The expert debate concluded with:
%s

Respond as this persona would on social media. Be authentic to their perspective.
Output JSON:
{
  "reaction": "their response text (2-3 sentences, social media style)",
  "sentiment": "positive|neutral|negative|mixed",
  "amplify": "what aspect they would share/amplify",
  "distort": "how they might misrepresent or oversimplify the findings",
  "trust_score": 0-100
}`, persona.Name, persona.Persona, persona.Bias, persona.KnowledgeLvl, topic, verdictOrSynthesis)

	messages := []llm.Message{
		{Role: "system", Content: "You are a social media simulation engine. You generate realistic public reactions from diverse personas. Always respond with valid JSON only, no markdown or extra text."},
		{Role: "user", Content: prompt},
	}

	resp, err := client.Complete(ctx, messages, 0.8, 512)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse JSON from response (strip markdown code fences if present)
	cleaned := stripCodeFences(resp)
	var parsed personaReactionJSON
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse persona reaction JSON: %w (raw: %s)", err, truncate(cleaned, 200))
	}

	return &models.SocialReaction{
		PersonaID:   persona.ID,
		PersonaName: persona.Name,
		PersonaType: persona.Persona,
		Reaction:    parsed.Reaction,
		Sentiment:   parsed.Sentiment,
		Amplify:     parsed.Amplify,
		Distort:     parsed.Distort,
		TrustScore:  parsed.TrustScore,
	}, nil
}

// generateNarrativeAnalysis calls the LLM to analyze the overall narrative landscape.
func generateNarrativeAnalysis(ctx context.Context, client *llm.Client, reactions []models.SocialReaction) (*narrativeAnalysisJSON, error) {
	var sb strings.Builder
	for i, r := range reactions {
		sb.WriteString(fmt.Sprintf("%d. %s (%s, bias: N/A): %q [sentiment: %s, trust: %d/100]\n",
			i+1, r.PersonaName, r.PersonaType, r.Reaction, r.Sentiment, r.TrustScore))
	}

	prompt := fmt.Sprintf(`Given these %d public reactions to an expert debate:
%s

Analyze the narrative landscape:
{
  "narrative_summary": "overall public reaction summary",
  "dominant_narrative": "the main narrative that will spread",
  "risk_areas": ["where misinformation might spread", ...],
  "overall_sentiment": "positive|neutral|negative|mixed"
}`, len(reactions), sb.String())

	messages := []llm.Message{
		{Role: "system", Content: "You are a narrative analysis engine. You analyze public reactions and identify narrative patterns, misinformation risks, and overall sentiment. Always respond with valid JSON only, no markdown or extra text."},
		{Role: "user", Content: prompt},
	}

	resp, err := client.Complete(ctx, messages, 0.3, 1024)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	cleaned := stripCodeFences(resp)
	var parsed narrativeAnalysisJSON
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse narrative analysis JSON: %w (raw: %s)", err, truncate(cleaned, 200))
	}

	return &parsed, nil
}

// stripCodeFences removes markdown code fences from LLM output.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	// Remove ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		// Remove first line (```json or ```)
		lines = lines[1:]
		// Remove last line if it's ```
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			lines = lines[:len(lines)-1]
		}
		s = strings.Join(lines, "\n")
	}
	return strings.TrimSpace(s)
}

// truncate shortens a string to max n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
