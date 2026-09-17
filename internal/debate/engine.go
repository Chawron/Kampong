package debate

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/kampong/debate/internal/agent"
	"github.com/kampong/debate/internal/config"
	"github.com/kampong/debate/internal/graph"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
	"github.com/kampong/debate/internal/store"
	"github.com/kampong/debate/internal/ws"
)

// Engine orchestrates the full debate lifecycle.
type Engine struct {
	agentRuntime  *agent.Runtime
	judgeModule   *Judge
	crossExam     *CrossExaminer
	extractor     *graph.Extractor
	hub           *ws.Hub
	cfg           *config.Config
	clientManager *llm.ClientManager     // optional; when set, cross-family verifier runs
	calibration   store.CalibrationStore // optional; cross-debate calibration rollups
}

// NewEngine creates a debate engine.
func NewEngine(agentRuntime *agent.Runtime, judgeModule *Judge, crossExam *CrossExaminer, extractor *graph.Extractor, hub *ws.Hub, cfg *config.Config) *Engine {
	e := &Engine{
		agentRuntime: agentRuntime,
		judgeModule:  judgeModule,
		crossExam:    crossExam,
		extractor:    extractor,
		hub:          hub,
		cfg:          cfg,
	}

	// Wire up research progress callback — broadcasts each research round to the frontend
	agentRuntime.OnResearchRound = func(debateID string, agentID string, round agent.ResearchRound) {
		e.hub.Broadcast(debateID, ws.Event{
			Event:    "research_round",
			DebateID: debateID,
			Data: map[string]interface{}{
				"agent_id":  agentID,
				"round":     round.Round,
				"queries":   round.Queries,
				"results":   round.Results,
				"findings":  round.Findings,
				"reasoning": round.Reasoning,
			},
		})
	}

	return e
}

// SetLLMClient updates the LLM client used by the graph extractor.
// Called when API credentials change to ensure the extractor uses the current key.
func (e *Engine) SetLLMClient(client *llm.Client) {
	if e.extractor != nil {
		e.extractor.SetClient(client)
	}
}

// SetClientManager wires the multi-provider manager so the cross-family
// verifier has something to pick from.
func (e *Engine) SetClientManager(cm *llm.ClientManager) {
	e.clientManager = cm
}

// SetCalibrationStore wires the cross-debate calibration rolling-average backend.
func (e *Engine) SetCalibrationStore(cs store.CalibrationStore) {
	e.calibration = cs
}

// Run executes a full debate from start to verdict.
// This is designed to run in a goroutine; it broadcasts all events via the WebSocket hub.
func (e *Engine) Run(ctx context.Context, session *models.DebateSession) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("DEBATE PANIC [%s]: %v", session.ID, r)
			session.SetStatus(models.StatusError)
			e.hub.Broadcast(session.ID, ws.Event{
				Event:    "error",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"code":    "internal",
					"message": fmt.Sprintf("Internal error: %v", r),
				},
			})
		}
	}()

	phases := e.getPhases(session.Mode)

	for _, phase := range phases {
		select {
		case <-ctx.Done():
			session.SetStatus(models.StatusCancelled)
			e.hub.Broadcast(session.ID, ws.Event{
				Event:    "phase_change",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"phase":   "cancelled",
					"message": "Debate cancelled",
				},
			})
			return
		default:
		}

		session.SetStatus(phase.status)
		if phase.round > 0 {
			session.SetRound(phase.round)
		}

		e.hub.Broadcast(session.ID, ws.Event{
			Event:    "phase_change",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"phase":        string(phase.status),
				"round":        session.GetRound(),
				"total_rounds": session.TotalRounds,
				"mode":         string(session.Mode),
				"message":      phase.message,
			},
		})

		if err := phase.executor(ctx, session); err != nil {
			if ctx.Err() != nil {
				// User-requested cancel mid-phase: classify as cancelled, not error.
				session.SetStatus(models.StatusCancelled)
				e.hub.Broadcast(session.ID, ws.Event{
					Event:    "phase_change",
					DebateID: session.ID,
					Data: map[string]interface{}{
						"phase":   "cancelled",
						"message": "Debate cancelled",
					},
				})
				return
			}
			log.Printf("DEBATE ERROR [%s] phase=%s: %v", session.ID, phase.status, err)
			session.SetStatus(models.StatusError)
			e.hub.Broadcast(session.ID, ws.Event{
				Event:    "error",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"code":    "internal",
					"message": fmt.Sprintf("Error during %s: %v", phase.status, err),
				},
			})
			return
		}
	}

	// Pillar 1+3: Post-debate analysis (calibration, consistency, auto-critique)
	e.runPostDebateAnalysis(ctx, session)
}

