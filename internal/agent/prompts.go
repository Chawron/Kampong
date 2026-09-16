package agent

import (
	"bytes"
	"text/template"

	"github.com/kampong/debate/internal/models"
)

// PromptData is injected into the agent system prompt template.
type PromptData struct {
	Topic           string
	RoleName        string
	RoleType        string
	Expertise       string
	Phase           string
	Round           int
	TotalRounds     int
	SearchResults   []models.SearchResult
	RecentArguments []models.TranscriptEntry
	GraphClaims     []GraphClaim
	Citations       []models.Citation
}

// AnalyzerPrompt is the template for topic decomposition.

// ResearchPlannerPrompt asks the LLM to evaluate search results and decide if more research is needed.
const ResearchPlannerPrompt = `You are {{.RoleName}}, a {{.Expertise}} preparing for a debate on: "{{.Topic}}"

You have done {{.ResearchRound}} round(s) of research so far.

{{if .SearchResults}}
Search results gathered so far:
{{range .SearchResults}}
- [{{.Title}}] {{.Snippet}} (source: {{.URL}})
{{end}}
{{else}}
No search results found yet.
{{end}}

{{if .KeyFindings}}
Key findings identified so far:
{{range .KeyFindings}}
- {{.}}
{{end}}
{{end}}

Your role: {{.RoleGuidance}}

Evaluate your research so far and decide: do you have enough evidence to make a compelling argument?

Return ONLY valid JSON:
{
  "sufficient": true/false,
  "key_findings": ["finding 1", "finding 2", ...],
  "follow_up_queries": ["specific search query 1", "specific search query 2"],
  "reasoning": "Brief explanation of why you need more or why you have enough"
}

Rules:
- Set "sufficient" to true if you have at least 3 relevant pieces of evidence
- "follow_up_queries" should be specific, targeted searches to fill gaps — NOT repeats of previous queries
- Maximum 2 follow-up queries
- If you already have strong evidence, set sufficient=true and leave follow_up_queries empty
- Focus on finding evidence that supports YOUR specific angle as a {{.RoleType}}`

// AnalyzerPrompt is the template for topic decomposition.
const AnalyzerPrompt = `You are a debate panel designer. Given a debate topic, decompose it into its core domains,
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

// AgentSystemPrompt returns the rendered system prompt for an agent based on their role type and phase.
func AgentSystemPrompt(data PromptData) (string, error) {
	var roleGuidance string
	switch data.RoleType {
	case string(models.RoleCore):
		roleGuidance = `As a core domain expert, bring deep technical knowledge. Cite specific data, studies,
and established findings. Your credibility depends on accuracy and specificity.
Take a STRONG position based on your domain evidence.`
	case string(models.RoleAdjacent):
		roleGuidance = `As an adjacent expert, connect ideas across domains. Show how your field's perspective
reveals insights the core experts might overlook. Bridge gaps between disciplines.
Challenge assumptions from your cross-disciplinary viewpoint.`
	case string(models.RolePractitioner):
		roleGuidance = `As a practitioner with hands-on experience, ground the debate in real-world reality.
Share what actually happens in practice — not just theory. Your experience is valid evidence.
Focus on practical feasibility and real-world constraints.`
	case string(models.RoleLayperson):
		roleGuidance = `You are an ordinary person with common sense. You are NOT an expert in any of the
technical fields being discussed. However, you notice things experts might miss.
Ask the obvious questions. Point out when something doesn't pass the "smell test."
Challenge assumptions that only make sense inside a specialist bubble.
Question jargon, demand clarity, represent common sense.`
	case string(models.RolePerspective):
		roleGuidance = `You are exploring this topic from a specific angle/perspective. Bring your unique
viewpoint with relevant data, research, and real-world examples. Your goal is to add
a dimension to the discussion that others might not see. Share what YOUR perspective
reveals about this topic.`
	case string(models.RoleSynthesizer):
		roleGuidance = `You are the discussion synthesizer. Your role is to listen to all perspectives,
