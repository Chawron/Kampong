package medical

import (
	"strings"
)

// ClinicalGuideline represents a clinical practice guideline.
type ClinicalGuideline struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Organization string   `json:"organization"` // WHO, AHA, ADA, etc.
	Year         int      `json:"year"`
	Condition    string   `json:"condition"`
	Category     string   `json:"category"` // diagnosis, treatment, prevention, screening
	Recommendations []GuidelineRecommendation `json:"recommendations"`
	EvidenceLevel int     `json:"evidence_level"` // 1-5
	Keywords     []string `json:"keywords"`
}

// GuidelineRecommendation represents a specific recommendation.
type GuidelineRecommendation struct {
	ID           string `json:"id"`
	Statement    string `json:"statement"`
	Strength     string `json:"strength"` // strong, conditional, weak
	EvidenceLevel int   `json:"evidence_level"`
	Applicability string `json:"applicability"` // specific patient population
}

// GuidelineMatcher matches patient cases to relevant clinical guidelines.
type GuidelineMatcher struct {
	guidelines []ClinicalGuideline
}

// NewGuidelineMatcher creates a new guideline matcher with loaded guidelines.
func NewGuidelineMatcher() *GuidelineMatcher {
	matcher := &GuidelineMatcher{
		guidelines: []ClinicalGuideline{},
	}
	matcher.loadGuidelines()
	return matcher
}

// loadGuidelines loads clinical guidelines into the matcher.
func (g *GuidelineMatcher) loadGuidelines() {
	// Hypertension guidelines (JNC 8 / ACC/AHA)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "htn-aha-2017",
		Title:        "2017 ACC/AHA Hypertension Guidelines",
		Organization: "American College of Cardiology/American Heart Association",
		Year:         2017,
		Condition:    "hypertension",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"hypertension", "high blood pressure", "htn", "blood pressure"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "htn-1",
				Statement:     "For adults with BP ≥130/80 mmHg, recommend lifestyle modifications",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "All adults with elevated BP",
			},
			{
				ID:            "htn-2",
				Statement:     "For adults with BP ≥140/90 mmHg or ≥130/80 with CVD risk ≥10%, initiate pharmacologic therapy",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Adults with stage 1-2 hypertension",
			},
			{
				ID:            "htn-3",
				Statement:     "First-line agents: thiazide diuretics, CCBs, ACE inhibitors, or ARBs",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Adults requiring pharmacologic therapy",
			},
			{
				ID:            "htn-4",
				Statement:     "BP target <130/80 mmHg for most adults with hypertension",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Adults on treatment",
			},
		},
	})

	// Diabetes guidelines (ADA)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "dm-ada-2024",
		Title:        "ADA Standards of Medical Care in Diabetes 2024",
		Organization: "American Diabetes Association",
		Year:         2024,
		Condition:    "diabetes",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"diabetes", "diabetes mellitus", "dm", "hyperglycemia", "glucose"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "dm-1",
				Statement:     "HbA1c target <7% for most nonpregnant adults with diabetes",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Adults with diabetes",
			},
			{
				ID:            "dm-2",
				Statement:     "Metformin as first-line pharmacologic agent for type 2 diabetes",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Adults with type 2 diabetes",
			},
			{
				ID:            "dm-3",
				Statement:     "For patients with ASCVD or high risk, consider GLP-1 RA or SGLT2 inhibitor",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Type 2 diabetes with CVD risk",
			},
			{
				ID:            "dm-4",
				Statement:     "Screen for diabetes complications annually: retinopathy, nephropathy, neuropathy",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "All adults with diabetes",
			},
		},
	})

	// Heart failure guidelines (ACC/AHA)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "hf-aha-2022",
		Title:        "2022 AHA/ACC/HFSA Heart Failure Guidelines",
		Organization: "American Heart Association",
		Year:         2022,
		Condition:    "heart failure",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"heart failure", "hf", "congestive heart failure", "chf", "cardiomyopathy"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "hf-1",
				Statement:     "For HFrEF (EF ≤40%), use quadruple therapy: ARNI/ACEi/ARB + beta-blocker + MRA + SGLT2i",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Patients with HFrEF",
			},
			{
				ID:            "hf-2",
				Statement:     "Diuretics for volume overload symptoms",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "HF patients with congestion",
			},
			{
				ID:            "hf-3",
				Statement:     "Consider ICD for primary prevention if EF ≤35% despite optimal therapy",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "HFrEF patients",
			},
		},
	})

	// Pneumonia guidelines (IDSA/ATS)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "pna-idsa-2019",
		Title:        "2019 IDSA/ATS Community-Acquired Pneumonia Guidelines",
		Organization: "Infectious Diseases Society of America",
		Year:         2019,
		Condition:    "pneumonia",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"pneumonia", "community-acquired pneumonia", "cap", "lung infection"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "pna-1",
				Statement:     "For outpatient CAP without comorbidities: amoxicillin or doxycycline or macrolide (if local resistance <25%)",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Outpatient CAP, healthy",
			},
			{
				ID:            "pna-2",
				Statement:     "For outpatient CAP with comorbidities: combination therapy (beta-lactam + macrolide/doxycycline) or respiratory fluoroquinolone",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Outpatient CAP with comorbidities",
			},
			{
				ID:            "pna-3",
				Statement:     "Assess severity using CURB-65 or PSI to determine site of care",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "All CAP patients",
			},
		},
	})

	// Asthma guidelines (GINA)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "asthma-gina-2024",
		Title:        "GINA 2024 Asthma Management Strategy",
		Organization: "Global Initiative for Asthma",
		Year:         2024,
		Condition:    "asthma",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"asthma", "wheezing", "bronchospasm", "reactive airway"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "asthma-1",
				Statement:     "All adults and adolescents with asthma should receive ICS-containing therapy (no SABA-only treatment)",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "All asthma patients",
			},
			{
				ID:            "asthma-2",
				Statement:     "Track 1 (preferred): As-needed low dose ICS-formoterol for mild asthma",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Mild asthma",
			},
			{
				ID:            "asthma-3",
				Statement:     "For moderate-severe asthma: maintenance ICS-LABA + as-needed reliever",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Moderate-severe asthma",
			},
		},
	})

	// CKD guidelines (KDIGO)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "ckd-kdigo-2024",
		Title:        "KDIGO 2024 CKD Management Guidelines",
		Organization: "Kidney Disease: Improving Global Outcomes",
		Year:         2024,
		Condition:    "chronic kidney disease",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"chronic kidney disease", "ckd", "renal failure", "kidney disease", "elevated creatinine"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "ckd-1",
				Statement:     "Use ACEi or ARB for CKD with albuminuria (uACR ≥30 mg/g)",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "CKD with albuminuria",
			},
			{
				ID:            "ckd-2",
				Statement:     "Consider SGLT2 inhibitor for CKD with eGFR ≥20 mL/min/1.73m²",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "CKD patients",
			},
			{
				ID:            "ckd-3",
				Statement:     "BP target <120 mmHg systolic (standardized measurement) if tolerated",
				Strength:      "conditional",
				EvidenceLevel: 2,
				Applicability: "CKD patients",
			},
		},
	})

	// Anticoagulation guidelines (CHEST)
	g.guidelines = append(g.guidelines, ClinicalGuideline{
		ID:           "vt-che-2021",
		Title:        "CHEST 2021 Venous Thromboembolism Guidelines",
		Organization: "American College of Chest Physicians",
		Year:         2021,
		Condition:    "venous thromboembolism",
		Category:     "treatment",
		EvidenceLevel: 1,
		Keywords:     []string{"dvt", "pe", "pulmonary embolism", "deep vein thrombosis", "venous thromboembolism", "vte", "blood clot"},
		Recommendations: []GuidelineRecommendation{
			{
				ID:            "vte-1",
				Statement:     "For acute DVT/PE: anticoagulation for minimum 3 months",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "Acute VTE",
			},
			{
				ID:            "vte-2",
				Statement:     "DOACs preferred over warfarin for non-cancer VTE",
				Strength:      "strong",
				EvidenceLevel: 1,
				Applicability: "VTE without cancer",
			},
			{
				ID:            "vte-3",
				Statement:     "For unprovoked VTE, consider extended anticoagulation if bleeding risk low",
				Strength:      "conditional",
				EvidenceLevel: 2,
				Applicability: "Unprovoked VTE",
			},
		},
	})
}

