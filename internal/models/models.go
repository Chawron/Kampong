package models

import (
	"sync"
	"time"
)

// DebateStatus represents the current phase of a debate.
type DebateStatus string

const (
	StatusAnalyzing      DebateStatus = "analyzing"
	StatusComposingPanel DebateStatus = "composing_panel"
	StatusOpening        DebateStatus = "opening"
	StatusRebuttal       DebateStatus = "rebuttal"
	StatusCrossExam      DebateStatus = "cross_examination"
	StatusFinalRebuttal  DebateStatus = "final_rebuttal"
	StatusClosing        DebateStatus = "closing"
	StatusVerdict        DebateStatus = "verdict"
	StatusCancelled      DebateStatus = "cancelled"
	StatusError          DebateStatus = "error"
	StatusPaused         DebateStatus = "paused"          // Human-in-the-loop
	StatusSocialReaction DebateStatus = "social_reaction" // Social simulation phase
	// Discussion mode phases
	StatusPerspectives     DebateStatus = "perspectives"
	StatusDeepDive         DebateStatus = "deep_dive"
	StatusCrossPollination DebateStatus = "cross_pollination"
	StatusSynthesis        DebateStatus = "synthesis"
)

// DebateMode represents the debate depth.
type DebateMode string

const (
	ModeQuick      DebateMode = "quick"
	ModeStandard   DebateMode = "standard"
	ModeDeep       DebateMode = "deep"
	ModeDiscussion DebateMode = "discussion"
)

// AgentRoleType classifies an agent's role in the panel.
type AgentRoleType string

const (
	RoleCore         AgentRoleType = "core"
	RoleAdjacent     AgentRoleType = "adjacent"
	RolePractitioner AgentRoleType = "practitioner"
	RoleLayperson    AgentRoleType = "layperson"
	RoleJudge        AgentRoleType = "judge"
	RolePerspective  AgentRoleType = "perspective"   // discussion mode: angle expert
	RoleSynthesizer  AgentRoleType = "synthesizer"    // discussion mode: replaces judge
)

// Agent represents a single debate participant.
type Agent struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Role           string        `json:"role"`
	Expertise      string        `json:"expertise"`
	RoleType       AgentRoleType `json:"role_type"`
	Icon           string        `json:"icon"`
	ProviderName   string        `json:"provider_name"`   // Which LLM provider this agent uses
	ProviderFamily string        `json:"provider_family"` // Resolved vendor family for diversity routing
	SystemPrompt   string        `json:"-"`
}

// Family returns the agent's vendor family (used by llm.AgentFamilySource).
func (a Agent) Family() string { return a.ProviderFamily }

// TranscriptEntry records one speech or Q&A exchange.
type TranscriptEntry struct {
	ID             string         `json:"id"`
	AgentID        string         `json:"agent_id"`
	AgentName      string         `json:"agent_name"`
	AgentRole      string         `json:"agent_role"`
	RoleType       AgentRoleType  `json:"role_type,omitempty"`
	Round          int            `json:"round"`
	Phase          DebateStatus   `json:"phase"`
	Text           string         `json:"text"`
	Confidence     int            `json:"confidence,omitempty"`     // 0-100, Pillar 1
	EvidenceScore  int            `json:"evidence_score,omitempty"` // 0-100, Pillar 3
	SearchQuery    string         `json:"search_query,omitempty"`
	SearchResults  []SearchResult `json:"search_results,omitempty"`
	Citations      []Citation     `json:"citations,omitempty"` // reachable/unreachable URLs the agent cited
	Timestamp      time.Time      `json:"timestamp"`
	QuestionText   string         `json:"question_text,omitempty"`
	TargetAgentID  string         `json:"target_agent_id,omitempty"`
	IsUserInject   bool           `json:"is_user_inject,omitempty"`   // Pillar 2: user-injected evidence
	IsChallenge    bool           `json:"is_challenge,omitempty"`     // Pillar 2: user challenge
}

// SearchResult holds a single web search hit.
type SearchResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Snippet       string  `json:"snippet"`
	Score         float64 `json:"score"`
	Source        string  `json:"source,omitempty"`         // which searcher surfaced this
	Domain        string  `json:"domain,omitempty"`         // eTLD+1
	CrossVerified bool    `json:"cross_verified,omitempty"` // true when two sources agreed on the domain
}