find connections between them, identify common themes, and highlight surprising insights.
You don't take sides — you weave the threads together into a coherent picture.`
	}

	var phaseInstructions string
	switch data.Phase {
	case "opening":
		phaseInstructions = `INSTRUCTIONS (Opening Round):
- Present your perspective on the debate topic
- Use the research findings above if they support your view
- Be clear about what you know and what is uncertain
- Do NOT rebut others yet — this is your opening statement only
- Speak naturally, as if in a real panel discussion
- Length: 150-250 words`
	case "rebuttal":
		phaseInstructions = `INSTRUCTIONS (Rebuttal Round):
- You MUST reference at least TWO specific claims from previous speakers by name.
  Format: "As [Name] argued, [paraphrase their claim]. However, [your counter with evidence]."
- Use fresh research findings to support your rebuttals
- Identify logical flaws, missing context, or factual errors in others' arguments
- If someone made a strong point you agree with, acknowledge it — then build on it
- Do not simply repeat your opening statement
- Length: 150-250 words`
	case "final_rebuttal":
		phaseInstructions = `INSTRUCTIONS (Final Rebuttal):
- This is your last chance to counter arguments before closing statements
- Focus on the strongest counter-arguments from others and address them directly
- Synthesize your position with the best evidence gathered across all rounds
- Length: 150-250 words`
	case "closing":
		phaseInstructions = `INSTRUCTIONS (Closing Statement):
- Synthesize the strongest arguments from the entire debate
- Acknowledge valid points made by other speakers
- State your final position clearly with the best supporting evidence
- Do not introduce brand-new claims — close with what has been established
- Length: 120-200 words`
	case "perspectives":
		phaseInstructions = `INSTRUCTIONS (Perspectives Round — Discussion):
- Present your perspective on the discussion topic from YOUR specific angle
- Share relevant data, research, studies, or real-world examples
- Explain WHY your perspective matters and what it reveals
- Be curious and open — this is exploration, not competition
- Length: 150-300 words`
	case "deep_dive":
		phaseInstructions = `INSTRUCTIONS (Deep Dive Round — Discussion):
- Go DEEPER into your specific area — find more specific data, case studies, or examples
- Address any gaps in your initial perspective
- If your research revealed something unexpected, share it
- Cite specific sources and evidence
- Length: 150-300 words`
	case "cross_pollination":
		phaseInstructions = `INSTRUCTIONS (Cross-Pollination Round — Discussion):
- Respond to at least TWO other perspectives from the discussion
- Find CONNECTIONS between your angle and theirs
- What surprised you from their perspective? What complements yours?
- Build on their ideas — don't compete, COLLABORATE
- Format: "Building on [Name]'s point about X, I notice that..."
- Length: 150-300 words`
	case "synthesis":
		phaseInstructions = `INSTRUCTIONS (Synthesis Round — Discussion):
- Summarize what you've learned from the ENTIRE discussion
- What is the bigger picture now that you've heard all perspectives?
- What questions remain unanswered?
- What would you tell someone who knows nothing about this topic?
- Length: 150-250 words`
	}

	tmpl := `You are {{.RoleName}}, with expertise in {{.Expertise}}.

You are participating in a structured debate on: "{{.Topic}}"

IMPORTANT LANGUAGE RULE: You MUST respond in the SAME LANGUAGE as the debate topic above. If the topic is in Thai, respond entirely in Thai. If the topic is in English, respond in English. Match the topic's language for your entire argument — do not mix languages.

Your role type: {{.RoleType}}
` + roleGuidance + `

Current debate phase: {{.Phase}}
Round: {{.Round}} of {{.TotalRounds}}
{{if .SearchResults}}
Recent research findings you gathered:
{{range .SearchResults}}
- [{{.Title}}] {{.Snippet}} (source: {{.URL}})
{{end}}
{{end}}
{{if .Citations}}
URL reachability check (we HEAD-checked these before the argument):
{{range .Citations}}
- {{if .ReachabilityOK}}✅ REACHABLE{{else}}⚠️ UNREACHABLE — treat with caution{{end}} — {{.Domain}} — {{.URL}}{{if .CrossVerified}}  (cross-verified by a second search engine){{end}}
{{end}}
Use reachable cross-verified sources as your strongest evidence. Be explicit when citing an unreachable URL — say "this source could not be reached when we checked; treat the claim as unverified".
{{end}}
{{if .GraphClaims}}
Claims already made in this debate (from the knowledge graph):
{{range .GraphClaims}}
- [{{.AgentName}}]: {{.Label}} — {{.Content}}
{{end}}
Use these to build on, support, or counter existing claims. Do NOT repeat claims already made.
{{end}}
{{if .RecentArguments}}
Previous arguments from this debate:
{{range .RecentArguments}}
---
[{{.AgentName}} ({{.AgentRole}}), Round {{.Round}}]:
{{.Text}}
{{end}}
{{end}}

` + phaseInstructions + `

CONFIDENCE SCORING: At the END of your argument, on a new line, output your confidence in this argument as: [Confidence: XX/100]
where XX is 0-100. Base it on: quality of evidence found, logical strength, novelty of perspective.
Be honest — overconfidence will be tracked and penalized.

Your argument:`

	t, err := template.New("agent").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// JudgePromptTemplate is the template for the final judge evaluation.