type debatePhase struct {
	status   models.DebateStatus
	round    int
	message  string
	executor func(context.Context, *models.DebateSession) error
}

func (e *Engine) getPhases(mode models.DebateMode) []debatePhase {
	switch mode {
	case models.ModeQuick:
		return []debatePhase{
			{status: models.StatusOpening, round: 1, message: "Round 1 — Opening Statements", executor: e.executeRound},
			{status: models.StatusRebuttal, round: 2, message: "Round 2 — Rebuttals", executor: e.executeRound},
			{status: models.StatusVerdict, message: "Judge is evaluating...", executor: e.executeVerdict},
		}
	case models.ModeStandard:
		return []debatePhase{
			{status: models.StatusOpening, round: 1, message: "Round 1 — Opening Statements", executor: e.executeRound},
			{status: models.StatusRebuttal, round: 2, message: "Round 2 — Rebuttals", executor: e.executeRound},
			{status: models.StatusClosing, round: 3, message: "Round 3 — Closing Statements", executor: e.executeRound},
			{status: models.StatusVerdict, message: "Judge is evaluating...", executor: e.executeVerdict},
		}
	case models.ModeDiscussion:
		return []debatePhase{
			{status: models.StatusPerspectives, round: 1, message: "Round 1 — Each perspective shares their angle", executor: e.executeRound},
			{status: models.StatusDeepDive, round: 2, message: "Round 2 — Deep dive into each area with research", executor: e.executeRound},
			{status: models.StatusCrossPollination, round: 3, message: "Round 3 — Cross-pollination: connect perspectives", executor: e.executeRound},
			{status: models.StatusSynthesis, round: 4, message: "Round 4 — Synthesis: what's the bigger picture?", executor: e.executeRound},
			{status: models.StatusVerdict, message: "Synthesizer is weaving insights together...", executor: e.executeSynthesis},
		}
	default: // ModeDeep
		return []debatePhase{
			{status: models.StatusOpening, round: 1, message: "Round 1 — Opening Statements", executor: e.executeRound},
			{status: models.StatusRebuttal, round: 2, message: "Round 2 — Rebuttals", executor: e.executeRound},
			{status: models.StatusCrossExam, round: 3, message: "Round 3 — Cross-Examination", executor: e.executeCrossExam},
			{status: models.StatusFinalRebuttal, round: 4, message: "Round 4 — Final Rebuttals", executor: e.executeRound},
			{status: models.StatusClosing, round: 5, message: "Round 5 — Closing Statements", executor: e.executeRound},
			{status: models.StatusVerdict, message: "Judge is evaluating...", executor: e.executeVerdict},
		}
	}
}

