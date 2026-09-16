package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

// PanelSuggestion is the structured output from the topic analyzer LLM call.
type PanelSuggestion struct {
	CoreDomains             []string         `json:"core_domains"`
	AdjacentDomains         []string         `json:"adjacent_domains"`
	PractitionerPerspective string           `json:"practitioner_perspective"`
	LaypersonAngle          string           `json:"layperson_angle"`
	PotentialBiases         []string         `json:"potential_biases"`
	SuggestedPanel          []SuggestedAgent `json:"suggested_panel"`
}

// DiscussionSuggestion is the structured output for discussion-mode topics.
type DiscussionSuggestion struct {
	TopicType    string `json:"topic_type"` // "exploratory", "causal", "comparative"
	Overview     string `json:"overview"`
	Perspectives []DiscussionPerspective    `json:"perspectives"`
	KeyQuestions []string                   `json:"key_questions"`
}

// DiscussionPerspective is one angle/perspective for a discussion.
type DiscussionPerspective struct {
	Angle       string `json:"angle"`
	Description string `json:"description"`
	Expertise   string `json:"expertise"`
	Type        string `json:"type"` // "perspective", "practitioner", "layperson"
}

// SuggestedAgent represents one agent proposed by the analyzer.
type SuggestedAgent struct {
	Role      string `json:"role"`
	Expertise string `json:"expertise"`
	Type      string `json:"type"`
}

const analyzerPromptTmpl = `You are a debate panel designer. Given a debate topic, decompose it into its core domains,
adjacent domains, practitioner perspectives, and layperson angles. Then suggest a balanced panel.

Debate topic: {{.Topic}}

Return ONLY valid JSON with this exact structure:
{
  "core_domains": ["string", ...],
  "adjacent_domains": ["string", ...],
  "practitioner_perspective": "string describing hands-on role",
  "layperson_angle": "string describing common-sense perspective",
  "potential_biases": ["string describing blind spots experts might have", ...],
  "suggested_panel": [
    {
      "role": "Short role title",
      "expertise": "Specific knowledge areas relevant to this topic",
      "type": "core|adjacent|practitioner|layperson"
    }
  ]
}

Rules:
- Suggest exactly {{.PanelSize}} agents total ({{.CoreCount}} core, {{.AdjacentCount}} adjacent, {{.PractitionerCount}} practitioner, {{.LayCount}} layperson)
- Core agents must have deep expertise in the primary domains
- Adjacent agents bring cross-domain perspective
- Practitioner has hands-on real-world experience with the subject
- Layperson represents common sense and everyday observation
- No two agents should share the same narrow specialty
- Roles must be specific to THIS topic — never generic like "Expert 1"`

const discussionPromptTmpl = `You are a discussion panel designer. Given a discussion topic, identify the key perspectives
and angles needed to fully explore it. This is NOT a debate — there are no sides. The goal is to
understand the topic from multiple angles.

Discussion topic: {{.Topic}}

Return ONLY valid JSON with this exact structure:
{
  "topic_type": "exploratory|causal|comparative",
  "overview": "Brief description of what this discussion is about",
  "perspectives": [
    {
      "angle": "Short name for this perspective angle",
      "description": "What this perspective explores and why it matters",
      "expertise": "Specific knowledge areas this perspective needs",
      "type": "perspective|practitioner|layperson"
    }
  ],
  "key_questions": [
    "Important questions this discussion should answer"
  ]
}

Rules:
- Suggest exactly {{.PanelSize}} perspectives total
- At least {{.PerspectiveCount}} different angle perspectives (each covering a different dimension)
- {{.PractitionerCount}} practitioner perspective (someone with hands-on experience)
- {{.LayCount}} layperson perspective (everyday person affected by this topic)
- Each perspective should explore a DIFFERENT dimension — no overlap
- Perspectives should be specific to THIS topic — never generic
- Key questions should be things the panel can actually explore with research`

// Analyzer decomposes a topic into domains and suggests a panel.
type Analyzer struct {
	client *llm.Client
	cfg    *config.Config
}

// NewAnalyzer creates a topic analyzer.
func NewAnalyzer(client *llm.Client, cfg *config.Config) *Analyzer {
	return &Analyzer{client: client, cfg: cfg}
}

// IsDiscussionTopic detects if a topic is discussion-style rather than debate-style.
func IsDiscussionTopic(topic string) bool {
	lower := strings.ToLower(strings.TrimSpace(topic))
	// Questions starting with why, how, what causes, etc.
	discussionStarters := []string{
		"why ", "how ", "what makes", "what causes", "what drives",
		"what factors", "what are the reasons", "what influences",
		"what is the impact", "what effect", "to what extent",
		"is it true that", "can you explain",
	}
	for _, starter := range discussionStarters {
		if strings.HasPrefix(lower, starter) {
			return true
		}
	}
	// Topics without "vs", "versus", "or", "better" are likely discussions
	debateIndicators := []string{" vs ", " versus ", " or ", " better ", " worse ",
		"debate", "argue", "pro", "con", "agree", "disagree"}
	hasDebateIndicator := false
	for _, ind := range debateIndicators {
		if strings.Contains(lower, ind) {
			hasDebateIndicator = true
			break
		}
	}
	if !hasDebateIndicator && strings.Contains(lower, "?") {
		return true
	}
	return false
}