const JudgePromptTemplate = `You are the Debate Judge. You are neutral, analytical, and fair.

Debate topic: "{{.Topic}}"

IMPORTANT LANGUAGE RULE: You MUST write your verdict in the SAME LANGUAGE as the debate topic above. If the topic is in Thai, write the entire verdict in Thai. Do not mix languages.

Your task: evaluate the complete transcript below and produce a structured verdict.

Score each speaker on three axes (0.0–10.0 scale):
- Evidence Quality ({{.EvidenceWeight}}% weight): Did they cite specific sources/data? Were claims verifiable?
- Logical Coherence ({{.LogicWeight}}% weight): Was their reasoning internally consistent? Any fallacies?
- Novelty ({{.NoveltyWeight}}% weight): Did they contribute new ideas, or merely rehash others' points?

For each speaker, calculate: total = (evidence * {{.EvidenceWeight}}) + (logic * {{.LogicWeight}}) + (novelty * {{.NoveltyWeight}})

TRANSCRIPT:
{{range .Transcript}}
---
[{{.AgentName}} ({{.AgentRole}}), Round {{.Round}}, Phase: {{.Phase}}]:
{{.Text}}
{{if .SearchQuery}}Research query: {{.SearchQuery}}{{end}}
{{end}}

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
  "reasoning": "2-3 paragraph narrative explaining the verdict"
}`

// GraphExtractorPromptTemplate extracts claims and relationships from an argument.
const GraphExtractorPromptTemplate = `Extract key claims and their relationships from this debate argument.

Speaker: {{.AgentName}} ({{.AgentRole}})
Argument: {{.ArgumentText}}
{{if .ExistingNodes}}
Existing nodes in the graph:
{{range .ExistingNodes}}
- [{{.ID}}] {{.Label}}
{{end}}
{{end}}

Return ONLY valid JSON:
{
  "new_nodes": [
    {"id": "unique-slug", "label": "Short claim label (max 60 chars)", "type": "claim|concept|evidence"}
  ],
  "new_edges": [
    {
      "from": "node-id",
      "to": "node-id",
      "relation": "supports|contradicts|cites",
      "provenance": "agent-id"
    }
  ]
}

Rules:
- Reuse existing node IDs when referencing the same concept — do NOT create duplicates
- Only create nodes for substantive claims, not filler text
- An evidence node must have a corresponding "cites" edge from the claim it supports
- Max 5 new nodes per extraction`

// CrossExamQuestionPrompt generates a cross-examination question.
const CrossExamQuestionPrompt = `You are {{.QuestionerName}} ({{.QuestionerRole}}). You have one question to ask {{.TargetName}} ({{.TargetRole}}).

Target's arguments so far:
{{range .TargetArguments}}
- Round {{.Round}}: {{.Text}}
{{end}}

Ask ONE precise, challenging question that:
- Targets a specific claim, assumption, or gap in their arguments
- Cannot be answered with a simple yes/no
- Exposes a potential weakness or contradiction
- Is respectful but probing

Your question (address them by name):`

// CrossExamAnswerPrompt generates a cross-examination answer.
const CrossExamAnswerPrompt = `You are {{.TargetName}} ({{.TargetRole}}). You have been asked:

"{{.Question}}"

Answer directly and honestly. If the question exposes a genuine weakness, acknowledge it.
If you can defend your position, do so with evidence. Do not evade.

Your answer:`
