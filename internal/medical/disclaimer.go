package medical

// Disclaimer contains medical disclaimer and consent text.
const (
	// MainDisclaimer is the primary disclaimer shown to users.
	MainDisclaimer = `IMPORTANT MEDICAL DISCLAIMER

This AI Debate System is designed for educational and discussion purposes only. It is NOT a substitute for professional medical advice, diagnosis, or treatment.

KEY LIMITATIONS:
• This system does NOT provide medical advice
• AI agents may generate inaccurate or incomplete information
• Always seek the advice of qualified healthcare professionals
• Never disregard professional medical advice or delay in seeking it based on information from this system

EMERGENCY situations:
If you think you may have a medical emergency, call your doctor, go to the emergency department, or call emergency services immediately.

LIMITED CAPABILITIES:
• AI agents cannot perform physical examinations
• AI agents cannot order or interpret actual medical tests
• AI agents do not have access to your complete medical history
• AI agents cannot consider all individual factors that affect diagnosis and treatment

USE AT YOUR OWN RISK:
By using this system, you acknowledge that:
1. You will verify all information with qualified healthcare professionals
2. You will not rely solely on this system for medical decisions
3. The developers are not liable for any decisions made based on this system's output
4. This system is a discussion tool, not a diagnostic tool`

	// InformedConsent is the consent text users must accept before using medical features.
	InformedConsent = `INFORMED CONSENT FOR MEDICAL DISCUSSION SYSTEM

I understand and agree that:

1. PURPOSE: This system is for educational discussion and exploring differential diagnoses through AI agent debate. It is NOT a diagnostic tool.

2. LIMITATIONS: I understand that:
   - AI agents may provide inaccurate information
   - The system cannot replace professional medical judgment
   - I must verify all information with qualified healthcare providers
   - The system cannot access my actual medical records or perform examinations

3. NOT MEDICAL ADVICE: I understand that:
   - Output from this system is NOT medical advice
   - I should not make medical decisions based solely on this system
   - I will consult qualified healthcare professionals for actual medical care

4. EMERGENCIES: I understand that:
   - This system is not for emergency medical situations
   - I should call emergency services or go to the ER for urgent medical needs
   - The system cannot provide emergency medical guidance

5. PRIVACY: I understand that:
   - I should not input sensitive personal health information
   - Case discussions should be de-identified
   - The system is not HIPAA-compliant for real patient data

6. LIABILITY: I accept that:
   - I use this system at my own risk
   - The developers are not liable for any decisions made based on this system
   - I am responsible for verifying information with healthcare professionals

7. PROFESSIONAL USE: If I am a healthcare professional:
   - I will use this system only for educational purposes
   - I will not use it for actual patient diagnosis or treatment decisions
   - I will maintain professional standards and judgment

By proceeding, I confirm that I have read, understood, and agree to these terms.`

	// CaseDisclaimer is shown with each medical case discussion.
	CaseDisclaimer = `This case discussion is for educational purposes only. The AI agents' analysis does not constitute medical advice. Always consult qualified healthcare professionals for actual medical decisions.`

	// RedFlagWarning is shown when red flags are detected.
	RedFlagWarning = `⚠️ RED FLAGS DETECTED

This case contains warning signs that may indicate serious or life-threatening conditions.

IMMEDIATE ACTION REQUIRED:
• Seek immediate medical attention
• Call emergency services if symptoms are severe
• Do not delay seeking care based on this AI discussion

This is NOT a diagnosis. These are potential warning signs that require professional medical evaluation.`
)

// ConsentRecord tracks user consent for medical features.
type ConsentRecord struct {
	UserID        string `json:"user_id"`
	AcceptedAt    string `json:"accepted_at"`
	IPAddress     string `json:"ip_address"`
	Version       string `json:"version"`
	MedicalProfessional bool `json:"medical_professional"`
}

// ValidateConsent checks if consent is valid and current.
func ValidateConsent(consent *ConsentRecord) bool {
	if consent == nil {
		return false
	}
	// Consent should be re-accepted periodically (e.g., every 6 months)
	// For now, just check if it exists
	return consent.AcceptedAt != ""
}

// GetDisclaimerForMode returns the appropriate disclaimer based on debate mode.
func GetDisclaimerForMode(mode MedicalDebateMode) string {
	switch mode {
	case ModeDifferentialDiagnosis:
		return CaseDisclaimer + "\n\nThis differential diagnosis discussion explores multiple possible conditions. It does NOT provide a diagnosis."
	case ModeTreatmentPlanning:
		return CaseDisclaimer + "\n\nThis treatment planning discussion explores various treatment options. Actual treatment decisions must be made with healthcare providers."
	case ModeSecondOpinion:
		return CaseDisclaimer + "\n\nThis second opinion discussion provides alternative perspectives. It does NOT replace professional medical consultation."
	case ModeCaseReview:
		return CaseDisclaimer + "\n\nThis case review is for educational purposes. It does NOT constitute peer review or quality assurance."
	default:
		return CaseDisclaimer
	}
}
