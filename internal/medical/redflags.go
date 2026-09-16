package medical

// RedFlagDetector identifies critical warning signs in medical cases.
type RedFlagDetector struct {
	rules []RedFlagRule
}

// RedFlagRule defines a rule for detecting red flags.
type RedFlagRule struct {
	ID          string
	Symptoms    []string // symptoms that trigger this rule
	Severity    string   // critical, urgent, important
	Description string
	Action      string
}

// NewRedFlagDetector creates a new red flag detector with standard rules.
func NewRedFlagDetector() *RedFlagDetector {
	return &RedFlagDetector{
		rules: []RedFlagRule{
			// Chest pain rules
			{
				ID:          "chest_pain_cardiac",
				Symptoms:    []string{"chest pain", "chest pressure", "chest tightness"},
				Severity:    "critical",
				Description: "Chest pain with cardiac features - rule out acute coronary syndrome",
				Action:      "Immediate ECG, troponin, consider emergency referral",
			},
			{
				ID:          "chest_pain_radiation",
				Symptoms:    []string{"chest pain radiating to arm", "chest pain radiating to jaw", "left arm pain"},
				Severity:    "critical",
				Description: "Chest pain with radiation - high suspicion for cardiac origin",
				Action:      "Immediate cardiac workup, consider emergency services",
			},
			// Neurological rules
			{
				ID:          "stroke_symptoms",
				Symptoms:    []string{"facial droop", "arm weakness", "speech difficulty", "sudden numbness", "sudden confusion"},
				Severity:    "critical",
				Description: "Possible stroke symptoms - time-critical emergency",
				Action:      "Immediate emergency services, CT head, neurology consult",
			},
			{
				ID:          "severe_headache",
				Symptoms:    []string{"worst headache", "thunderclap headache", "sudden severe headache"},
				Severity:    "critical",
				Description: "Severe sudden headache - rule out subarachnoid hemorrhage",
				Action:      "Immediate CT head, neurology consult, consider lumbar puncture",
			},
			// Respiratory rules
			{
				ID:          "severe_dyspnea",
				Symptoms:    []string{"severe shortness of breath", "unable to speak in sentences", "respiratory distress"},
				Severity:    "critical",
				Description: "Severe respiratory distress",
				Action:      "Immediate oxygen, consider emergency services, chest X-ray",
			},
			{
				ID:          "hemoptysis",
				Symptoms:    []string{"coughing blood", "hemoptysis"},
				Severity:    "urgent",
				Description: "Coughing up blood - requires urgent evaluation",
				Action:      "Chest X-ray, CT chest, consider bronchoscopy",
			},
			// Abdominal rules
			{
				ID:          "acute_abdomen",
				Symptoms:    []string{"severe abdominal pain", "rigid abdomen", "rebound tenderness"},
				Severity:    "critical",
				Description: "Acute abdomen - possible surgical emergency",
				Action:      "Surgical consult, CT abdomen, prepare for possible emergency surgery",
			},
			{
				ID:          "gi_bleeding",
				Symptoms:    []string{"vomiting blood", "black tarry stools", "bright red blood in stool"},
				Severity:    "urgent",
				Description: "Gastrointestinal bleeding",
				Action:      "Urgent endoscopy, IV fluids, monitor hemodynamics",
			},
			// Systemic rules
			{
				ID:          "high_fever",
				Symptoms:    []string{"high fever", "temperature over 39C", "fever with rash"},
				Severity:    "urgent",
				Description: "High fever - possible serious infection",
				Action:      "Blood cultures, consider sepsis workup, infectious disease consult",
			},
			{
				ID:          "altered_mental_status",
				Symptoms:    []string{"confusion", "altered consciousness", "decreased level of consciousness"},
				Severity:    "critical",
				Description: "Altered mental status - requires urgent evaluation",
				Action:      "Check glucose, CT head, consider toxicology screen, infectious workup",
			},
			// Trauma rules
			{
				ID:          "head_injury",
				Symptoms:    []string{"head injury with loss of consciousness", "head injury with vomiting", "head injury in anticoagulated patient"},
				Severity:    "critical",
				Description: "Head injury with high-risk features",
				Action:      "CT head, neurosurgery consult, observe for deterioration",
			},
			// Pregnancy rules
			{
				ID:          "pregnancy_emergency",
				Symptoms:    []string{"pregnancy with bleeding", "pregnancy with severe pain", "pregnancy with hypertension"},
				Severity:    "critical",
				Description: "Pregnancy with emergency features",
				Action:      "Obstetric emergency consult, ultrasound, monitor fetal status",
			},
			// Medication rules
			{
				ID:          "drug_interaction",
				Symptoms:    []string{"on warfarin with new medication", "on MAOI with SSRI", "on multiple QT-prolonging drugs"},
				Severity:    "urgent",
				Description: "Potential dangerous drug interaction",
				Action:      "Review medications, check interactions, consider alternatives",
			},
			// Age-related rules
			{
				ID:          "elderly_fall",
				Symptoms:    []string{"elderly patient with fall", "elderly with hip pain", "elderly on anticoagulants with fall"},
				Severity:    "urgent",
				Description: "Elderly patient with fall - high risk for fractures and bleeding",
				Action:      "X-ray affected area, check for head injury, review medications",
			},
			// Cancer red flags
			{
				ID:          "unexplained_weight_loss",
				Symptoms:    []string{"unexplained weight loss", "unintentional weight loss over 5%"},
				Severity:    "important",
				Description: "Unexplained weight loss - possible malignancy",
				Action:      "Comprehensive workup including imaging and labs",
			},
			{
				ID:          "persistent_symptoms",
				Symptoms:    []string{"persistent cough over 3 weeks", "persistent hoarseness", "persistent difficulty swallowing"},
				Severity:    "important",
				Description: "Persistent symptoms - requires investigation",
				Action:      "Appropriate imaging and specialist referral",
			},
		},
	}
}