// Analyze decomposes the given topic and returns a structured panel suggestion.
func (a *Analyzer) Analyze(ctx context.Context, topic string) (*PanelSuggestion, error) {
	tmpl, err := template.New("analyzer").Parse(analyzerPromptTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	data := map[string]interface{}{
		"Topic":             topic,
		"PanelSize":         a.cfg.Debate.Panel.TotalAgents,
		"CoreCount":         a.cfg.Debate.Panel.CoreCount,
		"AdjacentCount":     a.cfg.Debate.Panel.AdjacentCount,
		"PractitionerCount": a.cfg.Debate.Panel.PractitionerCount,
		"LayCount":          a.cfg.Debate.Panel.LayCount,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a debate panel designer. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	rawJSON, err := a.client.Complete(ctx, messages, 0.3, 16384)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	rawJSON = cleanJSON(rawJSON)

	var suggestion PanelSuggestion
	if err := json.Unmarshal([]byte(rawJSON), &suggestion); err != nil {
		retryMsg := []llm.Message{
			{Role: "user", Content: buf.String() + "\n\nYou MUST return ONLY valid JSON. Return ONLY the JSON object, no markdown fences, no explanation."},
		}
		rawJSON2, err2 := a.client.Complete(ctx, retryMsg, 0.1, 16384)
		if err2 != nil {
			return nil, fmt.Errorf("llm retry failed: %w (original: %w)", err2, err)
		}
		rawJSON2 = cleanJSON(rawJSON2)
		if err := json.Unmarshal([]byte(rawJSON2), &suggestion); err != nil {
			return nil, fmt.Errorf("unmarshal retry: %w (raw: %s)", err, rawJSON2[:min(len(rawJSON2), 200)])
		}
	}

	return &suggestion, nil
}

// AnalyzeDiscussion analyzes a discussion topic and returns perspective suggestions.
func (a *Analyzer) AnalyzeDiscussion(ctx context.Context, topic string) (*DiscussionSuggestion, error) {
	tmpl, err := template.New("discussion").Parse(discussionPromptTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	totalAgents := a.cfg.Debate.Panel.TotalAgents
	data := map[string]interface{}{
		"Topic":             topic,
		"PanelSize":         totalAgents,
		"PerspectiveCount":  totalAgents - a.cfg.Debate.Panel.PractitionerCount - a.cfg.Debate.Panel.LayCount,
		"PractitionerCount": a.cfg.Debate.Panel.PractitionerCount,
		"LayCount":          a.cfg.Debate.Panel.LayCount,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a discussion panel designer. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	rawJSON, err := a.client.Complete(ctx, messages, 0.3, 16384)
	if err != nil {
		return nil, fmt.Errorf("llm complete: %w", err)
	}

	rawJSON = cleanJSON(rawJSON)

	var suggestion DiscussionSuggestion
	if err := json.Unmarshal([]byte(rawJSON), &suggestion); err != nil {
		retryMsg := []llm.Message{
			{Role: "user", Content: buf.String() + "\n\nYou MUST return ONLY valid JSON. Return ONLY the JSON object."},
		}
		rawJSON2, err2 := a.client.Complete(ctx, retryMsg, 0.1, 16384)
		if err2 != nil {
			return nil, fmt.Errorf("llm retry failed: %w (original: %w)", err2, err)
		}
		rawJSON2 = cleanJSON(rawJSON2)
		if err := json.Unmarshal([]byte(rawJSON2), &suggestion); err != nil {
			return nil, fmt.Errorf("unmarshal retry: %w (raw: %s)", err, rawJSON2[:min(len(rawJSON2), 200)])
		}
	}

	return &suggestion, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// HeuristicComplexity provides a fast, zero-cost complexity estimate without an LLM call.
// Returns nil if the topic is ambiguous and an LLM assessment is recommended.
func HeuristicComplexity(topic string) *models.DebateComplexity {
	lower := strings.ToLower(strings.TrimSpace(topic))

	// Very short topics with no question words → likely quick
	questionWords := []string{"why", "how", "what", "should", "is", "are", "can", "will", "does"}
	hasQuestionWord := false
	for _, w := range questionWords {
		if strings.HasPrefix(lower, w+" ") {
			hasQuestionWord = true
			break
		}
	}

	if len(lower) < 20 && !hasQuestionWord {
		return &models.DebateComplexity{
			Score:           "simple",
			RecommendedMode: "quick",
			Reasons:         []string{"Short, straightforward topic"},
			ControversyLevel: 10,
			DomainCount:     1,
		}
	}

	// Strong indicators for deep/complex topics
	deepIndicators := []string{
		"vs", "versus", "debate", "controversial", "controversy",
		"ethical", "morality", "should we", "is it right",
		"climate change", "abortion", "immigration", "gun control",
		"artificial intelligence", "geopolitics",
	}
	deepScore := 0
	var deepReasons []string
	for _, indicator := range deepIndicators {
		if strings.Contains(lower, indicator) {
			deepScore++
			deepReasons = append(deepReasons, fmt.Sprintf("Contains indicator: %s", indicator))
		}
	}

	if deepScore >= 2 {
		return &models.DebateComplexity{
			Score:           "highly_complex",
			RecommendedMode: "deep",
			Reasons:         deepReasons,
			ControversyLevel: 80,
			DomainCount:     3,
		}
	}

	if deepScore == 1 {
		return &models.DebateComplexity{
			Score:           "complex",
			RecommendedMode: "deep",
			Reasons:         deepReasons,
			ControversyLevel: 60,
			DomainCount:     2,
		}
	}

	// Discussion-style topics (from IsDiscussionTopic logic)
	if IsDiscussionTopic(topic) {
		return &models.DebateComplexity{
			Score:           "moderate",
			RecommendedMode: "discussion",
			Reasons:         []string{"Exploratory topic better suited to discussion"},
			ControversyLevel: 30,
			DomainCount:     2,
		}
	}

	// Ambiguous — return nil to signal that LLM assessment is needed
	return nil
}
