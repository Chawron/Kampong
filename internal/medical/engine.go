package medical

import (
	"context"
	"fmt"
	"log"

	"github.com/kampong/debate/internal/debate"
	"github.com/kampong/debate/internal/models"
)

// MedicalDebateEngine wraps the standard debate engine with medical-specific logic.
type MedicalDebateEngine struct {
	engine      *debate.Engine
	redFlags    *RedFlagDetector
	evidence    *EvidenceScorer
	pubmed      *PubMedSearcher
}

// NewMedicalDebateEngine creates a new medical debate engine.
func NewMedicalDebateEngine(engine *debate.Engine) *MedicalDebateEngine {
	return &MedicalDebateEngine{
		engine:   engine,
		redFlags: NewRedFlagDetector(),
		evidence: NewEvidenceScorer(),
		pubmed:   NewPubMedSearcher(""), // No API key by default
	}
}

// RunMedicalDebate runs a medical-specific debate with enhanced medical logic.
func (m *MedicalDebateEngine) RunMedicalDebate(ctx context.Context, session *models.DebateSession, caseData *MedicalCase) error {
	// Detect red flags
	caseData.RedFlags = m.redFlags.DetectRedFlags(caseData)
	
	// Log red flags
	if len(caseData.RedFlags) > 0 {
		log.Printf("MEDICAL: Detected %d red flags in case", len(caseData.RedFlags))
		for _, flag := range caseData.RedFlags {
			log.Printf("  - %s (%s): %s", flag.Symptom, flag.Severity, flag.Description)
		}
	}

	// Enhance the topic with medical context
	session.Topic = m.enhanceTopicWithMedicalContext(session.Topic, caseData)

	// Run the standard debate engine
	// The medical agents will use their specialized prompts
	m.engine.Run(ctx, session)

	return nil
}

// enhanceTopicWithMedicalContext adds medical context to the debate topic.
func (m *MedicalDebateEngine) enhanceTopicWithMedicalContext(topic string, caseData *MedicalCase) string {
	enhanced := topic + "\n\n## Medical Case Context\n\n"
	
	// Patient demographics
	enhanced += fmt.Sprintf("**Patient:** %d-year-old %s", caseData.PatientInfo.Age, caseData.PatientInfo.Sex)
	if caseData.PatientInfo.BMI > 0 {
		enhanced += fmt.Sprintf(", BMI: %.1f", caseData.PatientInfo.BMI)
	}
	enhanced += "\n\n"

	// Symptoms
	if len(caseData.Symptoms) > 0 {
		enhanced += "### Presenting Symptoms\n"
		for _, symptom := range caseData.Symptoms {
			enhanced += fmt.Sprintf("- **%s** (Severity: %d/10", symptom.Name, symptom.Severity)
			if symptom.Duration != "" {
				enhanced += fmt.Sprintf(", Duration: %s", symptom.Duration)
			}
			if symptom.Location != "" {
				enhanced += fmt.Sprintf(", Location: %s", symptom.Location)
			}
			enhanced += ")\n"
			if symptom.Aggravating != "" {
				enhanced += fmt.Sprintf("  - Aggravating: %s\n", symptom.Aggravating)
			}
			if symptom.Alleviating != "" {
				enhanced += fmt.Sprintf("  - Alleviating: %s\n", symptom.Alleviating)
			}
		}
		enhanced += "\n"
	}

	// Vital signs
	if caseData.VitalSigns.Temperature > 0 || caseData.VitalSigns.HeartRate > 0 {
		enhanced += "### Vital Signs\n"
		if caseData.VitalSigns.Temperature > 0 {
			enhanced += fmt.Sprintf("- Temperature: %.1f°C\n", caseData.VitalSigns.Temperature)
		}
		if caseData.VitalSigns.HeartRate > 0 {
			enhanced += fmt.Sprintf("- Heart Rate: %d bpm\n", caseData.VitalSigns.HeartRate)
		}
		if caseData.VitalSigns.BloodPressure != "" {
			enhanced += fmt.Sprintf("- Blood Pressure: %s\n", caseData.VitalSigns.BloodPressure)
		}
		if caseData.VitalSigns.RespiratoryRate > 0 {
			enhanced += fmt.Sprintf("- Respiratory Rate: %d /min\n", caseData.VitalSigns.RespiratoryRate)
		}
		if caseData.VitalSigns.SpO2 > 0 {
			enhanced += fmt.Sprintf("- SpO2: %d%%\n", caseData.VitalSigns.SpO2)
		}
		if caseData.VitalSigns.PainScore > 0 {
			enhanced += fmt.Sprintf("- Pain Score: %d/10\n", caseData.VitalSigns.PainScore)
		}
		enhanced += "\n"
	}

	// Medical history
	if len(caseData.MedicalHistory.PastConditions) > 0 {
		enhanced += "### Past Medical History\n"
		for _, condition := range caseData.MedicalHistory.PastConditions {
			enhanced += fmt.Sprintf("- %s\n", condition)
		}
		enhanced += "\n"
	}

	// Current medications
	if len(caseData.CurrentMeds) > 0 {
		enhanced += "### Current Medications\n"
		for _, med := range caseData.CurrentMeds {
			enhanced += fmt.Sprintf("- %s %s %s (%s)\n", med.Name, med.Dose, med.Frequency, med.Route)
		}
		enhanced += "\n"
	}

	// Allergies
	if len(caseData.Allergies) > 0 {
		enhanced += "### Allergies\n"
		for _, allergy := range caseData.Allergies {
			enhanced += fmt.Sprintf("- %s\n", allergy)
		}
		enhanced += "\n"
	}

	// Lab results
	if len(caseData.LabResults) > 0 {
		enhanced += "### Laboratory Results\n"
		for _, lab := range caseData.LabResults {
			flag := ""
			if lab.Flag == "high" {
				flag = " ↑"
			} else if lab.Flag == "low" {
				flag = " ↓"
			}
			enhanced += fmt.Sprintf("- %s: %.2f %s%s (Ref: %.2f-%.2f)\n", 
				lab.TestName, lab.Value, lab.Unit, flag, lab.ReferenceMin, lab.ReferenceMax)
		}
		enhanced += "\n"
	}

	// Imaging results
	if len(caseData.ImagingResults) > 0 {
		enhanced += "### 🩻 Imaging Studies\n"
		for _, img := range caseData.ImagingResults {
			enhanced += fmt.Sprintf("- **%s** — %s", img.Modality, img.BodyPart)
			if img.Finding != "" {
				enhanced += fmt.Sprintf("\n  - Finding: %s", img.Finding)
			}
			if img.ImagePath != "" {
				enhanced += fmt.Sprintf("\n  - Image: `%s`", img.ImagePath)
			}
			if img.Date != "" {
				enhanced += fmt.Sprintf(" (Date: %s)", img.Date)
			}
			enhanced += "\n"
		}
		enhanced += "\n"
	}

	// Red flags
	if len(caseData.RedFlags) > 0 {
		enhanced += "### ⚠️ Red Flags Detected\n"
		for _, flag := range caseData.RedFlags {
			enhanced += fmt.Sprintf("- **%s** (%s): %s\n", flag.Symptom, flag.Severity, flag.Description)
			enhanced += fmt.Sprintf("  - Action: %s\n", flag.Action)
		}
		enhanced += "\n"
	}

	// Disclaimer
	enhanced += "---\n**Disclaimer:** This is a simulated medical case discussion for educational purposes only. Not medical advice.\n"

	return enhanced
}

// CreateMedicalAgents creates a panel of medical agents for the debate.
func CreateMedicalAgents() []models.Agent {
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
