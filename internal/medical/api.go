package medical

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kampong/debate/internal/debate"
	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

// MedicalAPI handles medical debate API endpoints.
type MedicalAPI struct {
	engine           *debate.Engine
	medEngine        *MedicalDebateEngine
	redFlags         *RedFlagDetector
	evidence         *EvidenceScorer
	drugChecker      *DrugInteractionChecker
	labInterpreter   *LabInterpreter
	imageAnalyzer    *MedicalImageAnalyzer
	guidelineMatcher *GuidelineMatcher
	icdCoder         *ICDCoder
	pubmed           *PubMedSearcher
	secondOpinion    *SecondOpinionMode
	literatureSearcher *LiteratureSearcher
	reportFormatter  *MedicalReportFormatter
	consentStore     map[string]*ConsentRecord // In production, use a database
	consentMu        sync.Mutex                // guards consentStore
}

// NewMedicalAPI creates a new medical API handler.
func NewMedicalAPI(engine *debate.Engine, llmClient *llm.Client) *MedicalAPI {
	medEngine := NewMedicalDebateEngine(engine)
	return &MedicalAPI{
		engine:           engine,
		medEngine:        medEngine,
		redFlags:         NewRedFlagDetector(),
		evidence:         NewEvidenceScorer(),
		drugChecker:      NewDrugInteractionChecker(),
		labInterpreter:   NewLabInterpreter(),
		imageAnalyzer:    NewMedicalImageAnalyzer(llmClient, "data/uploads"),
		guidelineMatcher: NewGuidelineMatcher(),
		icdCoder:         NewICDCoder(),
		pubmed:           NewPubMedSearcher(""),
		secondOpinion:    NewSecondOpinionMode(engine, medEngine),
		literatureSearcher: NewLiteratureSearcher(""), // No API key by default
		reportFormatter:  NewMedicalReportFormatter(),
		consentStore:     make(map[string]*ConsentRecord),
	}
}

// SetLLMClient updates the LLM client used by ALL medical components.
// Called when the user updates API config through the UI or when the server
// syncs credentials from providers.
func (m *MedicalAPI) SetLLMClient(client *llm.Client) {
	m.imageAnalyzer.client = client
	// Also update the debate engine's extractor if it uses a separate client
	if m.medEngine != nil && m.medEngine.engine != nil {
		m.medEngine.engine.SetLLMClient(client)
	}
}

// HasValidAPIKey checks if the medical API's LLM client has an API key.
func (m *MedicalAPI) HasValidAPIKey() bool {
	return m.imageAnalyzer.client != nil && m.imageAnalyzer.client.HasAPIKey()
}

