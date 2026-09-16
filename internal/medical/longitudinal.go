package medical

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

const longitudinalDir = "data/longitudinal"

// sanitizePatientID validates a patient ID so it can never escape the
// longitudinal storage directory. Accepts UUIDs and simple alphanumeric IDs
// with dashes/underscores; rejects path separators, dots, and empty values.
func sanitizePatientID(patientID string) error {
	if patientID == "" || len(patientID) > 64 {
		return fmt.Errorf("invalid patient_id: must be 1-64 characters")
	}
	for _, c := range patientID {
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return fmt.Errorf("invalid patient_id: only letters, digits, dashes, underscores allowed")
		}
	}
	return nil
}

// longitudinalFilePath builds and validates the storage path for a patient.
func longitudinalFilePath(patientID string) (string, error) {
	if err := sanitizePatientID(patientID); err != nil {
		return "", err
	}
	return filepath.Join(longitudinalDir, patientID+".json"), nil
}

// SaveLongitudinalVisit saves a visit record for a patient.
// Loads existing case or creates a new one, appends the visit, and persists.
func SaveLongitudinalVisit(patientID string, visit models.LongitudinalVisit) error {
	if err := sanitizePatientID(patientID); err != nil {
		return err
	}
	if err := ensureLongitudinalDir(); err != nil {
		return err
	}

	caseData, err := LoadLongitudinalCase(patientID)
	if err != nil {
		// If file doesn't exist, create a new case
		caseData = &models.LongitudinalCase{
			PatientID:    patientID,
			VisitHistory: []models.LongitudinalVisit{},
			Trends:       []string{},
			Alerts:       []string{},
		}
	}

	caseData.VisitHistory = append(caseData.VisitHistory, visit)

	data, err := json.MarshalIndent(caseData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal longitudinal case: %w", err)
	}

	filePath, err := longitudinalFilePath(patientID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write longitudinal file: %w", err)
	}

	return nil
}

// LoadLongitudinalCase loads a patient's full longitudinal history.
func LoadLongitudinalCase(patientID string) (*models.LongitudinalCase, error) {
	filePath, err := longitudinalFilePath(patientID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read longitudinal file: %w", err)
	}

	var caseData models.LongitudinalCase
	if err := json.Unmarshal(data, &caseData); err != nil {
		return nil, fmt.Errorf("unmarshal longitudinal case: %w", err)
	}

	return &caseData, nil
}

// AnalyzeTrends uses LLM to identify trends across visits.
// Returns (trends, alerts, error).
func AnalyzeTrends(ctx context.Context, client *llm.Client, caseData *models.LongitudinalCase) ([]string, []string, error) {
	if len(caseData.VisitHistory) < 2 {
		return []string{"Insufficient visit history for trend analysis (need ≥ 2 visits)"}, []string{}, nil
	}

	prompt := buildTrendPrompt(caseData)

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a clinician analyzing a patient's longitudinal medical record.
Identify trends (improving, worsening, or stable patterns) across visits and flag any concerning changes.
Return ONLY a valid JSON object with two keys:
- "trends": array of strings describing observed trends
- "alerts": array of strings describing concerning changes that need attention
No markdown, no code fences, no extra text.`,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := client.Complete(ctx, messages, 0.2, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM call failed: %w", err)
	}

	trends, alerts, err := parseTrendResponse(response)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	return trends, alerts, nil
}

// buildTrendPrompt constructs the prompt from the longitudinal case data.
func buildTrendPrompt(caseData *models.LongitudinalCase) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Patient %s — Visit History (%d visits)\n\n", caseData.PatientID, len(caseData.VisitHistory)))

	for i, visit := range caseData.VisitHistory {
		sb.WriteString(fmt.Sprintf("### Visit %d — %s\n", i+1, visit.Date.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("- Debate ID: %s\n", visit.DebateID))
		sb.WriteString(fmt.Sprintf("- Primary Diagnosis: %s\n", visit.PrimaryDx))
		sb.WriteString(fmt.Sprintf("- Risk Score: %d/100\n", visit.RiskScore))

		if len(visit.KeyFindings) > 0 {
			sb.WriteString("- Key Findings:\n")
			for _, f := range visit.KeyFindings {
				sb.WriteString(fmt.Sprintf("  - %s\n", f))
			}
		}

		if len(visit.Changes) > 0 {
			sb.WriteString("- Changes from Prior Visit:\n")
			for _, c := range visit.Changes {
				sb.WriteString(fmt.Sprintf("  - %s\n", c))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Analyze the trends and identify any alerts. Return JSON.")
	return sb.String()
}

// parseTrendResponse extracts trends and alerts from the LLM JSON response.
func parseTrendResponse(response string) ([]string, []string, error) {
	response = strings.TrimSpace(response)

	// Strip markdown code fences if present
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

	// Find JSON object boundaries
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")
	if startIdx < 0 || endIdx < 0 || endIdx <= startIdx {
		return nil, nil, fmt.Errorf("no JSON object found in response")
	}
	jsonStr := response[startIdx : endIdx+1]

	var result struct {
		Trends []string `json:"trends"`
		Alerts []string `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, nil, fmt.Errorf("JSON unmarshal: %w", err)
	}

	return result.Trends, result.Alerts, nil
}

// ensureLongitudinalDir creates the data directory if it doesn't exist.
func ensureLongitudinalDir() error {
	return os.MkdirAll(longitudinalDir, 0755)
}