// executeRound runs a single debate round where each non-judge agent speaks in random order.
func (e *Engine) executeRound(ctx context.Context, session *models.DebateSession) error {
	phase := string(session.GetStatus())
	agents := e.shuffleNonJudgeAgents(session.Agents)

	for _, ag := range agents {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Pillar 2: Check if debate is paused — wait until resumed
		e.waitForResume(ctx, session)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Step 1: Search for evidence
		searchQuery, searchResults := e.agentRuntime.Search(ctx, ag, session, phase)

		// Broadcast: agent is searching
		e.hub.Broadcast(session.ID, ws.Event{
			Event:    "agent_searching",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":   ag.ID,
				"agent_name": ag.Name,
				"query":      searchQuery,
			},
		})

		// Step 2: Broadcast thinking with search results
		e.hub.Broadcast(session.ID, ws.Event{
			Event:    "agent_thinking",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":       ag.ID,
				"agent_name":     ag.Name,
				"search_query":   searchQuery,
				"search_results": searchResults,
			},
		})

		// Step 3: Attach search result URLs as metadata on the agent's claim node
		// (Don't create individual nodes per URL — that clutters the graph)
		// Source URLs will be attached to the claim node created by the extractor

		// Step 4: Generate argument via LLM with streaming
		fullText, err := e.agentRuntime.Generate(ctx, ag, session, phase, searchResults, func(token string) error {
			e.hub.Broadcast(session.ID, ws.Event{
				Event:    "agent_speaking",
				DebateID: session.ID,
				Data: map[string]interface{}{
					"agent_id": ag.ID,
					"token":    token,
				},
			})
			return nil
		})

		if err != nil {
			log.Printf("AGENT ERROR [%s] agent=%s: %v", session.ID, ag.Name, err)
			continue
		}

		// Pillar 1: Extract confidence score from argument text
		confidence := ExtractConfidence(fullText)
		if confidence > 0 {
			// Remove the confidence tag from display text
			fullText = confidenceRe.ReplaceAllString(fullText, "")
			fullText = strings.TrimSpace(fullText)
		}

		// Build transcript entry
		entry := &models.TranscriptEntry{
			ID:            fmt.Sprintf("entry-%d-%s-%d", session.GetRound(), ag.ID, len(session.GetTranscript())),
			AgentID:       ag.ID,
			AgentName:     ag.Name,
			AgentRole:     ag.Role,
			RoleType:      ag.RoleType,
			Round:         session.GetRound(),
			Phase:         models.DebateStatus(phase),
			Text:          fullText,
			Confidence:    confidence,
			SearchQuery:   searchQuery,
			SearchResults: searchResults,
		}

		// Verify cited URLs (Phase 3 — real evidence).
		// engine.go drives the round manually (not via SearchAndGenerate), so it
		// owns the citation fetch here. SearchAndGenerate does its own.
		citations := e.agentRuntime.FetchCitations(ctx, searchResults)
		entry.Citations = citations

		// Append to transcript
		session.AppendTranscript(*entry)

		// Step 5: Extract knowledge graph from this argument
		if e.extractor != nil {
			log.Printf("GRAPH [%s] running LLM extractor for agent=%s (text len=%d)", session.ID, ag.Name, len(entry.Text))
			e.extractor.ExtractAndMerge(ctx, *entry, ag.ID, searchResults)
			store := e.extractor.GetStore()
			if store != nil {
				nodes, edges := store.Snapshot()
				session.AppendGraphUpdate(models.GraphUpdate{
					NewNodes: nodes,
					NewEdges: edges,
				})

				e.hub.Broadcast(session.ID, ws.Event{
					Event:    "graph_update",
					DebateID: session.ID,
					Data: map[string]interface{}{
						"new_nodes": nodes,
						"new_edges": edges,
					},
				})
			}
		}

		reachable, unreachable := countCitationReachability(citations)

		// Broadcast: agent done
		e.hub.Broadcast(session.ID, ws.Event{
			Event:    "agent_done",
			DebateID: session.ID,
			Data: map[string]interface{}{
				"agent_id":      ag.ID,
				"entry_id":      entry.ID,
				"full_text":     entry.Text,
				"search_query":  entry.SearchQuery,
				"search_count":  len(entry.SearchResults),
				"confidence":    entry.Confidence,
				"role_type":     string(ag.RoleType),
				"vendor_family": ag.ProviderFamily,
				"provider_name": ag.ProviderName,
				"citations_reachable":   reachable,
				"citations_unreachable": unreachable,
			},
		})
	}

	return nil
}

// buildEvidenceGraph creates graph nodes and edges from search results.
func (e *Engine) buildEvidenceGraph(results []models.SearchResult, agentID string, agentName string) ([]models.GraphNode, []models.GraphEdge) {
	var nodes []models.GraphNode
	var edges []models.GraphEdge

	for _, sr := range results {
		if sr.Title == "" || sr.URL == "" {
			continue
		}
		nodeID := fmt.Sprintf("ev-%x", md5.Sum([]byte(sr.URL)))[:12]
		label := sr.Title
		if len(label) > 60 {
			label = label[:57] + "..."
		}
		nodes = append(nodes, models.GraphNode{
			ID:        nodeID,
			Label:     label,
			Type:      "evidence",
			Content:   sr.Snippet,
			SourceURL: sr.URL,
			AgentID:   agentID,
			AgentName: agentName,
		})
		edges = append(edges, models.GraphEdge{
			From:       agentID,
			To:         nodeID,
			Relation:   "cites",
			Provenance: agentID,
		})
	}
	return nodes, edges
}

// executeCrossExam runs the cross-examination phase (deep mode only).
func (e *Engine) executeCrossExam(ctx context.Context, session *models.DebateSession) error {
	return e.crossExam.Run(ctx, session, e.hub)
}

func countCitationReachability(cs []models.Citation) (reachable, unreachable int) {
	for _, c := range cs {
		if c.ReachabilityOK {
			reachable++
		} else {
			unreachable++
		}
	}
	return
}

// executeVerdict runs the judge evaluation.
func (e *Engine) executeVerdict(ctx context.Context, session *models.DebateSession) error {
	return e.judgeModule.Evaluate(ctx, session, e.hub)
}

// executeSynthesis runs the synthesizer evaluation (discussion mode).
func (e *Engine) executeSynthesis(ctx context.Context, session *models.DebateSession) error {
	return e.judgeModule.Synthesize(ctx, session, e.hub)
}