// GatherMedicalKnowledge calls all medical knowledge modules and returns a formatted string
// for injection into the debate topic context.
func (m *MedicalAPI) GatherMedicalKnowledge(ctx context.Context, caseData *MedicalCase) string {
	var sb strings.Builder

	// 1. Clinical Guidelines
	guidelines := m.guidelineMatcher.MatchGuidelines(caseData)
	if len(guidelines) > 0 {
		sb.WriteString("## 📋 Clinical Guidelines\n")
		for _, g := range guidelines {
			sb.WriteString(fmt.Sprintf("### %s (%s, %d)\n", g.Title, g.Organization, g.Year))
			for _, rec := range g.Recommendations {
				sb.WriteString(fmt.Sprintf("- %s (Strength: %s, Evidence Level: %d)\n", rec.Statement, rec.Strength, rec.EvidenceLevel))
			}
			sb.WriteString("\n")
		}
	}

	// 2. Drug Interactions
	if len(caseData.CurrentMeds) > 0 {
		interactions := m.drugChecker.CheckInteractions(caseData.CurrentMeds, caseData.Allergies, caseData.MedicalHistory.PastConditions)
		if interactions != nil && (len(interactions.DrugDrugInteractions) > 0 || len(interactions.DrugAllergyInteractions) > 0 || len(interactions.DrugDiseaseInteractions) > 0) {
			sb.WriteString("## ⚠️ Drug Interaction Warnings\n")
			for _, di := range interactions.DrugDrugInteractions {
				sb.WriteString(fmt.Sprintf("- **%s + %s** (%s): %s\n", di.Drug1, di.Drug2, di.Severity, di.Description))
				if di.Recommendation != "" {
					sb.WriteString(fmt.Sprintf("  - Recommendation: %s\n", di.Recommendation))
				}
			}
			for _, ai := range interactions.DrugAllergyInteractions {
				sb.WriteString(fmt.Sprintf("- **ALLERGY: %s + %s** (%s): %s\n", ai.Allergen, ai.Drug, ai.Severity, ai.Reaction))
			}
			for _, di := range interactions.DrugDiseaseInteractions {
				sb.WriteString(fmt.Sprintf("- **CONTRAINDICATION: %s + %s** (%s): %s\n", di.Drug, di.Disease, di.Severity, di.Description))
				if di.Recommendation != "" {
					sb.WriteString(fmt.Sprintf("  - Recommendation: %s\n", di.Recommendation))
				}
			}
			sb.WriteString("\n")
		}
	}

	// 3. Lab Interpretations
	if len(caseData.LabResults) > 0 {
		interpretations := m.labInterpreter.InterpretLabResults(caseData.LabResults, caseData.PatientInfo.Age, caseData.PatientInfo.Sex)
		if len(interpretations) > 0 {
			sb.WriteString("## 🔬 Lab Interpretations\n")
			for _, interp := range interpretations {
				flag := ""
				if interp.Flag == "high" || interp.Flag == "critical_high" {
					flag = " ↑ HIGH"
				} else if interp.Flag == "low" || interp.Flag == "critical_low" {
					flag = " ↓ LOW"
				} else {
					flag = " ✓ Normal"
				}
				sb.WriteString(fmt.Sprintf("- **%s**: %.2f %s%s (Ref: %s)\n",
					interp.TestName, interp.Value, interp.Unit, flag, interp.ReferenceRange))
				if interp.ClinicalSignificance != "" {
					sb.WriteString(fmt.Sprintf("  - Significance: %s\n", interp.ClinicalSignificance))
				}
				if len(interp.PossibleCauses) > 0 {
					sb.WriteString(fmt.Sprintf("  - Possible causes: %s\n", strings.Join(interp.PossibleCauses, ", ")))
				}
				if len(interp.RecommendedActions) > 0 {
					sb.WriteString(fmt.Sprintf("  - Actions: %s\n", strings.Join(interp.RecommendedActions, ", ")))
				}
				if len(interp.FollowUpTests) > 0 {
					sb.WriteString(fmt.Sprintf("  - Follow-up tests: %s\n", strings.Join(interp.FollowUpTests, ", ")))
				}
			}
			sb.WriteString("\n")
		}
	}

	// 4. ICD-10 Codes
	icdCodes := m.icdCoder.MatchCodes(caseData)
	if len(icdCodes) > 0 {
		sb.WriteString("## 🏷️ ICD-10 Code Matches\n")
		for _, code := range icdCodes {
			sb.WriteString(fmt.Sprintf("- **%s** — %s\n", code.Code, code.Description))
		}
		sb.WriteString("\n")
	}

	// 5. PubMed Literature Search (async-safe with timeout)
	if m.pubmed != nil {
		searchQuery := buildPubMedQuery(caseData)
		if searchQuery != "" {
			searchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()

			results, err := m.pubmed.Search(searchCtx, searchQuery, 5)
			if err == nil && results != nil && len(results.Articles) > 0 {
				sb.WriteString("## 📚 Medical Literature (PubMed)\n")
				for _, article := range results.Articles {
					sb.WriteString(fmt.Sprintf("- **%s** (%s) — %s\n", article.Title, article.Journal, article.PubDate))
					if article.Abstract != "" {
						abstract := article.Abstract
						if len(abstract) > 200 {
							abstract = abstract[:200] + "..."
						}
						sb.WriteString(fmt.Sprintf("  - %s\n", abstract))
					}
					if article.PMID != "" {
						sb.WriteString(fmt.Sprintf("  - PMID: %s | Evidence Level: %d\n", article.PMID, article.EvidenceLevel))
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// buildPubMedQuery creates a search query from the medical case data.
func buildPubMedQuery(caseData *MedicalCase) string {
	var terms []string

	// Use primary symptom as main search term
	if len(caseData.Symptoms) > 0 {
		terms = append(terms, caseData.Symptoms[0].Name)
	}

	// Add relevant conditions from history
	for _, cond := range caseData.MedicalHistory.PastConditions {
		terms = append(terms, cond)
	}

	// Add abnormal lab findings
	for _, lab := range caseData.LabResults {
		if lab.Flag == "high" || lab.Flag == "low" {
			terms = append(terms, lab.TestName+" abnormal")
		}
	}

	if len(terms) == 0 {
		return ""
	}

	// Build a focused query: primary symptom + "diagnosis OR treatment"
	query := terms[0]
	if len(terms) > 1 {
		query += " AND (" + strings.Join(terms[1:], " OR ") + ")"
	}
	query += " AND (diagnosis OR treatment OR guidelines)"

	return query
}

// HandleConsentText returns the informed consent text.
func (m *MedicalAPI) HandleConsentText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"consent_text": InformedConsent,
	})
}

// HandleConsent records user consent.
func (m *MedicalAPI) HandleConsent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Accepted          bool   `json:"accepted"`
		Timestamp         string `json:"timestamp"`
		MedicalProfessional bool  `json:"medical_professional"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !req.Accepted {
		http.Error(w, "Consent not accepted", http.StatusBadRequest)
		return
	}

	// Generate a simple user ID (in production, use proper auth)
	userID := fmt.Sprintf("user_%d", time.Now().UnixNano())

	consent := &ConsentRecord{
		UserID:              userID,
		AcceptedAt:          req.Timestamp,
		IPAddress:           r.RemoteAddr,
		MedicalProfessional: req.MedicalProfessional,
		Version:             "1.0",
	}

	m.consentMu.Lock()
	m.consentStore[userID] = consent
	m.consentMu.Unlock()

	// Remember the user with an HttpOnly cookie so subsequent requests
	// to protected endpoints carry the consent ID automatically.
	http.SetCookie(w, &http.Cookie{
		Name:     consentCookieName,
		Value:    userID,
		Path:     "/",
		MaxAge:   180 * 24 * 3600, // 6 months — matches the consent re-accept period
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user_id": userID,
	})
}

const consentCookieName = "kampong_med_consent"

// consentUserID extracts the consent user ID from a cookie, header, or query param.
func consentUserID(r *http.Request) string {
	if c, err := r.Cookie(consentCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("X-User-ID"); h != "" {
		return h
	}
	return r.URL.Query().Get("user_id")
}

// requireConsent enforces the informed-consent gate on protected medical
// endpoints. Writes a 403 JSON response and returns false when the user has
// not accepted consent (or the record is missing/expired).
func (m *MedicalAPI) requireConsent(w http.ResponseWriter, r *http.Request) bool {
	userID := consentUserID(r)
	if userID == "" {
		http.Error(w, `{"error":"informed consent required — POST /api/medical/consent first"}`, http.StatusForbidden)
		return false
	}
	m.consentMu.Lock()
	record := m.consentStore[userID]
	m.consentMu.Unlock()
	if !ValidateConsent(record) {
		http.Error(w, `{"error":"informed consent not accepted or expired — POST /api/medical/consent first"}`, http.StatusForbidden)
		return false
	}
	return true
}

// HandleStartDebate starts a medical debate from structured case data.
func (m *MedicalAPI) HandleStartDebate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.requireConsent(w, r) {
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Detect red flags
	caseData.RedFlags = m.redFlags.DetectRedFlags(&caseData)

	// Generate a topic summary from the case
	topic := m.generateTopicFromCase(&caseData)

	// Create a debate session
	session := &models.DebateSession{
		ID:          fmt.Sprintf("med_%d", time.Now().UnixNano()),
		Topic:       topic,
		Mode:        models.ModeDeep, // Use deep mode for medical cases
		Status:      models.StatusAnalyzing,
		Agents:      []models.Agent{},
		Transcript:  []models.TranscriptEntry{},
		CreatedAt:   time.Now(),
	}
	session.Graph.Nodes = []models.GraphNode{}
	session.Graph.Edges = []models.GraphEdge{}

	// Create medical agents based on the debate mode
	agents := m.createMedicalAgents(caseData)
	session.Agents = agents

	// Gather medical knowledge from all modules (guidelines, drug interactions, labs, ICD, PubMed)
	ctx := context.Background()

	// Validate API key before making any LLM calls
	if !m.HasValidAPIKey() {
		http.Error(w, `{"error":"No API key configured. Please add your API key in Settings or add a Provider."}`, http.StatusBadRequest)
		return
	}

	medicalKnowledge := m.GatherMedicalKnowledge(ctx, &caseData)

	// Store the medical case data + knowledge in the session topic
	session.Topic = topic + "\n\n[Medical Case Data: " + encodeCaseData(&caseData) + "]"
	if medicalKnowledge != "" {
		session.Topic += "\n\n" + medicalKnowledge
	}

	// Start the medical debate
	go m.medEngine.RunMedicalDebate(ctx, session, &caseData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"debate_id":         session.ID,
		"red_flags":         caseData.RedFlags,
		"topic":             topic,
		"agent_count":       len(agents),
		"medical_knowledge": medicalKnowledge,
	})
}

// generateTopicFromCase creates a debate topic from the medical case.
func (m *MedicalAPI) generateTopicFromCase(caseData *MedicalCase) string {
	topic := fmt.Sprintf("Medical Case: %d-year-old %s patient", 
		caseData.PatientInfo.Age, 
		caseData.PatientInfo.Sex)

	if len(caseData.Symptoms) > 0 {
		topic += " presenting with: "
		for i, symptom := range caseData.Symptoms {
			if i > 0 {
				topic += ", "
			}
			topic += symptom.Name
			if symptom.Severity > 0 {
				topic += fmt.Sprintf(" (severity %d/10)", symptom.Severity)
			}
		}
	}

	return topic
}

// createMedicalAgents creates the panel of medical agents.
func (m *MedicalAPI) createMedicalAgents(caseData MedicalCase) []models.Agent {
	configs := GetDefaultMedicalPanel()
	agents := make([]models.Agent, len(configs))

	for i, config := range configs {
		agents[i] = models.Agent{
			ID:        fmt.Sprintf("med_agent_%d", i),
			Name:      config.Name,
			Role:      string(config.Role),
			Expertise: config.Expertise,
			RoleType:  models.AgentRoleType(config.Role),
			Icon:      config.Icon,
		}
	}

	return agents
}

// encodeCaseData encodes the case data as a JSON string for storage.
func encodeCaseData(caseData *MedicalCase) string {
	data, err := json.Marshal(caseData)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// HandleGetCase retrieves a medical case by debate ID.
func (m *MedicalAPI) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	debateID := r.URL.Query().Get("debate_id")
	if debateID == "" {
		http.Error(w, "debate_id required", http.StatusBadRequest)
		return
	}

	// In production, retrieve from database
	// For now, return a placeholder
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"debate_id": debateID,
		"status":    "not_found",
	})
}

// HandleRedFlagCheck checks for red flags in a case without starting a debate.
func (m *MedicalAPI) HandleRedFlagCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	redFlags := m.redFlags.DetectRedFlags(&caseData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"red_flags": redFlags,
		"count":     len(redFlags),
	})
}

// HandleDrugInteractionCheck checks for drug interactions.
func (m *MedicalAPI) HandleDrugInteractionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Medications []Medication `json:"medications"`
		Allergies   []string     `json:"allergies"`
		Conditions  []string     `json:"conditions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result := m.drugChecker.CheckInteractions(req.Medications, req.Allergies, req.Conditions)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleLabInterpretation interprets lab results.
func (m *MedicalAPI) HandleLabInterpretation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		LabResults []LabResult `json:"lab_results"`
		Age        int         `json:"age"`
		Sex        string      `json:"sex"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	interpretations := m.labInterpreter.InterpretLabResults(req.LabResults, req.Age, req.Sex)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"interpretations": interpretations,
		"count":          len(interpretations),
	})
}

// HandleExtractLabImage extracts structured lab values from an uploaded lab report image.
func (m *MedicalAPI) HandleExtractLabImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Accept multipart file upload
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, `{"error":"failed to parse multipart form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file is required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"failed to read file"}`, http.StatusBadRequest)
		return
	}

	// Determine MIME type
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
	}

	// Convert to base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(fileData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64Data)

	// Build specialized lab extraction prompt
	prompt := `Analyze this lab report image and extract ALL laboratory test results.

For each test found, extract:
- Test name (e.g., "Hemoglobin", "Glucose", "Creatinine", "WBC", "Platelets", etc.)
- Value (numeric)
- Unit (e.g., "g/dL", "mg/dL", "x10^3/uL", "%", "mEq/L", "U/L")
- Reference range minimum (numeric, if shown)
- Reference range maximum (numeric, if shown)
- Flag: "high", "low", or "normal" (based on the report's markings like H, L, ↑, ↓, or by comparing value to reference range)

Return the results as a JSON array in this exact format:
[
  {"test_name": "Hemoglobin", "value": 14.5, "unit": "g/dL", "reference_min": 12.0, "reference_max": 17.5, "flag": "normal"},
  {"test_name": "Glucose", "value": 180, "unit": "mg/dL", "reference_min": 70, "reference_max": 100, "flag": "high"}
]

IMPORTANT:
- Return ONLY the JSON array, no other text
- Extract ALL tests visible in the image
- Use standard test names in English
- If a reference range is not shown, use 0 for min and max
- If the value cannot be determined, skip that test`

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a medical laboratory data extraction specialist. Your job is to accurately read lab report images and extract structured test results. Return only valid JSON.`,
		},
		{
			Role: "user",
			Content: []llm.ContentPart{
				{Type: "text", Text: prompt},
				{Type: "image_url", ImageURL: &llm.ImageURL{URL: dataURL}},
			},
		},
	}

	ctx := r.Context()
	response, err := m.imageAnalyzer.client.Complete(ctx, messages, 0.1, 4096)
	if err != nil {
		http.Error(w, `{"error":"failed to analyze image: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// Parse the JSON array from the response
	labs := parseLabExtractionResponse(response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"lab_results": labs,
		"count":       len(labs),
		"raw_response": response,
	})
}

// parseLabExtractionResponse extracts lab results from the LLM's JSON response.
func parseLabExtractionResponse(response string) []LabResult {
	var labs []LabResult

	// Try to find JSON array in the response
	response = strings.TrimSpace(response)

	// Strip markdown code blocks if present
	if strings.Contains(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		if len(jsonLines) > 0 {
			response = strings.Join(jsonLines, "\n")
		}
	}

	// Find the JSON array boundaries
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")
	if startIdx < 0 || endIdx < 0 || endIdx <= startIdx {
		return labs
	}
	jsonStr := response[startIdx : endIdx+1]

	// Parse into generic structure first
	var rawResults []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rawResults); err != nil {
		return labs
	}

	for _, raw := range rawResults {
		lab := LabResult{}

		if name, ok := raw["test_name"].(string); ok {
			lab.TestName = name
		}
		if name, ok := raw["name"].(string); ok && lab.TestName == "" {
			lab.TestName = name
		}

		if val, ok := raw["value"].(float64); ok {
			lab.Value = val
		} else if val, ok := raw["value"].(string); ok {
			fmt.Sscanf(val, "%f", &lab.Value)
		}

		if unit, ok := raw["unit"].(string); ok {
			lab.Unit = unit
		}

		if refMin, ok := raw["reference_min"].(float64); ok {
			lab.ReferenceMin = refMin
		} else if refMin, ok := raw["ref_min"].(float64); ok {
			lab.ReferenceMin = refMin
		}

		if refMax, ok := raw["reference_max"].(float64); ok {
			lab.ReferenceMax = refMax
		} else if refMax, ok := raw["ref_max"].(float64); ok {
			lab.ReferenceMax = refMax
		}

		if flag, ok := raw["flag"].(string); ok {
			lab.Flag = flag
		} else {
			// Derive flag from value and reference range
			if lab.Value > 0 && lab.ReferenceMax > 0 {
				if lab.Value > lab.ReferenceMax {
					lab.Flag = "high"
				} else if lab.Value < lab.ReferenceMin {
					lab.Flag = "low"
				} else {
					lab.Flag = "normal"
				}
			} else {
				lab.Flag = "normal"
			}
		}

		if lab.TestName != "" {
			labs = append(labs, lab)
		}
	}

	return labs
}