// MatchGuidelines finds relevant guidelines for a medical case.
func (g *GuidelineMatcher) MatchGuidelines(caseData *MedicalCase) []ClinicalGuideline {
	var matched []ClinicalGuideline

	// Extract keywords from case
	keywords := g.extractKeywords(caseData)

	// Score each guideline
	for _, guideline := range g.guidelines {
		score := g.calculateMatchScore(guideline, keywords)
		if score > 0 {
			matched = append(matched, guideline)
		}
	}

	return matched
}

// extractKeywords extracts relevant keywords from a medical case.
func (g *GuidelineMatcher) extractKeywords(caseData *MedicalCase) []string {
	var keywords []string

	// Add symptoms
	for _, symptom := range caseData.Symptoms {
		keywords = append(keywords, strings.ToLower(symptom.Name))
	}

	// Add conditions
	for _, condition := range caseData.MedicalHistory.PastConditions {
		keywords = append(keywords, strings.ToLower(condition))
	}

	// Add medications (may indicate conditions)
	for _, med := range caseData.CurrentMeds {
		keywords = append(keywords, strings.ToLower(med.Name))
	}

	// Add abnormal lab findings
	for _, lab := range caseData.LabResults {
		if lab.Flag == "high" || lab.Flag == "low" {
			keywords = append(keywords, strings.ToLower(lab.TestName))
		}
	}

	return keywords
}

// calculateMatchScore calculates how well a guideline matches the case keywords.
func (g *GuidelineMatcher) calculateMatchScore(guideline ClinicalGuideline, keywords []string) int {
	score := 0

	for _, guidelineKeyword := range guideline.Keywords {
		for _, caseKeyword := range keywords {
			if strings.Contains(caseKeyword, guidelineKeyword) || strings.Contains(guidelineKeyword, caseKeyword) {
				score++
				break
			}
		}
	}

	return score
}