// DetectRedFlags analyzes a medical case and returns identified red flags.
func (d *RedFlagDetector) DetectRedFlags(caseData *MedicalCase) []RedFlag {
	var flags []RedFlag

	// Check symptoms against rules
	for _, rule := range d.rules {
		for _, symptom := range caseData.Symptoms {
			for _, triggerSymptom := range rule.Symptoms {
				if containsIgnoreCase(symptom.Name, triggerSymptom) {
					flags = append(flags, RedFlag{
						Symptom:     symptom.Name,
						Severity:    rule.Severity,
						Description: rule.Description,
						Action:      rule.Action,
					})
					break // One match per rule is enough
				}
			}
		}
	}

	// Check vital signs for critical values
	if caseData.VitalSigns.Temperature > 39.0 {
		flags = append(flags, RedFlag{
			Symptom:     "High fever",
			Severity:    "urgent",
			Description: "Temperature > 39°C",
			Action:      "Consider sepsis workup, blood cultures",
		})
	}

	if caseData.VitalSigns.HeartRate > 120 || caseData.VitalSigns.HeartRate < 40 {
		flags = append(flags, RedFlag{
			Symptom:     "Abnormal heart rate",
			Severity:    "urgent",
			Description: "Heart rate outside normal range",
			Action:      "ECG, cardiac monitoring",
		})
	}

	if caseData.VitalSigns.SpO2 < 90 {
		flags = append(flags, RedFlag{
			Symptom:     "Low oxygen saturation",
			Severity:    "critical",
			Description: "SpO2 < 90%",
			Action:      "Immediate oxygen therapy, consider emergency services",
		})
	}

	// Check for drug interactions
	if len(caseData.CurrentMeds) > 0 {
		// Simple check for common dangerous combinations
		// In production, this would use a comprehensive drug interaction database
		flags = append(flags, checkDrugInteractions(caseData.CurrentMeds)...)
	}

	return flags
}

// checkDrugInteractions checks for common dangerous drug combinations.
func checkDrugInteractions(meds []Medication) []RedFlag {
	var flags []RedFlag

	// This is a simplified check - production would use a comprehensive database
	medNames := make(map[string]bool)
	for _, med := range meds {
		medNames[med.Name] = true
	}

	// Check for warfarin + NSAIDs
	if medNames["warfarin"] && (medNames["ibuprofen"] || medNames["naproxen"] || medNames["aspirin"]) {
		flags = append(flags, RedFlag{
			Symptom:     "Drug interaction",
			Severity:    "urgent",
			Description: "Warfarin + NSAID increases bleeding risk",
			Action:      "Review medications, consider alternatives, monitor INR",
		})
	}

	// Check for multiple QT-prolonging drugs
	qtDrugs := []string{"amiodarone", "sotalol", "fluconazole", "azithromycin", "ciprofloxacin"}
	qtCount := 0
	for _, drug := range qtDrugs {
		if medNames[drug] {
			qtCount++
		}
	}
	if qtCount >= 2 {
		flags = append(flags, RedFlag{
			Symptom:     "Drug interaction",
			Severity:    "urgent",
			Description: "Multiple QT-prolonging drugs increase arrhythmia risk",
			Action:      "ECG monitoring, review medications, consider alternatives",
		})
	}

	return flags
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