// HandleImageAnalysis analyzes a medical image.
func (m *MedicalAPI) HandleImageAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.requireConsent(w, r) {
		return
	}

	var req struct {
		ImageID         string `json:"image_id"`     // ID of a previously uploaded file
		Base64Data      string `json:"base64_data"`  // Inline base64 data URL (alternative to image_id)
		Modality        string `json:"modality"`
		BodyPart        string `json:"body_part"`
		ClinicalContext string `json:"clinical_context"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// SECURITY: Never trust a client-supplied file path. Only allow:
	// 1. An image_id that resolves to a previously uploaded file in the uploads dir, or
	// 2. Inline base64 data.
	var image *MedicalImage
	if req.Base64Data != "" {
		image = &MedicalImage{
			ID:         req.ImageID,
			Base64Data: req.Base64Data,
			Modality:   req.Modality,
			BodyPart:   req.BodyPart,
		}
	} else if req.ImageID != "" {
		filePath, err := m.imageAnalyzer.ResolveUploadedFile(req.ImageID)
		if err != nil {
			http.Error(w, "Image not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		image = &MedicalImage{
			ID:       req.ImageID,
			FilePath: filePath,
			Modality: req.Modality,
			BodyPart: req.BodyPart,
		}
	} else {
		http.Error(w, "image_id or base64_data is required", http.StatusBadRequest)
		return
	}

	if m.imageAnalyzer.client == nil {
		http.Error(w, "No LLM client configured", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	result, err := m.imageAnalyzer.AnalyzeImage(ctx, image, req.ClinicalContext)
	if err != nil {
		http.Error(w, "Failed to analyze image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleGuidelineMatch finds relevant clinical guidelines for a case.
func (m *MedicalAPI) HandleGuidelineMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	guidelines := m.guidelineMatcher.MatchGuidelines(&caseData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"guidelines": guidelines,
		"count":      len(guidelines),
	})
}

// HandleICDCodeMatch finds relevant ICD codes for a case.
func (m *MedicalAPI) HandleICDCodeMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	codes := m.icdCoder.MatchCodes(&caseData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"codes": codes,
		"count": len(codes),
	})
}

// HandleSecondOpinion runs multiple independent panels on a case.
func (m *MedicalAPI) HandleSecondOpinion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.requireConsent(w, r) {
		return
	}

	var req struct {
		CaseData  MedicalCase `json:"case_data"`
		NumPanels int         `json:"num_panels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NumPanels < 2 {
		req.NumPanels = 2
	}

	ctx := r.Context()
	result, err := m.secondOpinion.RunSecondOpinion(ctx, &req.CaseData, req.NumPanels)
	if err != nil {
		http.Error(w, "Failed to run second opinion: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleLiteratureSearch performs a real-time medical literature search.
func (m *MedicalAPI) HandleLiteratureSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query   string        `json:"query"`
		Options SearchOptions `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Options.MaxResults == 0 {
		req.Options.MaxResults = 10
	}

	ctx := r.Context()
	result, err := m.literatureSearcher.SearchLiterature(ctx, req.Query, req.Options)
	if err != nil {
		http.Error(w, "Failed to search literature: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleGenerateReport generates a formatted medical report.
func (m *MedicalAPI) HandleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CaseData     MedicalCase          `json:"case_data"`
		DebateResult *MedicalDebateResult `json:"debate_result"`
		Format       string               `json:"format"` // soap, discharge, consultation, full, html
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var report string
	var contentType string

	switch req.Format {
	case "soap":
		soap := m.reportFormatter.FormatSOAPNote(&req.CaseData, req.DebateResult)
		report = fmt.Sprintf("SUBJECTIVE:\n%s\n\nOBJECTIVE:\n%s\n\nASSESSMENT:\n%s\n\nPLAN:\n%s",
			soap.Subjective, soap.Objective, soap.Assessment, soap.Plan)
		contentType = "text/plain"
	case "discharge":
		discharge := m.reportFormatter.FormatDischargeSummary(&req.CaseData, req.DebateResult)
		report = fmt.Sprintf("ADMISSION DATE: %s\nDISCHARGE DATE: %s\n\nADMISSION DIAGNOSIS: %s\nDISCHARGE DIAGNOSIS: %s\n\nHOSPITAL COURSE:\n%s\n\nDISCHARGE CONDITION: %s\n\nDISCHARGE MEDICATIONS:\n%s\n\nFOLLOW-UP INSTRUCTIONS:\n%s",
			discharge.AdmissionDate, discharge.DischargeDate,
			discharge.AdmissionDiagnosis, discharge.DischargeDiagnosis,
			discharge.HospitalCourse, discharge.DischargeCondition,
			strings.Join(discharge.DischargeMedications, "\n"),
			discharge.FollowUpInstructions)
		contentType = "text/plain"
	case "consultation":
		consult := m.reportFormatter.FormatConsultationReport(&req.CaseData, req.DebateResult, "Medical Consultation Service")
		report = fmt.Sprintf("CONSULTING SERVICE: %s\nDATE: %s\n\nREASON FOR CONSULTATION: %s\n\nHPI:\n%s\n\nPMH: %s\n\nLABS: %s\n\nIMPRESSION:\n%s\n\nRECOMMENDATIONS:\n%s",
			consult.ConsultingService, consult.DateOfConsultation,
			consult.ReasonForConsultation, consult.HistoryOfPresentIllness,
			consult.PastMedicalHistory, consult.LaboratoryData,
			consult.Impression, strings.Join(consult.Recommendations, "\n"))
		contentType = "text/plain"
	case "html":
		markdown := m.reportFormatter.FormatMedicalReport(&req.CaseData, req.DebateResult)
		report = m.reportFormatter.ExportToHTML(markdown)
		contentType = "text/html"
	default: // "full" or default
		report = m.reportFormatter.FormatMedicalReport(&req.CaseData, req.DebateResult)
		contentType = "text/markdown"
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	w.Write([]byte(report))
}

// HandleDifferentialRanking generates a ranked differential diagnosis from case data.
func (m *MedicalAPI) HandleDifferentialRanking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !m.HasValidAPIKey() {
		http.Error(w, `{"error":"No API key configured. Please add your API key in Settings or add a Provider."}`, http.StatusBadRequest)
		return
	}

	// Detect red flags if not already present
	if len(caseData.RedFlags) == 0 {
		caseData.RedFlags = m.redFlags.DetectRedFlags(&caseData)
	}

	ctx := r.Context()
	rankings, err := GenerateDifferentialRanking(ctx, m.imageAnalyzer.client, &caseData)
	if err != nil {
		http.Error(w, "Failed to generate differential ranking: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rankings": rankings,
		"count":    len(rankings),
	})
}

// HandleRiskScore computes a dynamic composite risk score from case data.
func (m *MedicalAPI) HandleRiskScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var caseData MedicalCase
	if err := json.NewDecoder(r.Body).Decode(&caseData); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Run all risk component analyses
	redFlags := m.redFlags.DetectRedFlags(&caseData)

	var interactions *InteractionCheckResult
	if len(caseData.CurrentMeds) > 0 {
		interactions = m.drugChecker.CheckInteractions(caseData.CurrentMeds, caseData.Allergies, caseData.MedicalHistory.PastConditions)
	}

	var labInterps []LabInterpretation
	if len(caseData.LabResults) > 0 {
		interps := m.labInterpreter.InterpretLabResults(caseData.LabResults, caseData.PatientInfo.Age, caseData.PatientInfo.Sex)
		labInterps = make([]LabInterpretation, len(interps))
		for i, interp := range interps {
			labInterps[i] = *interp
		}
	}

	riskScore := ComputeDynamicRiskScore(&caseData, redFlags, interactions, labInterps)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(riskScore)
}

// HandleLongitudinalSave saves a visit record for a patient.
func (m *MedicalAPI) HandleLongitudinalSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.requireConsent(w, r) {
		return
	}

	var req struct {
		PatientID string                  `json:"patient_id"`
		Visit     models.LongitudinalVisit `json:"visit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.PatientID == "" {
		http.Error(w, "patient_id is required", http.StatusBadRequest)
		return
	}

	if err := SaveLongitudinalVisit(req.PatientID, req.Visit); err != nil {
		http.Error(w, "Failed to save visit: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"patient_id": req.PatientID,
	})
}

// HandleLongitudinalLoad loads a patient's full longitudinal history.
func (m *MedicalAPI) HandleLongitudinalLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !m.requireConsent(w, r) {
		return
	}

	patientID := r.URL.Query().Get("patient_id")
	if patientID == "" {
		http.Error(w, "patient_id query parameter is required", http.StatusBadRequest)
		return
	}

	caseData, err := LoadLongitudinalCase(patientID)
	if err != nil {
		http.Error(w, "Failed to load longitudinal case: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(caseData)
}
