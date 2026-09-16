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

// RunCrossFamilyVerifier picks an out-of-family LLM and asks it to flag weak claims.
//
// Goal: the agents in the debate come from one set of vendor families. The
// verifier comes from a *different* family, so it can spot claims the original
// panel might have rubber-stamped because they "look right" to the same models.
//
// Returns (result, true) when the run actually happened. If no out-of-family
// provider is available, returns a Skipped result and false — the engine logs
// a warning but does not fail the debate.
func RunCrossFamilyVerifier(
	ctx context.Context,
	manager *llm.ClientManager,
	session *models.DebateSession,
) (*models.CrossVerifyResult, bool) {
	if manager == nil || session == nil {
		return &models.CrossVerifyResult{Skipped: true, SkipReason: "no manager / session"}, false
	}
	if session.GetVerdict() == nil {
		return &models.CrossVerifyResult{Skipped: true, SkipReason: "no verdict to verify"}, false
	}

	verifier, err := manager.PickOutOfFamilyProvider(agentFamilySources(session.Agents))
	if err != nil {
		log.Printf("CROSS-VERIFY: skipped — %v", err)
		return &models.CrossVerifyResult{Skipped: true, SkipReason: err.Error()}, false
	}

	prompt, err := renderVerifierPrompt(session)
	if err != nil {
		return &models.CrossVerifyResult{
			Skipped:         true,
			SkipReason:      fmt.Sprintf("render prompt: %v", err),
			VerifierProvider: verifier.Name,
			VerifierFamily:   verifier.ProviderFamily,
		}, false
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are an independent debate auditor from a different model family. Return ONLY valid JSON."},
		{Role: "user", Content: prompt},
	}
	raw, err := verifier.Client.Complete(ctx, messages, 0.3, 4096)
	if err != nil {
		return &models.CrossVerifyResult{
			Skipped:         true,
			SkipReason:      fmt.Sprintf("llm call: %v", err),
			VerifierProvider: verifier.Name,
			VerifierFamily:   verifier.ProviderFamily,
		}, false
	}

	raw = cleanJSONBlock(raw)
	var resp struct {
		WeakClaims []struct {
			Claim string `json:"claim"`
			Why   string `json:"why"`
		} `json:"weak_claims"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("CROSS-VERIFY: parse error: %v\nraw: %.200s", err, raw)
		return &models.CrossVerifyResult{
			Skipped:         true,
			SkipReason:      fmt.Sprintf("parse: %v", err),
			VerifierProvider: verifier.Name,
			VerifierFamily:   verifier.ProviderFamily,
		}, false
	}

	result := &models.CrossVerifyResult{
		VerifierProvider: verifier.Name,
		VerifierFamily:   verifier.ProviderFamily,
		Summary:          resp.Summary,
	}
	for _, w := range resp.WeakClaims {
		families := familiesSupportingClaim(session, w.Claim)
		result.WeakClaims = append(result.WeakClaims, models.WeakClaim{
			Claim:          w.Claim,
			Why:            w.Why,
			SourceFamilies: families,
			VerifierFamily: verifier.ProviderFamily,
		})
	}
	return result, true
}

func familiesSupportingClaim(session *models.DebateSession, claimText string) []string {
	verdict := session.GetVerdict()
	if verdict == nil {
		return nil
	}
	for _, c := range verdict.ConsolidatedClaims {
		if !strings.Contains(strings.ToLower(c.Claim), strings.ToLower(claimText)) &&
			!strings.Contains(strings.ToLower(claimText), strings.ToLower(c.Claim)) {
			continue
		}
		seen := map[string]bool{}
		var fams []string
		for _, name := range c.Support {
			id := lookupAgentIDByName(session.Agents, name)
			if id == "" {
				continue
			}
			for _, a := range session.Agents {
				if a.ID == id {
					if a.ProviderFamily != "" && !seen[a.ProviderFamily] {
						seen[a.ProviderFamily] = true
						fams = append(fams, a.ProviderFamily)
					}
					break
				}
			}
		}
		return fams
	}
	return nil
}

func renderVerifierPrompt(session *models.DebateSession) (string, error) {
	tmpl, err := template.New("verify").Parse(verifyPromptTemplate)
	if err != nil {
		return "", err
	}
	verdict := session.GetVerdict()
	if verdict == nil {
		return "", fmt.Errorf("no verdict available")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"Topic":             session.Topic,
		"Synthesis":         verdict.Synthesis,
		"ConsolidatedClaims": verdict.ConsolidatedClaims,
		"ConsensusPoints":   verdict.ConsensusPoints,
		"Unresolved":        verdict.UnresolvedQuestions,
		"Reasoning":         verdict.Reasoning,
		"VendorFamilies":    distinctFamilies(session.Agents),
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func distinctFamilies(agents []models.Agent) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range agents {
		if a.ProviderFamily != "" && !seen[a.ProviderFamily] {
			seen[a.ProviderFamily] = true
			out = append(out, a.ProviderFamily)
		}
	}
	return out
}

func cleanJSONBlock(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"```json", "```"} {
		s = strings.TrimPrefix(s, p)
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

// agentFamilySources adapts a []models.Agent into the []llm.AgentFamilySource
// interface slice. We allocate only when there's actual work to do.
func agentFamilySources(agents []models.Agent) []llm.AgentFamilySource {
	out := make([]llm.AgentFamilySource, len(agents))
	for i := range agents {
		out[i] = agents[i]
	}
	return out
}

const verifyPromptTemplate = `You are auditing a debate from OUTSIDE the model family that produced it. Be skeptical.
The original debate came from these vendor families: {{range .VendorFamilies}}{{.}} {{end}}.

Topic: "{{.Topic}}"

## Consolidated Claims
{{range .ConsolidatedClaims}}
- {{.Claim}}  (status: {{.Status}})
  Support: {{range .Support}}{{.}} {{end}}
  Oppose:  {{range .Oppose}}{{.}} {{end}}
  Evidence: {{.Evidence}}
{{end}}

## Consensus Points
{{range .ConsensusPoints}}- {{.}}
{{end}}

## Unresolved Questions
{{range .UnresolvedQuestions}}- {{.}}
{{end}}

## Verdict Reasoning
{{.Reasoning}}

## Synthesis
{{.Synthesis}}

Identify claims you find WEAK or UNDERSUPPORTED. Be specific: which claim, why it's weak,
what kind of evidence would strengthen it. Skip claims that look solid.

Return ONLY valid JSON:
{
  "weak_claims": [
    { "claim": "...", "why": "..." }
  ],
  "summary": "1-2 sentence overall assessment from an outside perspective."
}`
