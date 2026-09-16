package debate

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/ws"
)

// CrossExaminer handles the cross-examination phase of deep-mode debates.
type CrossExaminer struct {
	client *llm.Client
}

// NewCrossExaminer creates a cross-examination module.
func NewCrossExaminer(client *llm.Client) *CrossExaminer {
	return &CrossExaminer{client: client}
}

// Run executes the cross-examination phase: each non-judge agent asks one
// question to another agent, who then answers.
func (c *CrossExaminer) Run(ctx context.Context, session *models.DebateSession, hub *ws.Hub) error {
	debaters := c.getDebaters(session.Agents)
	if len(debaters) < 2 {
		return nil // need at least 2 to cross-examine
	}

	for i, questioner := range debaters {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Pick target: next agent in the list (wrapping around)
		target := debaters[(i+1)%len(debaters)]

		// Generate question
		question, err := c.generateQuestion(ctx, questioner, target, session)
		if err != nil {
			log.Printf("CROSS-EXAM QUESTION ERROR: %v", err)
			continue
		}

		// Broadcast question
		qEntry := models.TranscriptEntry{
			AgentID:      questioner.ID,
			AgentName:    questioner.Name,
			AgentRole:    questioner.Role,
			Round:        session.Round,
			Phase:        models.StatusCrossExam,
			QuestionText: question,
			TargetAgentID: target.ID,
		}
		session.AppendTranscript(qEntry)

		hub.Broadcast(session.ID, ws.Event{
			Event:    "agent_done",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":      questioner.ID,
				"full_text":     question,
				"is_question":   true,
				"target_agent_id": target.ID,
				"target_agent_name": target.Name,
				"vendor_family": questioner.ProviderFamily,
				"provider_name": questioner.ProviderName,
			},
		})

		// Generate answer
		answer, err := c.generateAnswer(ctx, target, questioner, question)
		if err != nil {
			log.Printf("CROSS-EXAM ANSWER ERROR: %v", err)
			continue
		}

		// Broadcast answer
		aEntry := models.TranscriptEntry{
			AgentID:   target.ID,
			AgentName: target.Name,
			AgentRole: target.Role,
			Round:     session.Round,
			Phase:     models.StatusCrossExam,
			Text:      answer,
		}
		session.AppendTranscript(aEntry)

		hub.Broadcast(session.ID, ws.Event{
			Event:    "agent_done",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":   target.ID,
				"full_text":  answer,
				"is_answer":  true,
				"questioner_agent_id": questioner.ID,
				"questioner_agent_name": questioner.Name,
				"vendor_family": target.ProviderFamily,
				"provider_name": target.ProviderName,
			},
		})
	}

	return nil
}

func (c *CrossExaminer) generateQuestion(ctx context.Context, questioner, target models.Agent, session *models.DebateSession) (string, error) {
	// Gather target's prior arguments
	var targetArgs []models.TranscriptEntry
	for _, entry := range session.GetTranscript() {
		if entry.AgentID == target.ID {
			targetArgs = append(targetArgs, entry)
		}
	}

	tmpl, err := template.New("question").Parse(questionPromptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"QuestionerName": questioner.Name,
		"QuestionerRole": questioner.Role,
		"TargetName":     target.Name,
		"TargetRole":     target.Role,
		"TargetArguments": targetArgs,
	}); err != nil {
		return "", err
	}

	messages := []llm.Message{
		{Role: "user", Content: buf.String()},
	}

	return c.client.Complete(ctx, messages, 0.7, 4096)
}

func (c *CrossExaminer) generateAnswer(ctx context.Context, target, questioner models.Agent, question string) (string, error) {
	tmpl, err := template.New("answer").Parse(answerPromptTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"TargetName":     target.Name,
		"TargetRole":     target.Role,
		"QuestionerName": questioner.Name,
		"Question":       question,
	}); err != nil {
		return "", err
	}

	// Use target's system prompt for consistency
	messages := []llm.Message{
		{Role: "system", Content: fmt.Sprintf("You are %s, %s. Expertise: %s. Answer truthfully.", target.Name, target.Role, target.Expertise)},
		{Role: "user", Content: buf.String()},
	}

	return c.client.Complete(ctx, messages, 0.7, 4096)
}

func (c *CrossExaminer) getDebaters(agents []models.Agent) []models.Agent {
	var debaters []models.Agent
	for _, a := range agents {
		if a.RoleType != models.RoleJudge {
			debaters = append(debaters, a)
		}
	}
	return debaters
}

const questionPromptTemplate = `You are {{.QuestionerName}} ({{.QuestionerRole}}). You have one question to ask {{.TargetName}} ({{.TargetRole}}).

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

const answerPromptTemplate = `You are {{.TargetName}} ({{.TargetRole}}). You have been asked by {{.QuestionerName}}:

"{{.Question}}"

Answer directly and honestly. If the question exposes a genuine weakness, acknowledge it.
If you can defend your position, do so with evidence. Do not evade.

Your answer:`
