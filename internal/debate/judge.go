package debate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/ws"
)

// Judge handles final verdict evaluation.
type Judge struct {
	client *llm.Client
	cfg    *config.Config
}

// NewJudge creates a judge module.
func NewJudge(client *llm.Client, cfg *config.Config) *Judge {
	return &Judge{client: client, cfg: cfg}
}

// Evaluate scores all arguments and broadcasts the verdict.
func (j *Judge) Evaluate(ctx context.Context, session *models.DebateSession, hub *ws.Hub) error {
	transcript := session.GetTranscript()

	// Build judge prompt with knowledge graph context
	tmpl, err := template.New("judge").Parse(judgePromptTemplate)
	if err != nil {
		return fmt.Errorf("parse judge template: %w", err)
	}

	// Prepare graph data for the judge
	graphNodes := session.Graph.Nodes
	graphEdges := session.Graph.Edges

	// Build claims list from graph
	var claimsList []map[string]interface{}
	for _, n := range graphNodes {
		if n.Type == "claim" {
			claimsList = append(claimsList, map[string]interface{}{
				"label":     n.Label,
				"content":   n.Content,
				"agent":     n.AgentName,
				"round":     n.Round,
			})
		}
	}

	// Build evidence list from graph
	var evidenceList []map[string]interface{}
	for _, n := range graphNodes {
		if n.Type == "evidence" && n.SourceURL != "" {
			evidenceList = append(evidenceList, map[string]interface{}{
				"label": n.Label,
				"source": n.SourceURL,
				"agent": n.AgentName,
			})
		}
	}

	// Build contradiction pairs from graph
	var contradictions []map[string]string
	for _, e := range graphEdges {
		if e.Relation == "contradicts" {
			fromNode := findNode(graphNodes, e.From)
			toNode := findNode(graphNodes, e.To)
			if fromNode != nil && toNode != nil {
				contradictions = append(contradictions, map[string]string{
					"from":      fromNode.Label,
					"from_agent": fromNode.AgentName,
					"to":        toNode.Label,
					"to_agent":  toNode.AgentName,
				})
			}
		}
	}

	data := map[string]interface{}{
		"Topic":           session.Topic,
		"EvidenceWeight":  j.cfg.Debate.Scoring.EvidenceWeight * 100,
		"LogicWeight":     j.cfg.Debate.Scoring.LogicWeight * 100,
		"NoveltyWeight":   j.cfg.Debate.Scoring.NoveltyWeight * 100,
		"Transcript":      transcript,
		"Claims":          claimsList,
		"Evidence":        evidenceList,
		"Contradictions":  contradictions,
		"HasGraph":        len(graphNodes) > 0,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute judge template: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a neutral debate judge. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	rawJSON, err := j.client.Complete(ctx, messages, 0.3, 32768)
	if err != nil {
		return fmt.Errorf("judge llm call: %w", err)
	}

	// Clean response
	rawJSON = strings.TrimSpace(rawJSON)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var verdict models.Verdict
	if err := json.Unmarshal([]byte(rawJSON), &verdict); err != nil {
		log.Printf("JUDGE JSON PARSE ERROR [%s]: %v\nRaw: %s", session.ID, err, rawJSON[:min(len(rawJSON), 500)])
		// Fallback: simple winner selection
		verdict = j.fallbackVerdict(session)
	}

	session.SetVerdict(&verdict)
	session.SetStatus(models.StatusVerdict)

	hub.Broadcast(session.ID, ws.Event{
		Event:    "verdict",
		DebateID: session.ID,
		Data:     verdict,
	})

	return nil
}

func findNode(nodes []models.GraphNode, id string) *models.GraphNode {
	for _, n := range nodes {
		if n.ID == id {
			return &n
		}
	}
	return nil
}

// Synthesize produces a discussion synthesis — no winner, just insights.
func (j *Judge) Synthesize(ctx context.Context, session *models.DebateSession, hub *ws.Hub) error {
	transcript := session.GetTranscript()

	tmpl, err := template.New("synthesis").Parse(synthesisPromptTemplate)
	if err != nil {
		return fmt.Errorf("parse synthesis template: %w", err)
	}

	// Prepare graph data
	graphNodes := session.Graph.Nodes
	var claimsList []map[string]interface{}
	for _, n := range graphNodes {
		if n.Type == "claim" {
			claimsList = append(claimsList, map[string]interface{}{
				"label": n.Label, "content": n.Content, "agent": n.AgentName,
			})
		}
	}

	data := map[string]interface{}{
		"Topic":      session.Topic,
		"Transcript": transcript,
		"Claims":     claimsList,
		"HasGraph":   len(graphNodes) > 0,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute synthesis template: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a discussion synthesizer. Return ONLY valid JSON."},
		{Role: "user", Content: buf.String()},
	}

	rawJSON, err := j.client.Complete(ctx, messages, 0.3, 32768)
	if err != nil {
		return fmt.Errorf("synthesis llm call: %w", err)
	}

	rawJSON = strings.TrimSpace(rawJSON)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var synthesis models.Synthesis
	if err := json.Unmarshal([]byte(rawJSON), &synthesis); err != nil {
		log.Printf("SYNTHESIS JSON PARSE ERROR [%s]: %v\nRaw: %s", session.ID, err, rawJSON[:min(len(rawJSON), 500)])
		synthesis = models.Synthesis{
			Overview: "The synthesizer was unable to complete the analysis. Please review the discussion transcript.",
		}
	}

	session.SetSynthesis(&synthesis)
	session.SetStatus(models.StatusVerdict)

	// Broadcast as a verdict event but with synthesis data
	hub.Broadcast(session.ID, ws.Event{
		Event:    "synthesis",
		DebateID: session.ID,
		Data:     synthesis,
	})

	return nil
}

// fallbackVerdict creates a simple verdict if the LLM fails.
func (j *Judge) fallbackVerdict(session *models.DebateSession) models.Verdict {
	agents := session.Agents
	if len(agents) == 0 {
		return models.Verdict{Reasoning: "Unable to determine winner."}
	}

	winner := agents[0]
	for _, a := range agents {
		if a.RoleType != models.RoleJudge {
			winner = a
			break
		}
	}

	return models.Verdict{
		WinnerAgentID: winner.ID,
		Scorecard: []models.AgentScore{
			{AgentID: winner.ID, AgentName: winner.Name, Evidence: 5.0, Logic: 5.0, Novelty: 5.0, Total: 5.0},
		},
		Reasoning: "The judge was unable to fully evaluate this debate. Please review the transcript manually.",
	}
}

const judgePromptTemplate = `You are the Debate Judge. You are neutral, analytical, and fair.

Debate topic: "{{.Topic}}"

Your task has TWO parts:
1. Score each speaker
2. Consolidate the debate — synthesize what was established, contested, and unresolved

## PART 1: Score each speaker (0.0–10.0 scale)
- Evidence Quality ({{.EvidenceWeight}}%): Did they cite specific sources/data? Were claims verifiable?
- Logical Coherence ({{.LogicWeight}}%): Was their reasoning internally consistent? Any fallacies?
- Novelty ({{.NoveltyWeight}}%): Did they contribute new ideas, or merely rehash others' points?

## PART 2: Consolidate the debate
Using the knowledge graph claims and evidence below, synthesize the debate's findings:
- "consolidated_claims": For each major claim debated, state whether it is "established" (strong evidence, no effective counter), "contested" (evidence on both sides), "disputed" (strong counter-evidence exists), or "unresolved" (insufficient evidence)
- "consensus_points": Things all or most speakers agreed on
- "unresolved_questions": Important questions the debate raised but did not answer
- "synthesis": A 3-5 paragraph narrative that consolidates the entire debate — what did we learn, what is now clearer, what remains uncertain

## DEBATE TRANSCRIPT:
{{range .Transcript}}
---
[{{.AgentName}} ({{.AgentRole}}), Round {{.Round}}, Phase: {{.Phase}}]:
{{.Text}}
{{if .SearchQuery}}Research query: {{.SearchQuery}}{{end}}
{{end}}

{{if .HasGraph}}
## KNOWLEDGE GRAPH — Claims extracted during debate:
{{range .Claims}}
- [{{.agent}}, Round {{.round}}]: {{.label}} — {{.content}}
{{end}}

## KNOWLEDGE GRAPH — Evidence cited:
{{range .Evidence}}
- [{{.agent}}]: {{.label}} (source: {{.source}})
{{end}}

{{if .Contradictions}}
## KNOWLEDGE GRAPH — Direct contradictions found:
{{range .Contradictions}}
- {{.from_agent}}: "{{.from}}" CONTRADICTS {{.to_agent}}: "{{.to}}"
{{end}}
{{end}}
{{end}}

CONFIDENCE-WEIGHTED SCORING: Each agent's argument may include a confidence score [Confidence: XX/100].
When scoring, consider the agent's confidence calibration:
- If an agent provides strong evidence AND high confidence → reward
- If an agent provides weak evidence BUT high confidence → penalize (overconfidence)
- If an agent provides strong evidence BUT low confidence → note as underconfident
Include in your verdict a "calibration_notes" field describing each agent's confidence accuracy.

Return ONLY valid JSON:
{
  "winner_agent_id": "id of highest-scoring speaker",
  "scorecard": [
    {
      "agent_id": "...",
      "agent_name": "...",
      "evidence": 8.5,
      "logic": 7.0,
      "novelty": 9.0,
      "total": 8.075
    }
  ],
  "turning_points": [
    "Describe a specific moment where the debate shifted"
  ],
  "strongest_evidence": {
    "agent_id": "...",
    "claim": "The single most well-supported factual claim in the debate",
    "why": "Why this evidence was so compelling"
  },
  "consolidated_claims": [
    {
      "claim": "The claim being evaluated",
      "status": "established|contested|disputed|unresolved",
      "support": ["Agent Name 1", "Agent Name 2"],
      "oppose": ["Agent Name 3"],
      "evidence": "Brief summary of evidence quality for this claim"
    }
  ],
  "consensus_points": [
    "Things that most or all speakers agreed on"
  ],
  "unresolved_questions": [
    "Important questions the debate raised but did not settle"
  ],
  "synthesis": "3-5 paragraph narrative consolidating the entire debate: what did we learn, what is now clearer, what remains uncertain, and what would resolve the remaining questions",
  "calibration_notes": "Per-agent notes on confidence accuracy: e.g. 'Agent X was overconfident (claimed 90 but weak evidence)', 'Agent Y was underconfident (claimed 50 but strong evidence)'",
  "reasoning": "2-3 paragraph narrative explaining the verdict and scoring"
}`

const synthesisPromptTemplate = `You are the Discussion Synthesizer. You have listened to a multi-perspective discussion.

Discussion topic: "{{.Topic}}"

IMPORTANT LANGUAGE RULE: You MUST write your synthesis in the SAME LANGUAGE as the discussion topic above. If the topic is in Thai, write entirely in Thai. Do not mix languages.

Your task: synthesize the discussion into key insights. There is NO winner — this is about understanding.

## DISCUSSION TRANSCRIPT:
{{range .Transcript}}
---
[{{.AgentName}} ({{.AgentRole}}), Round {{.Round}}, Phase: {{.Phase}}]:
{{.Text}}
{{end}}

{{if .HasGraph}}
## KEY CLAIMS from the knowledge graph:
{{range .Claims}}
- [{{.agent}}]: {{.label}} — {{.content}}
{{end}}
{{end}}

Return ONLY valid JSON:
{
  "key_insights": [
    {
      "theme": "Short theme name",
      "finding": "What was discovered about this theme",
      "support_by": ["Agent Name 1", "Agent Name 2"],
      "evidence": "Quality of evidence supporting this finding"
    }
  ],
  "common_themes": [
    "Things that multiple perspectives identified as important"
  ],
  "surprising_findings": [
    "Unexpected discoveries or counter-intuitive findings from the discussion"
  ],
  "open_questions": [
    "Questions that remain unanswered and need further exploration"
  ],
  "overview": "3-5 paragraph narrative: What did we learn? What is the full picture? What would you tell someone who knows nothing about this topic? What deserves further research?"
}`