// Citation is a URL an agent actually surfaced, with reachability metadata.
type Citation struct {
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	Snippet        string    `json:"snippet"`
	Domain         string    `json:"domain"`
	SourceFamily   string    `json:"source_family,omitempty"`
	CrossVerified  bool      `json:"cross_verified,omitempty"`
	FetchedAt      time.Time `json:"fetched_at,omitempty"`
	ReachabilityOK bool      `json:"reachability_ok"`
}

// GraphNode is a concept, claim, or evidence node in the knowledge graph.
type GraphNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Refs     int    `json:"refs"`
	Content  string `json:"content,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	AgentID  string `json:"agent_id,omitempty"`
	Round    int    `json:"round,omitempty"`
	Phase    string `json:"phase,omitempty"`
}

// GraphEdge is a relationship between two graph nodes.
type GraphEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Relation   string `json:"relation"`
	Provenance string `json:"provenance"`
}

// AgentScore is a single agent's score breakdown.
type AgentScore struct {
	AgentID   string  `json:"agent_id"`
	AgentName string  `json:"agent_name"`
	Evidence  float64 `json:"evidence"`
	Logic     float64 `json:"logic"`
	Novelty   float64 `json:"novelty"`
	Total     float64 `json:"total"`
}

// ConsolidatedClaim is a finding synthesized by the judge from the debate.
type ConsolidatedClaim struct {
	Claim      string   `json:"claim"`
	Status     string   `json:"status"`      // "established", "contested", "disputed", "unresolved"
	Support    []string `json:"support"`     // agent names who support this
	Oppose     []string `json:"oppose"`      // agent names who oppose this
	Evidence   string   `json:"evidence"`    // summary of evidence quality
}

// Verdict is the judge's final evaluation.
type Verdict struct {
	WinnerAgentID     string              `json:"winner_agent_id"`
	Scorecard         []AgentScore        `json:"scorecard"`
	TurningPoints     []string            `json:"turning_points"`
	StrongestEvidence struct {
		AgentID string `json:"agent_id"`
		Claim   string `json:"claim"`
		Why     string `json:"why"`
	} `json:"strongest_evidence"`
	Reasoning          string              `json:"reasoning"`
	CalibrationNotes   string              `json:"calibration_notes,omitempty"` // Pillar 1: confidence calibration notes
	ConsolidatedClaims []ConsolidatedClaim `json:"consolidated_claims"`
	ConsensusPoints    []string            `json:"consensus_points"`
	UnresolvedQuestions []string           `json:"unresolved_questions"`
	Synthesis          string              `json:"synthesis"`
}

// Insight is a key finding from a discussion.
type Insight struct {
	Theme      string   `json:"theme"`
	Finding    string   `json:"finding"`
	SupportBy  []string `json:"support_by"`
	Evidence   string   `json:"evidence"`
}

// Synthesis is the discussion mode equivalent of a Verdict — no winner, just insights.
type Synthesis struct {
	KeyInsights       []Insight `json:"key_insights"`
	CommonThemes      []string  `json:"common_themes"`
	SurprisingFindings []string `json:"surprising_findings"`
	OpenQuestions     []string  `json:"open_questions"`
	Overview          string    `json:"overview"`
}

// ── Pillar 1: Confidence & Calibration ──

// AgentCalibration tracks an agent's confidence accuracy over time.
type AgentCalibration struct {
	AgentID        string  `json:"agent_id"`
	AgentName      string  `json:"agent_name"`
	TotalArguments int     `json:"total_arguments"`
	AvgConfidence  float64 `json:"avg_confidence"`  // average self-reported confidence
	JudgeAvgScore  float64 `json:"judge_avg_score"` // average score received from judge
	CalibrationRatio float64 `json:"calibration_ratio"` // judge_score / confidence (>1 = underconfident, <1 = overconfident)
}

// ── Pillar 2: Human-in-the-Loop ──

// UserInjection is evidence or a question injected by the user mid-debate.
type UserInjection struct {
	ID            string    `json:"id"`
	DebateID      string    `json:"debate_id"`
	Type          string    `json:"type"` // "evidence", "question", "challenge", "redirect"
	TargetAgentID string    `json:"target_agent_id,omitempty"`
	Text          string    `json:"text"`
	Timestamp     time.Time `json:"timestamp"`
}

// DebateControl holds the control state for human-in-the-loop interaction.
type DebateControl struct {
	IsPaused       bool         `json:"is_paused"`
	PausedAt       string       `json:"paused_at,omitempty"`
	PauseReason    string       `json:"pause_reason,omitempty"`
	PreviousStatus DebateStatus `json:"previous_status,omitempty"` // status before pause, for resume
}

// ── Pillar 3: Evaluation & Calibration ──

// ConsistencyCheck tracks whether an agent's claims are consistent across rounds.
type ConsistencyCheck struct {
	AgentID       string   `json:"agent_id"`
	AgentName     string   `json:"agent_name"`
	Claims        []string `json:"claims"`
	Contradictions []string `json:"contradictions,omitempty"`
	ConsistencyPct int     `json:"consistency_pct"` // 0-100
}

// AutoCritique is a post-debate quality review by a separate agent set.
type AutoCritique struct {
	DebateQuality    string   `json:"debate_quality"`    // "excellent", "good", "fair", "poor"
	Strengths        []string `json:"strengths"`
	Weaknesses       []string `json:"weaknesses"`
	MissingPerspectives []string `json:"missing_perspectives"`
	ResearchQuality  int      `json:"research_quality"`  // 0-100
	ArgumentDepth    int      `json:"argument_depth"`     // 0-100
	DiversityScore   int      `json:"diversity_score"`    // 0-100
	Recommendations  []string `json:"recommendations"`
}

// ── Pillar 4: Social/Public Reaction Layer ──

// SocialPersona represents a simulated public reaction persona.
type SocialPersona struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Persona     string `json:"persona"`      // "concerned_parent", "skeptic", "executive", "journalist", "patient", etc.
	Bias        string `json:"bias"`         // "supportive", "neutral", "skeptical", "hostile"
	KnowledgeLvl string `json:"knowledge_level"` // "expert", "informed", "casual", "uninformed"
}

// SocialReaction is one persona's reaction to the debate outcome.
type SocialReaction struct {
	PersonaID    string `json:"persona_id"`
	PersonaName  string `json:"persona_name"`
	PersonaType  string `json:"persona_type"`
	Reaction     string `json:"reaction"`
	Sentiment    string `json:"sentiment"` // "positive", "neutral", "negative", "mixed"
	Amplify      string `json:"amplify"`   // what they'd share/amplify
	Distort      string `json:"distort"`   // how they might misrepresent
	TrustScore   int    `json:"trust_score"` // 0-100, how much they trust the expert verdict
}

// SocialSimulation holds the full public reaction simulation results.
type SocialSimulation struct {
	Personas       []SocialPersona  `json:"personas"`
	Reactions      []SocialReaction `json:"reactions"`
	NarrativeSummary string         `json:"narrative_summary"`
	DominantNarrative string        `json:"dominant_narrative"`
	RiskAreas      []string         `json:"risk_areas"` // where narrative might be distorted
	OverallSentiment string         `json:"overall_sentiment"`
}

// ── Pillar 5: Cost & Latency Control ──

// ResearchCacheEntry stores a cached search result to avoid duplicate API calls.
type ResearchCacheEntry struct {
	Query     string         `json:"query"`
	Results   []SearchResult `json:"results"`
	Timestamp time.Time      `json:"timestamp"`
	HitCount  int            `json:"hit_count"` // how many agents reused this
}

// DebateComplexity assesses topic difficulty for adaptive depth selection.
type DebateComplexity struct {
	Score           string `json:"score"`            // "simple", "moderate", "complex", "highly_complex"
	RecommendedMode string `json:"recommended_mode"` // suggested debate mode
	Reasons         []string `json:"reasons"`
	ControversyLevel int    `json:"controversy_level"` // 0-100
	DomainCount     int     `json:"domain_count"`     // number of domains involved
}

// ── Pillar 6: Medical Enhancements ──

// DifferentialRanking is a ranked differential diagnosis with probabilities.
type DifferentialRanking struct {
	Diagnosis    string  `json:"diagnosis"`
	ICD10        string  `json:"icd10,omitempty"`
	Probability  float64 `json:"probability"`  // 0.0-1.0
	Evidence     string  `json:"evidence"`
	Supporting   []string `json:"supporting"`   // findings that support
	Against      []string `json:"against"`      // findings that argue against
	NextTest     string  `json:"next_test,omitempty"` // recommended test to confirm/rule out
	Urgency      string  `json:"urgency"`       // "routine", "urgent", "emergency"
}

// DynamicRiskScore combines multiple risk factors into a composite score.
type DynamicRiskScore struct {
	TotalScore       int      `json:"total_score"`        // 0-100 composite risk
	RiskLevel        string   `json:"risk_level"`         // "low", "moderate", "high", "critical"
	RedFlagScore     int      `json:"red_flag_score"`     // 0-100
	DrugRiskScore    int      `json:"drug_risk_score"`    // 0-100
	LabRiskScore     int      `json:"lab_risk_score"`     // 0-100
	AgeRiskScore     int      `json:"age_risk_score"`     // 0-100
	HistoryRiskScore int      `json:"history_risk_score"` // 0-100
	ContributingFactors []string `json:"contributing_factors"`
	ImmediateActions []string `json:"immediate_actions"`
}

// LongitudinalCase tracks a patient across multiple visits.
type LongitudinalCase struct {
	PatientID    string             `json:"patient_id"`
	VisitHistory []LongitudinalVisit `json:"visit_history"`
	Trends       []string           `json:"trends"`       // observed trends over time
	Alerts       []string           `json:"alerts"`       // alerts based on longitudinal data
}

// LongitudinalVisit is one visit in a patient's longitudinal record.
type LongitudinalVisit struct {
	DebateID    string    `json:"debate_id"`
	Date        time.Time `json:"date"`
	PrimaryDx   string    `json:"primary_diagnosis"`
	RiskScore   int       `json:"risk_score"`
	KeyFindings []string  `json:"key_findings"`
	Changes     []string  `json:"changes_from_prior"` // what changed since last visit
}

// GraphUpdate holds incremental graph changes broadcast via WebSocket.
type GraphUpdate struct {
	NewNodes []GraphNode `json:"new_nodes"`
	NewEdges []GraphEdge `json:"new_edges"`
}

// ── Vendor diversity audit (post-debate) ──

// ClaimVendorSplit summarises which vendor families supported / opposed a claim.
type ClaimVendorSplit struct {
	Claim          string   `json:"claim"`
	SupportFamilies []string `json:"support_families"`
	OpposeFamilies  []string `json:"oppose_families"`
}

// VendorAudit is the output of ComputeVendorAudit, attached to post_debate_analysis.
type VendorAudit struct {
	VendorDistribution map[string]int         `json:"vendor_distribution"` // family → agent count
	InterVendorClaims  []ClaimVendorSplit     `json:"inter_vendor_claims"`
	BiasWarnings       []string               `json:"bias_warnings,omitempty"`
	FamilyScorecard    map[string]float64     `json:"family_scorecard,omitempty"` // family → avg judge score
}

// ── Cross-family verifier (post-debate) ──

// WeakClaim is one claim that the cross-family verifier flagged.
type WeakClaim struct {
	Claim          string `json:"claim"`
	Why            string `json:"why"`
	SourceFamilies []string `json:"source_families"`
	VerifierFamily string `json:"verifier_family"`
}

// CrossVerifyResult is the output of the cross-family verifier.
type CrossVerifyResult struct {
	VerifierFamily string       `json:"verifier_family"`
	VerifierProvider string     `json:"verifier_provider"`
	WeakClaims     []WeakClaim  `json:"weak_claims"`
	Summary        string       `json:"summary"`
	Skipped        bool         `json:"skipped,omitempty"`
	SkipReason     string       `json:"skip_reason,omitempty"`
}

// DebateSession holds the full state of an active debate.
type DebateSession struct {
	ID          string             `json:"id"`
	Topic       string             `json:"topic"`
	Mode        DebateMode         `json:"mode"`
	Status      DebateStatus       `json:"status"`
	Round       int                `json:"round"`
	TotalRounds int                `json:"total_rounds"`
	Agents      []Agent            `json:"agents"`
	Transcript  []TranscriptEntry  `json:"transcript"`
	Graph       struct {
		Nodes []GraphNode `json:"nodes"`
		Edges []GraphEdge `json:"edges"`
	} `json:"graph"`
	Verdict   *Verdict   `json:"verdict,omitempty"`
	SynthesisResult *Synthesis `json:"synthesis_result,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// Attachments for medical context
	Attachments []Attachment `json:"attachments,omitempty"`
	// Pillar 1: Calibration
	Calibrations []AgentCalibration `json:"calibrations,omitempty"`
	// Pillar 2: Human-in-the-loop
	Control    DebateControl    `json:"control,omitempty"`
	Injections []UserInjection  `json:"injections,omitempty"`
	// Pillar 3: Evaluation
	ConsistencyChecks []ConsistencyCheck `json:"consistency_checks,omitempty"`
	Critique          *AutoCritique      `json:"critique,omitempty"`
	// Phase 2: Vendor diversity audit + cross-family verifier
	VendorAudit *VendorAudit        `json:"vendor_audit,omitempty"`
	CrossVerify *CrossVerifyResult `json:"cross_verify,omitempty"`
	// Pillar 4: Social simulation
	SocialResult *SocialSimulation `json:"social_result,omitempty"`
	// Pillar 5: Cost control
	ResearchCache []ResearchCacheEntry `json:"-"` // not serialized, runtime only
	// Pillar 6: Medical enhancements
	DifferentialRanking []DifferentialRanking `json:"differential_ranking,omitempty"`
	RiskScore           *DynamicRiskScore     `json:"risk_score,omitempty"`
	mu        sync.RWMutex
}

