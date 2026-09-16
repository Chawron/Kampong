package medical

import (
	"time"
)

// MedicalCase represents a structured medical case for diagnosis debate.
type MedicalCase struct {
	ID              string           `json:"id"`
	PatientInfo     PatientInfo      `json:"patient_info"`
	Symptoms        []Symptom        `json:"symptoms"`
	MedicalHistory  MedicalHistory   `json:"medical_history"`
	CurrentMeds     []Medication     `json:"current_meds"`
	Allergies       []string         `json:"allergies"`
	VitalSigns      VitalSigns       `json:"vital_signs"`
	LabResults      []LabResult      `json:"lab_results"`
	ImagingResults  []ImagingResult  `json:"imaging_results"`
	RedFlags        []RedFlag        `json:"red_flags"`
	CreatedAt       time.Time        `json:"created_at"`
}

// PatientInfo contains demographic information.
type PatientInfo struct {
	Age           int     `json:"age"`
	Sex           string  `json:"sex"`            // male, female, other
	Ethnicity     string  `json:"ethnicity"`
	Weight        float64 `json:"weight"`          // kg
	Height        float64 `json:"height"`          // cm
	BMI           float64 `json:"bmi"`
	Occupation    string  `json:"occupation"`
	SmokingStatus string  `json:"smoking_status"` // never, former, current
	AlcoholUse    string  `json:"alcohol_use"`    // none, moderate, heavy
}

// Symptom represents a patient symptom with details.
type Symptom struct {
	Name        string `json:"name"`
	Severity    int    `json:"severity"`     // 1-10
	Onset       string `json:"onset"`        // acute, chronic, subacute
	Duration    string `json:"duration"`     // e.g., "3 days", "2 weeks"
	Frequency   string `json:"frequency"`    // constant, intermittent, episodic
	Location    string `json:"location"`
	Radiation   string `json:"radiation"`    // if pain radiates
	Aggravating string `json:"aggravating"`  // what makes it worse
	Alleviating string `json:"alleviating"`  // what makes it better
	Associated  string `json:"associated"`   // associated symptoms
}

// MedicalHistory contains past medical history.
type MedicalHistory struct {
	PastConditions   []string `json:"past_conditions"`
	Surgeries        []string `json:"surgeries"`
	FamilyHistory    []string `json:"family_history"`
	SocialHistory    string   `json:"social_history"`
	MenstrualHistory string   `json:"menstrual_history"` // if applicable
}

// Medication represents a current medication.
type Medication struct {
	Name       string `json:"name"`
	Dose       string `json:"dose"`
	Frequency  string `json:"frequency"`
	Route      string `json:"route"` // oral, IV, topical, etc.
	StartDate  string `json:"start_date"`
}

// VitalSigns contains current vital signs.
type VitalSigns struct {
	Temperature    float64 `json:"temperature"`      // Celsius
	HeartRate      int     `json:"heart_rate"`       // bpm
	BloodPressure  string  `json:"blood_pressure"`   // e.g., "120/80"
	RespiratoryRate int    `json:"respiratory_rate"` // breaths/min
	SpO2           int     `json:"spo2"`             // percentage
	PainScore      int     `json:"pain_score"`       // 0-10
}

// LabResult represents a laboratory test result.
type LabResult struct {
	TestName    string  `json:"test_name"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	ReferenceMin float64 `json:"reference_min"`
	ReferenceMax float64 `json:"reference_max"`
	Flag        string  `json:"flag"` // normal, low, high, critical
	Date        string  `json:"date"`
}

// ImagingResult represents medical imaging.
type ImagingResult struct {
	Modality   string `json:"modality"` // X-ray, CT, MRI, Ultrasound, etc.
	BodyPart   string `json:"body_part"`
	Finding    string `json:"finding"`
	ImagePath  string `json:"image_path"` // path to uploaded image
	Date       string `json:"date"`
}

// RedFlag represents a critical warning sign.
type RedFlag struct {
	Symptom     string `json:"symptom"`
	Severity    string `json:"severity"` // critical, urgent, important
	Description string `json:"description"`
	Action      string `json:"action"` // recommended action
}

// EvidenceLevel represents the quality of medical evidence.
type EvidenceLevel struct {
	Level       int    `json:"level"`        // 1-5 (Oxford levels)
	Description string `json:"description"`
	StudyType   string `json:"study_type"`   // RCT, cohort, case-control, etc.
	SampleSize  int    `json:"sample_size"`
	Confidence  float64 `json:"confidence"`  // 0-1
	BiasRisk    string `json:"bias_risk"`    // low, moderate, high
}

// Diagnosis represents a potential diagnosis.
type Diagnosis struct {
	Name           string          `json:"name"`
	ICD10Code      string          `json:"icd10_code"`
	Probability    float64         `json:"probability"`    // 0-1
	EvidenceLevel  EvidenceLevel   `json:"evidence_level"`
	Supporting     []string        `json:"supporting"`     // supporting evidence
	Against        []string        `json:"against"`        // contradicting evidence
	Differential   []string        `json:"differential"`   // differential diagnoses
	RedFlags       []string        `json:"red_flags"`      // associated red flags
	Confidence     string          `json:"confidence"`     // high, moderate, low
}

// TreatmentPlan represents a treatment recommendation.
type TreatmentPlan struct {
	Diagnosis       string   `json:"diagnosis"`
	Medications     []string `json:"medications"`
	Procedures      []string `json:"procedures"`
	Lifestyle       []string `json:"lifestyle"`
	FollowUp        string   `json:"follow_up"`
	Referrals       []string `json:"referrals"`
	PatientEducation string  `json:"patient_education"`
	Precautions     []string `json:"precautions"`
}

// MedicalDebateResult contains the full debate output.
type MedicalDebateResult struct {
	CaseID              string          `json:"case_id"`
	DifferentialDiagnosis []Diagnosis   `json:"differential_diagnosis"`
	RecommendedWorkup   []string        `json:"recommended_workup"`
	TreatmentPlan       TreatmentPlan   `json:"treatment_plan"`
	RedFlags            []RedFlag       `json:"red_flags"`
	EvidenceSummary     string          `json:"evidence_summary"`
	ConfidenceStatement string          `json:"confidence_statement"`
	Disclaimer          string          `json:"disclaimer"`
	DebateTranscript    []string        `json:"debate_transcript"`
}

// MedicalAgentRole defines specialized medical agent roles.
type MedicalAgentRole string

const (
	RoleDiagnostician    MedicalAgentRole = "diagnostician"
	RoleSpecialist       MedicalAgentRole = "specialist"
	RoleRadiologist      MedicalAgentRole = "radiologist"
	RolePathologist      MedicalAgentRole = "pathologist"
	RolePharmacist       MedicalAgentRole = "pharmacist"
	RoleEvidenceReviewer MedicalAgentRole = "evidence_reviewer"
	RolePatientAdvocate  MedicalAgentRole = "patient_advocate"
	RoleEthicist         MedicalAgentRole = "ethicist"
)

// MedicalDebateMode defines medical-specific debate modes.
type MedicalDebateMode string

const (
	ModeDifferentialDiagnosis MedicalDebateMode = "differential_diagnosis"
	ModeTreatmentPlanning     MedicalDebateMode = "treatment_planning"
	ModeSecondOpinion         MedicalDebateMode = "second_opinion"
	ModeCaseReview            MedicalDebateMode = "case_review"
)
