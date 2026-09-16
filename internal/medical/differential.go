package medical

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/kampong/debate/internal/llm"
	"github.com/kampong/debate/internal/models"
)

// GenerateDifferentialRanking uses LLM to create a ranked differential diagnosis
// from the medical case data. Returns diagnoses sorted by probability descending.
func GenerateDifferentialRanking(ctx context.Context, client *llm.Client, caseData *MedicalCase) ([]models.DifferentialRanking, error) {
	prompt := buildDifferentialPrompt(caseData)

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are a senior diagnostician with expertise across all medical specialties.
Given a clinical case, generate a ranked differential diagnosis list.
Return ONLY a valid JSON array — no markdown, no explanation, no code fences.
Each element must have:
- "diagnosis": string (diagnosis name)
- "icd10": string (ICD-10 code, or empty string if unknown)
- "probability": number (0.0–1.0, sum of all probabilities should be ≤ 1.0)
- "evidence": string (brief evidence summary)
- "supporting": array of strings (findings that support this diagnosis)
- "against": array of strings (findings that argue against)
- "next_test": string (recommended next test to confirm or rule out, or empty)
- "urgency": string ("routine", "urgent", or "emergency")
Sort by probability descending. Include 3–8 diagnoses.`,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	response, err := client.Complete(ctx, messages, 0.2, 2048)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	rankings, err := parseDifferentialResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Sort by probability descending
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Probability > rankings[j].Probability
	})

	return rankings, nil
}

// buildDifferentialPrompt constructs the clinical prompt from case data.
func buildDifferentialPrompt(caseData *MedicalCase) string {
	var sb strings.Builder

	sb.WriteString("## Clinical Case\n\n")

	// Demographics
	sb.WriteString(fmt.Sprintf("**Patient**: %d-year-old %s", caseData.PatientInfo.Age, caseData.PatientInfo.Sex))
	if caseData.PatientInfo.Ethnicity != "" {
		sb.WriteString(fmt.Sprintf(", %s", caseData.PatientInfo.Ethnicity))
	}
	sb.WriteString("\n")
	if caseData.PatientInfo.BMI > 0 {
		sb.WriteString(fmt.Sprintf("**BMI**: %.1f | **Weight**: %.1f kg | **Height**: %.1f cm\n",
			caseData.PatientInfo.BMI, caseData.PatientInfo.Weight, caseData.PatientInfo.Height))
	}
	if caseData.PatientInfo.SmokingStatus != "" && caseData.PatientInfo.SmokingStatus != "never" {
		sb.WriteString(fmt.Sprintf("**Smoking**: %s\n", caseData.PatientInfo.SmokingStatus))
	}
	if caseData.PatientInfo.AlcoholUse != "" && caseData.PatientInfo.AlcoholUse != "none" {
		sb.WriteString(fmt.Sprintf("**Alcohol**: %s\n", caseData.PatientInfo.AlcoholUse))
	}

	// Symptoms
	if len(caseData.Symptoms) > 0 {
		sb.WriteString("\n### Symptoms\n")
		for _, s := range caseData.Symptoms {
			sb.WriteString(fmt.Sprintf("- **%s** (severity %d/10, onset: %s, duration: %s", s.Name, s.Severity, s.Onset, s.Duration))
			if s.Location != "" {
				sb.WriteString(fmt.Sprintf(", location: %s", s.Location))
			}
			if s.Frequency != "" {
				sb.WriteString(fmt.Sprintf(", frequency: %s", s.Frequency))
			}
			sb.WriteString(")\n")
			if s.Aggravating != "" {
				sb.WriteString(fmt.Sprintf("  - Aggravating: %s\n", s.Aggravating))
			}
			if s.Alleviating != "" {
				sb.WriteString(fmt.Sprintf("  - Alleviating: %s\n", s.Alleviating))
			}
			if s.Associated != "" {
				sb.WriteString(fmt.Sprintf("  - Associated: %s\n", s.Associated))
			}
		}
	}

	// Vital signs
	vs := caseData.VitalSigns
	if vs.Temperature > 0 || vs.HeartRate > 0 || vs.BloodPressure != "" || vs.SpO2 > 0 {
		sb.WriteString("\n### Vital Signs\n")
		if vs.Temperature > 0 {
			sb.WriteString(fmt.Sprintf("- Temperature: %.1f°C\n", vs.Temperature))
		}
		if vs.HeartRate > 0 {
			sb.WriteString(fmt.Sprintf("- Heart Rate: %d bpm\n", vs.HeartRate))
		}
		if vs.BloodPressure != "" {
			sb.WriteString(fmt.Sprintf("- Blood Pressure: %s\n", vs.BloodPressure))
		}
		if vs.RespiratoryRate > 0 {
			sb.WriteString(fmt.Sprintf("- Respiratory Rate: %d /min\n", vs.RespiratoryRate))
		}
		if vs.SpO2 > 0 {
			sb.WriteString(fmt.Sprintf("- SpO2: %d%%\n", vs.SpO2))
		}
		if vs.PainScore > 0 {
			sb.WriteString(fmt.Sprintf("- Pain Score: %d/10\n", vs.PainScore))
		}
	}

	// Labs
	if len(caseData.LabResults) > 0 {
		sb.WriteString("\n### Lab Results\n")
		for _, lab := range caseData.LabResults {
			flag := ""
			if lab.Flag == "high" || lab.Flag == "critical_high" {
				flag = " ↑"
			} else if lab.Flag == "low" || lab.Flag == "critical_low" {
				flag = " ↓"
			}
			sb.WriteString(fmt.Sprintf("- %s: %.2f %s%s (ref: %.1f–%.1f)\n",
				lab.TestName, lab.Value, lab.Unit, flag, lab.ReferenceMin, lab.ReferenceMax))
		}
	}

	// Imaging
	if len(caseData.ImagingResults) > 0 {
		sb.WriteString("\n### Imaging\n")
		for _, img := range caseData.ImagingResults {
			sb.WriteString(fmt.Sprintf("- **%s %s**: %s\n", img.Modality, img.BodyPart, img.Finding))
		}
	}

	// Medical history
	if len(caseData.MedicalHistory.PastConditions) > 0 {
		sb.WriteString("\n### Past Medical History\n")
		for _, cond := range caseData.MedicalHistory.PastConditions {
			sb.WriteString(fmt.Sprintf("- %s\n", cond))
		}
	}
	if len(caseData.MedicalHistory.FamilyHistory) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Family History\n- %s\n", strings.Join(caseData.MedicalHistory.FamilyHistory, ", ")))
	}

	// Current medications
	if len(caseData.CurrentMeds) > 0 {
		sb.WriteString("\n### Current Medications\n")
		for _, med := range caseData.CurrentMeds {
			sb.WriteString(fmt.Sprintf("- %s %s %s\n", med.Name, med.Dose, med.Frequency))
		}
	}

	// Allergies
	if len(caseData.Allergies) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Allergies\n- %s\n", strings.Join(caseData.Allergies, ", ")))
	}

	// Red flags
	if len(caseData.RedFlags) > 0 {
		sb.WriteString("\n### Red Flags Identified\n")
		for _, rf := range caseData.RedFlags {
			sb.WriteString(fmt.Sprintf("- ⚠️ **%s** (%s): %s\n", rf.Symptom, rf.Severity, rf.Description))
		}
	}

	sb.WriteString("\nGenerate a ranked differential diagnosis as a JSON array.")
	return sb.String()
}

// parseDifferentialResponse extracts the JSON array from the LLM response.
func parseDifferentialResponse(response string) ([]models.DifferentialRanking, error) {
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

	// Find JSON array boundaries
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")
	if startIdx < 0 || endIdx < 0 || endIdx <= startIdx {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	jsonStr := response[startIdx : endIdx+1]

	var rankings []models.DifferentialRanking
	if err := json.Unmarshal([]byte(jsonStr), &rankings); err != nil {
		return nil, fmt.Errorf("JSON unmarshal: %w", err)
	}

	return rankings, nil
}