// Attachment represents an uploaded file with its analysis.
type Attachment struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`         // "image", "document"
	FilePath     string `json:"file_path"`
	MimeType     string `json:"mime_type"`
	Analysis     string `json:"analysis"`     // For images: vision model output
	TextContent  string `json:"text_content"` // For text files
	Size         int64  `json:"size"`
}

// AppendTranscript safely appends a transcript entry.
func (s *DebateSession) AppendTranscript(entry TranscriptEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Transcript = append(s.Transcript, entry)
}

// GetTranscript safely returns a copy of the transcript.
func (s *DebateSession) GetTranscript() []TranscriptEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TranscriptEntry, len(s.Transcript))
	copy(out, s.Transcript)
	return out
}

// GetRecentTranscript returns the last N entries, newest first.
func (s *DebateSession) GetRecentTranscript(n int) []TranscriptEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Transcript) == 0 {
		return nil
	}
	start := len(s.Transcript) - n
	if start < 0 {
		start = 0
	}
	out := make([]TranscriptEntry, len(s.Transcript)-start)
	copy(out, s.Transcript[start:])
	return out
}

// AppendGraphUpdate safely merges new nodes and edges.
func (s *DebateSession) AppendGraphUpdate(update GraphUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Deduplicate nodes by ID
	nodeSet := make(map[string]bool)
	for _, n := range s.Graph.Nodes {
		nodeSet[n.ID] = true
	}
	for _, n := range update.NewNodes {
		if !nodeSet[n.ID] {
			s.Graph.Nodes = append(s.Graph.Nodes, n)
			nodeSet[n.ID] = true
		}
	}
	s.Graph.Edges = append(s.Graph.Edges, update.NewEdges...)
}

// SetStatus safely updates the debate status.
func (s *DebateSession) SetStatus(status DebateStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// SetRound safely updates the current round.
func (s *DebateSession) SetRound(r int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Round = r
}

// GetRound safely returns the current round.
func (s *DebateSession) GetRound() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Round
}

// SetVerdict safely sets the verdict.
func (s *DebateSession) SetVerdict(v *Verdict) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Verdict = v
}

// GetVerdict safely returns the verdict (may be nil).
func (s *DebateSession) GetVerdict() *Verdict {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Verdict
}

// SetSynthesis safely sets the synthesis result.
func (s *DebateSession) SetSynthesis(syn *Synthesis) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SynthesisResult = syn
}

// GetSynthesis safely returns the synthesis result (may be nil).
func (s *DebateSession) GetSynthesis() *Synthesis {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SynthesisResult
}

// GetStatus safely returns the debate status.
func (s *DebateSession) GetStatus() DebateStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// SetCalibrations safely sets the agent calibrations.
func (s *DebateSession) SetCalibrations(c []AgentCalibration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calibrations = c
}

// SetConsistencyChecks safely sets the consistency checks.
func (s *DebateSession) SetConsistencyChecks(c []ConsistencyCheck) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConsistencyChecks = c
}

// SetCritique safely sets the auto-critique result.
func (s *DebateSession) SetCritique(c *AutoCritique) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Critique = c
}

// SetVendorAudit safely sets the vendor audit.
func (s *DebateSession) SetVendorAudit(a *VendorAudit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VendorAudit = a
}

// SetCrossVerify safely sets the cross-family verification result.
func (s *DebateSession) SetCrossVerify(r *CrossVerifyResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CrossVerify = r
}

// SetSocialResult safely sets the social simulation result.
func (s *DebateSession) SetSocialResult(r *SocialSimulation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SocialResult = r
}

// Snapshot returns a deep copy of the session safe for marshalling or reading
// while the debate goroutine may still be mutating the original.
func (s *DebateSession) Snapshot() *DebateSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Build the copy field by field so the mutex is never copied.
	cp := &DebateSession{
		ID:               s.ID,
		Topic:            s.Topic,
		Mode:             s.Mode,
		Status:           s.Status,
		Round:            s.Round,
		TotalRounds:      s.TotalRounds,
		Verdict:          s.Verdict,
		SynthesisResult:  s.SynthesisResult,
		CreatedAt:        s.CreatedAt,
		Control:          s.Control,
		Critique:         s.Critique,
		VendorAudit:      s.VendorAudit,
		CrossVerify:      s.CrossVerify,
		SocialResult:     s.SocialResult,
		RiskScore:        s.RiskScore,
	}
	cp.Agents = make([]Agent, len(s.Agents))
	copy(cp.Agents, s.Agents)
	cp.Transcript = make([]TranscriptEntry, len(s.Transcript))
	copy(cp.Transcript, s.Transcript)
	cp.Graph.Nodes = make([]GraphNode, len(s.Graph.Nodes))
	copy(cp.Graph.Nodes, s.Graph.Nodes)
	cp.Graph.Edges = make([]GraphEdge, len(s.Graph.Edges))
	copy(cp.Graph.Edges, s.Graph.Edges)
	cp.Attachments = make([]Attachment, len(s.Attachments))
	copy(cp.Attachments, s.Attachments)
	cp.Calibrations = make([]AgentCalibration, len(s.Calibrations))
	copy(cp.Calibrations, s.Calibrations)
	cp.ConsistencyChecks = make([]ConsistencyCheck, len(s.ConsistencyChecks))
	copy(cp.ConsistencyChecks, s.ConsistencyChecks)
	cp.Injections = make([]UserInjection, len(s.Injections))
	copy(cp.Injections, s.Injections)
	cp.ResearchCache = make([]ResearchCacheEntry, len(s.ResearchCache))
	copy(cp.ResearchCache, s.ResearchCache)
	cp.DifferentialRanking = make([]DifferentialRanking, len(s.DifferentialRanking))
	copy(cp.DifferentialRanking, s.DifferentialRanking)
	return cp
}

// Pause sets the debate to paused state, preserving the previous status.
func (s *DebateSession) Pause(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Control.PreviousStatus = s.Status
	s.Control.IsPaused = true
	s.Control.PausedAt = time.Now().UTC().Format(time.RFC3339)
	s.Control.PauseReason = reason
	s.Status = StatusPaused
}

// Resume clears the paused state and restores the previous status.
func (s *DebateSession) Resume() DebateStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	restoreTo := s.Control.PreviousStatus
	if restoreTo == "" || restoreTo == StatusPaused {
		restoreTo = StatusOpening // fallback
	}
	s.Control.IsPaused = false
	s.Control.PausedAt = ""
	s.Control.PauseReason = ""
	s.Control.PreviousStatus = ""
	s.Status = restoreTo
	return restoreTo
}

// AddInjection appends a user injection and returns it.
func (s *DebateSession) AddInjection(inj UserInjection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Injections = append(s.Injections, inj)
}

// IsPaused returns whether the debate is currently paused.
func (s *DebateSession) IsPaused() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Control.IsPaused
}
