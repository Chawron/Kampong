package medical

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kampong/debate/internal/debate"
	"github.com/kampong/debate/internal/models"
)

// SecondOpinionMode runs multiple independent debate panels on the same case.
type SecondOpinionMode struct {
	engine    *debate.Engine
	medEngine *MedicalDebateEngine
}

// SecondOpinionResult contains results from multiple opinion panels.
type SecondOpinionResult struct {
	CaseID        string              `json:"case_id"`
	PanelResults  []PanelResult       `json:"panel_results"`
	Consensus     string              `json:"consensus"`
	Disagreements []string            `json:"disagreements"`
	Synthesis     string              `json:"synthesis"`
}

// PanelResult contains the result from a single opinion panel.
type PanelResult struct {
	PanelID       string    `json:"panel_id"`
	PanelName     string    `json:"panel_name"`
	DebateID      string    `json:"debate_id"`
	Diagnosis     []string  `json:"diagnosis"`
	Confidence    string    `json:"confidence"`
	KeyFindings   []string  `json:"key_findings"`
	Recommendations []string `json:"recommendations"`
}

// NewSecondOpinionMode creates a new second opinion mode.
func NewSecondOpinionMode(engine *debate.Engine, medEngine *MedicalDebateEngine) *SecondOpinionMode {
	return &SecondOpinionMode{
		engine:    engine,
		medEngine: medEngine,
	}
}

