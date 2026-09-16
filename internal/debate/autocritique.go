package debate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

const autoCritiquePromptTemplate = `You are an independent debate quality reviewer. Evaluate the quality of this debate.

Topic: "{{.Topic}}"
Agents: {{.AgentCount}}
Rounds: {{.TotalRounds}}

## TRANSCRIPT:
{{range .Transcript}}
---
[{{.AgentName}} ({{.AgentRole}}), Round {{.Round}}]:
{{.Text}}
{{end}}

Evaluate on: research quality, argument depth, diversity of perspectives.
Return ONLY valid JSON:
{
  "debate_quality": "excellent|good|fair|poor",
  "strengths": ["..."],
  "weaknesses": ["..."],
  "missing_perspectives": ["..."],
  "research_quality": 0-100,
  "argument_depth": 0-100,
  "diversity_score": 0-100,
  "recommendations": ["..."]
}`

// RunAutoCritique uses a separate LLM call to evaluate debate quality.
func RunAutoCritique(ctx context.Context, client *llm.Client, session *models.DebateSession) (*models.AutoCritique, error) {
	transcript := session.GetTranscript()
	if len(transcript) == 0 {
		return nil, fmt.Errorf("no transcript to critique")
	}

	tmpl, err := template.New("autocritique").Parse(autoCritiquePromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse autocritique template: %w", err)
	}

	data := map[string]interface{}{
		"Topic":       session.Topic,
		"AgentCount":  len(session.Agents),
		"TotalRounds": session.TotalRounds,
		"Transcript":  transcript,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute autocritique template: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are an independent debate quality reviewer. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	rawJSON, err := client.Complete(ctx, messages, 0.3, 4096)
	if err != nil {
		return nil, fmt.Errorf("autocritique LLM call: %w", err)
	}

	rawJSON = strings.TrimSpace(rawJSON)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var critique models.AutoCritique
	if err := json.Unmarshal([]byte(rawJSON), &critique); err != nil {
		log.Printf("AUTOCRITIQUE PARSE ERROR [%s]: %v", session.ID, err)
		return nil, fmt.Errorf("parse autocritique response: %w", err)
	}

	return &critique, nil
}