// shuffleNonJudgeAgents returns non-judge, non-synthesizer agents in random order.
func (e *Engine) shuffleNonJudgeAgents(agents []models.Agent) []models.Agent {
	var debaters []models.Agent
	for _, a := range agents {
		if a.RoleType != models.RoleJudge && a.RoleType != models.RoleSynthesizer {
			debaters = append(debaters, a)
		}
	}

	shuffled := make([]models.Agent, len(debaters))
	copy(shuffled, debaters)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled
}

// waitForResume blocks while the debate is paused (Pillar 2: Human-in-the-loop).
// Checks every 500ms if the debate has been resumed or context cancelled.
func (e *Engine) waitForResume(ctx context.Context, session *models.DebateSession) {
	for {
		if !session.IsPaused() {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
			// check again
		}
	}
}

// GetLLMClient returns the LLM client from the agent runtime (for calibration/critique).
func (e *Engine) GetLLMClient() *llm.Client {
	return e.agentRuntime.GetClient()
}

// runPostDebateAnalysis runs calibration, consistency check, auto-critique,
// vendor audit, and the cross-family verifier (when enabled) after the verdict.
func (e *Engine) runPostDebateAnalysis(ctx context.Context, session *models.DebateSession) {
	transcript := session.GetTranscript()
	verdict := session.GetVerdict()

	// Pillar 1+3: Compute agent calibrations
	if verdict != nil {
		session.SetCalibrations(ComputeCalibrations(transcript, verdict))
		log.Printf("CALIBRATION [%s] computed %d agent calibrations", session.ID, len(session.Calibrations))
	}

	// Pillar 3: Consistency check
	session.SetConsistencyChecks(CheckConsistency(transcript))
	log.Printf("CONSISTENCY [%s] checked %d agents", session.ID, len(session.ConsistencyChecks))

	// Pillar 3: Auto-critique (uses LLM, may fail gracefully)
	client := e.GetLLMClient()
	if client != nil {
		critique, err := RunAutoCritique(ctx, client, session)
		if err != nil {
			log.Printf("CRITIQUE [%s] failed (non-fatal): %v", session.ID, err)
		} else {
			session.SetCritique(critique)
			log.Printf("CRITIQUE [%s] quality=%s diversity=%d", session.ID, critique.DebateQuality, critique.DiversityScore)
		}
	}

	// Phase 2: Vendor diversity audit
	if verdict != nil {
		audit := ComputeVendorAudit(
			session.Agents,
			verdict.Scorecard,
			verdict.ConsolidatedClaims,
		)
		session.SetVendorAudit(&audit)
		log.Printf("VENDOR AUDIT [%s] families=%d warnings=%d",
			session.ID, len(audit.VendorDistribution), len(audit.BiasWarnings))
	}

	// Phase 2: Cross-family verifier (ON by default; toggleable via config).
	if e.cfg.Debate.CrossFamilyVerifierEnabled() && e.clientManager != nil && verdict != nil {
		result, ran := RunCrossFamilyVerifier(ctx, e.clientManager, session)
		if ran {
			session.SetCrossVerify(result)
			log.Printf("CROSS-VERIFY [%s] verifier=%s weak_claims=%d",
				session.ID, result.VerifierFamily, len(result.WeakClaims))
		} else if result != nil {
			log.Printf("CROSS-VERIFY [%s] skipped: %s", session.ID, result.SkipReason)
			session.SetCrossVerify(result)
		}
	}

	// Broadcast post-debate analysis
	calibrations := session.Calibrations
	consistency := session.ConsistencyChecks
	critique := session.Critique
	vendorAudit := session.VendorAudit
	crossVerify := session.CrossVerify
	e.hub.Broadcast(session.ID, ws.Event{
		Event:    "post_debate_analysis",
		DebateID: session.ID,
		Data: map[string]interface{}{
			"calibrations":    calibrations,
			"consistency":     consistency,
			"critique":        critique,
			"vendor_audit":    vendorAudit,
			"cross_verify":    crossVerify,
		},
	})

	// Standalone vendor_audit event for UI consumers that prefer it separate.
	if verdict != nil {
		e.hub.Broadcast(session.ID, ws.Event{
			Event:    "vendor_audit",
			DebateID: session.ID,
			Data:     vendorAudit,
		})
	}

	// Phase 4: Cross-debate calibration — fold this debate into the rolling
	// averages so future debates can down-weight historically over-confident
	// agents (see ApplyCalibrationWeight).
	UpdateCalibrationFromSession(e.calibration, session)
}