// RunSecondOpinion runs multiple independent panels on the same case.
func (s *SecondOpinionMode) RunSecondOpinion(ctx context.Context, caseData *MedicalCase, numPanels int) (*SecondOpinionResult, error) {
	if numPanels < 2 {
		numPanels = 2
	}
	if numPanels > 5 {
		numPanels = 5
	}

	result := &SecondOpinionResult{
		CaseID:       fmt.Sprintf("case_%d", caseData.CreatedAt.Unix()),
		PanelResults: make([]PanelResult, 0, numPanels),
	}

	// Run panels in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	panelResults := make([]PanelResult, numPanels)

	for i := 0; i < numPanels; i++ {
		wg.Add(1)
		go func(panelIndex int) {
			defer wg.Done()

			panelResult, err := s.runSinglePanel(ctx, caseData, panelIndex)
			if err != nil {
				// Log error but continue with other panels
				return
			}

			mu.Lock()
			panelResults[panelIndex] = *panelResult
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Collect results
	for _, pr := range panelResults {
		if pr.DebateID != "" {
			result.PanelResults = append(result.PanelResults, pr)
		}
	}

	// Analyze consensus and disagreements
	result.Consensus = s.analyzeConsensus(result.PanelResults)
	result.Disagreements = s.analyzeDisagreements(result.PanelResults)
	result.Synthesis = s.synthesizeOpinions(result.PanelResults)

	return result, nil
}

// runSinglePanel runs a single debate panel on the case.
func (s *SecondOpinionMode) runSinglePanel(ctx context.Context, caseData *MedicalCase, panelIndex int) (*PanelResult, error) {
	// Create a unique debate for this panel
	session := &models.DebateSession{
		ID:         fmt.Sprintf("panel_%d_%d", caseData.CreatedAt.Unix(), panelIndex),
		Topic:      fmt.Sprintf("Second Opinion Panel %d: %s", panelIndex+1, s.generateTopicFromCase(caseData)),
		Mode:       models.ModeDeep,
		Status:     models.StatusAnalyzing,
		Agents:     []models.Agent{},
		Transcript: []models.TranscriptEntry{},
	}
	session.Graph.Nodes = []models.GraphNode{}
	session.Graph.Edges = []models.GraphEdge{}

	// Create panel-specific agents with different perspectives
	agents := s.createPanelAgents(panelIndex)
	session.Agents = agents

	// Run the debate
	s.medEngine.RunMedicalDebate(ctx, session, caseData)

	// Extract results from the debate
	result := &PanelResult{
		PanelID:   fmt.Sprintf("panel_%d", panelIndex+1),
		PanelName: s.getPanelName(panelIndex),
		DebateID:  session.ID,
	}

	// Extract diagnosis and recommendations from transcript
	result.Diagnosis = s.extractDiagnosis(session)
	result.KeyFindings = s.extractKeyFindings(session)
	result.Recommendations = s.extractRecommendations(session)
	result.Confidence = s.assessConfidence(session)

	return result, nil
}

// createPanelAgents creates agents with different perspectives for a panel.
func (s *SecondOpinionMode) createPanelAgents(panelIndex int) []models.Agent {
	// Different panels have different compositions
	switch panelIndex {
	case 0:
		// Panel 1: Conservative approach
		return []models.Agent{
			{ID: "p1_diag", Name: "Conservative Diagnostician", Role: "diagnostician", Expertise: "Evidence-based conservative diagnosis", RoleType: models.RoleCore, Icon: "🩺"},
			{ID: "p1_spec", Name: "Internal Medicine Specialist", Role: "specialist", Expertise: "General internal medicine", RoleType: models.RoleCore, Icon: "⚕️"},
			{ID: "p1_evid", Name: "Evidence Reviewer", Role: "evidence_reviewer", Expertise: "Critical appraisal of literature", RoleType: models.RoleAdjacent, Icon: "📚"},
			{ID: "p1_synth", Name: "Synthesizer", Role: "synthesizer", Expertise: "Integrating multiple perspectives", RoleType: models.RoleSynthesizer, Icon: "🧩"},
		}
	case 1:
		// Panel 2: Aggressive/interventional approach
		return []models.Agent{
			{ID: "p2_diag", Name: "Interventional Diagnostician", Role: "diagnostician", Expertise: "Comprehensive diagnostic workup", RoleType: models.RoleCore, Icon: "🔬"},
			{ID: "p2_spec", Name: "Subspecialist", Role: "specialist", Expertise: "Advanced subspecialty expertise", RoleType: models.RoleCore, Icon: "⚕️"},
			{ID: "p2_pharm", Name: "Clinical Pharmacist", Role: "pharmacist", Expertise: "Complex medication management", RoleType: models.RoleAdjacent, Icon: "💊"},
			{ID: "p2_synth", Name: "Synthesizer", Role: "synthesizer", Expertise: "Integrating complex data", RoleType: models.RoleSynthesizer, Icon: "🧩"},
		}
	case 2:
		// Panel 3: Patient-centered approach
		return []models.Agent{
			{ID: "p3_diag", Name: "Patient-Centered Diagnostician", Role: "diagnostician", Expertise: "Shared decision-making", RoleType: models.RoleCore, Icon: "🤝"},
			{ID: "p3_adv", Name: "Patient Advocate", Role: "patient_advocate", Expertise: "Patient preferences and values", RoleType: models.RoleLayperson, Icon: "👥"},
			{ID: "p3_ethic", Name: "Medical Ethicist", Role: "ethicist", Expertise: "Ethical considerations", RoleType: models.RoleAdjacent, Icon: "⚖️"},
			{ID: "p3_synth", Name: "Synthesizer", Role: "synthesizer", Expertise: "Balancing medical and patient perspectives", RoleType: models.RoleSynthesizer, Icon: "🧩"},
		}
	case 3:
		// Panel 4: Cost-conscious approach
		return []models.Agent{
			{ID: "p4_diag", Name: "Cost-Effective Diagnostician", Role: "diagnostician", Expertise: "High-value care", RoleType: models.RoleCore, Icon: "💰"},
			{ID: "p4_spec", Name: "Primary Care Specialist", Role: "specialist", Expertise: "Primary care management", RoleType: models.RoleCore, Icon: "🏥"},
			{ID: "p4_evid", Name: "Health Economist", Role: "evidence_reviewer", Expertise: "Cost-effectiveness analysis", RoleType: models.RoleAdjacent, Icon: "📊"},
			{ID: "p4_synth", Name: "Synthesizer", Role: "synthesizer", Expertise: "Balancing outcomes and costs", RoleType: models.RoleSynthesizer, Icon: "🧩"},
		}
	default:
		// Panel 5: Academic/research approach
		return []models.Agent{
			{ID: "p5_diag", Name: "Academic Diagnostician", Role: "diagnostician", Expertise: "Academic medical center approach", RoleType: models.RoleCore, Icon: "🎓"},
			{ID: "p5_spec", Name: "Research Specialist", Role: "specialist", Expertise: "Cutting-edge research", RoleType: models.RoleCore, Icon: "🔬"},
			{ID: "p5_evid", Name: "Research Methodologist", Role: "evidence_reviewer", Expertise: "Research methodology", RoleType: models.RoleAdjacent, Icon: "📖"},
			{ID: "p5_synth", Name: "Synthesizer", Role: "synthesizer", Expertise: "Translating research to practice", RoleType: models.RoleSynthesizer, Icon: "🧩"},
		}
	}
}

// getPanelName returns a descriptive name for a panel.
func (s *SecondOpinionMode) getPanelName(panelIndex int) string {
	names := []string{
		"Conservative Approach Panel",
		"Interventional Approach Panel",
		"Patient-Centered Panel",
		"Cost-Conscious Panel",
		"Academic/Research Panel",
	}
	if panelIndex < len(names) {
		return names[panelIndex]
	}
	return fmt.Sprintf("Panel %d", panelIndex+1)
}

// generateTopicFromCase creates a topic string from the case.
func (s *SecondOpinionMode) generateTopicFromCase(caseData *MedicalCase) string {
	if len(caseData.Symptoms) > 0 {
		return fmt.Sprintf("Medical case: %d-year-old %s with %s",
			caseData.PatientInfo.Age,
			caseData.PatientInfo.Sex,
			caseData.Symptoms[0].Name)
	}
	return "Medical case discussion"
}

// extractDiagnosis extracts diagnosis from debate transcript.
func (s *SecondOpinionMode) extractDiagnosis(session *models.DebateSession) []string {
	// Extract from transcript - look for diagnostic statements
	var diagnoses []string
	for _, entry := range session.Transcript {
		// Simple extraction - in production, use LLM to extract
		if containsDiagnosticKeywords(entry.Text) {
			diagnoses = append(diagnoses, entry.Text[:min(len(entry.Text), 200)])
		}
	}
	return diagnoses
}

// extractKeyFindings extracts key findings from debate.
func (s *SecondOpinionMode) extractKeyFindings(session *models.DebateSession) []string {
	var findings []string
	for _, entry := range session.Transcript {
		if len(entry.Text) > 50 {
			findings = append(findings, entry.Text[:min(len(entry.Text), 150)])
		}
	}
	if len(findings) > 5 {
		findings = findings[:5]
	}
	return findings
}

// extractRecommendations extracts recommendations from debate.
func (s *SecondOpinionMode) extractRecommendations(session *models.DebateSession) []string {
	var recommendations []string
	for _, entry := range session.Transcript {
		if containsRecommendationKeywords(entry.Text) {
			recommendations = append(recommendations, entry.Text[:min(len(entry.Text), 200)])
		}
	}
	return recommendations
}

// assessConfidence assesses the confidence level of the panel.
func (s *SecondOpinionMode) assessConfidence(session *models.DebateSession) string {
	// Simple heuristic based on transcript length and agreement
	if len(session.Transcript) > 10 {
		return "high"
	} else if len(session.Transcript) > 5 {
		return "moderate"
	}
	return "low"
}

// analyzeConsensus analyzes consensus across panels.
func (s *SecondOpinionMode) analyzeConsensus(panels []PanelResult) string {
	if len(panels) == 0 {
		return "No panels completed"
	}

	// Simple consensus analysis
	consensusCount := 0
	for i := 0; i < len(panels); i++ {
		for j := i + 1; j < len(panels); j++ {
			if s.panelsAgree(panels[i], panels[j]) {
				consensusCount++
			}
		}
	}

	totalPairs := len(panels) * (len(panels) - 1) / 2
	if totalPairs == 0 {
		return "Insufficient data"
	}

	consensusRate := float64(consensusCount) / float64(totalPairs)
	if consensusRate > 0.7 {
		return "Strong consensus across panels"
	} else if consensusRate > 0.4 {
		return "Moderate consensus with some variation"
	}
	return "Limited consensus - significant variation between panels"
}

// analyzeDisagreements identifies disagreements between panels.
func (s *SecondOpinionMode) analyzeDisagreements(panels []PanelResult) []string {
	var disagreements []string

	// Compare diagnoses across panels
	for i := 0; i < len(panels); i++ {
		for j := i + 1; j < len(panels); j++ {
			if !s.panelsAgree(panels[i], panels[j]) {
				disagreements = append(disagreements,
					fmt.Sprintf("Panels %s and %s have different diagnostic approaches",
						panels[i].PanelName, panels[j].PanelName))
			}
		}
	}

	return disagreements
}

// synthesizeOpinions creates a synthesis of all panel opinions.
func (s *SecondOpinionMode) synthesizeOpinions(panels []PanelResult) string {
	if len(panels) == 0 {
		return "No panel results available"
	}

	synthesis := "## Second Opinion Synthesis\n\n"
	synthesis += fmt.Sprintf("**Number of Panels:** %d\n\n", len(panels))

	for _, panel := range panels {
		synthesis += fmt.Sprintf("### %s\n", panel.PanelName)
		synthesis += fmt.Sprintf("**Confidence:** %s\n\n", panel.Confidence)

		if len(panel.Diagnosis) > 0 {
			synthesis += "**Diagnosis:**\n"
			for _, dx := range panel.Diagnosis {
				synthesis += fmt.Sprintf("- %s\n", dx)
			}
			synthesis += "\n"
		}

		if len(panel.Recommendations) > 0 {
			synthesis += "**Recommendations:**\n"
			for _, rec := range panel.Recommendations {
				synthesis += fmt.Sprintf("- %s\n", rec)
			}
			synthesis += "\n"
		}
	}

	return synthesis
}

// panelsAgree checks if two panels have similar conclusions.
func (s *SecondOpinionMode) panelsAgree(p1, p2 PanelResult) bool {
	// Simple agreement check based on diagnosis overlap
	if len(p1.Diagnosis) == 0 || len(p2.Diagnosis) == 0 {
		return false
	}

	// Check for any overlap in diagnoses
	for _, dx1 := range p1.Diagnosis {
		for _, dx2 := range p2.Diagnosis {
			if strings.Contains(strings.ToLower(dx1), strings.ToLower(dx2)) ||
				strings.Contains(strings.ToLower(dx2), strings.ToLower(dx1)) {
				return true
			}
		}
	}

	return false
}

// Helper functions
func containsDiagnosticKeywords(text string) bool {
	keywords := []string{"diagnosis", "diagnose", "condition", "disease", "disorder", "syndrome"}
	textLower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(textLower, keyword) {
			return true
		}
	}
	return false
}

func containsRecommendationKeywords(text string) bool {
	keywords := []string{"recommend", "suggest", "advise", "should", "consider", "treatment", "management"}
	textLower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(textLower, keyword) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
