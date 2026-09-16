package medical

import (
	"fmt"
	"strings"
)

// LabInterpreter interprets laboratory test results.
type LabInterpreter struct {
	referenceRanges map[string]ReferenceRange
	criticalValues  map[string]CriticalValue
}

// ReferenceRange defines normal reference ranges for a lab test.
type ReferenceRange struct {
	TestName    string  `json:"test_name"`
	Unit        string  `json:"unit"`
	MinNormal   float64 `json:"min_normal"`
	MaxNormal   float64 `json:"max_normal"`
	MinCritical float64 `json:"min_critical"` // Below this is critical
	MaxCritical float64 `json:"max_critical"` // Above this is critical
	AgeGroups   []AgeGroupRange `json:"age_groups,omitempty"` // Different ranges for different ages
	SexSpecific bool   `json:"sex_specific"` // Different ranges for male/female
}

// AgeGroupRange defines reference ranges for specific age groups.
type AgeGroupRange struct {
	AgeGroup  string  `json:"age_group"` // pediatric, adult, elderly
	MinNormal float64 `json:"min_normal"`
	MaxNormal float64 `json:"max_normal"`
}

// CriticalValue defines critical value thresholds.
type CriticalValue struct {
	TestName    string  `json:"test_name"`
	LowCritical float64 `json:"low_critical"`
	HighCritical float64 `json:"high_critical"`
	Action      string  `json:"action"`
}

// LabInterpretation contains the interpretation of a lab result.
type LabInterpretation struct {
	TestName          string  `json:"test_name"`
	Value             float64 `json:"value"`
	Unit              string  `json:"unit"`
	Flag              string  `json:"flag"` // normal, low, high, critical_low, critical_high
	ReferenceRange    string  `json:"reference_range"`
	ClinicalSignificance string `json:"clinical_significance"`
	PossibleCauses    []string `json:"possible_causes"`
	RecommendedActions []string `json:"recommended_actions"`
	FollowUpTests     []string `json:"follow_up_tests"`
	Urgency           string  `json:"urgency"` // routine, urgent, emergency
}

// NewLabInterpreter creates a new lab interpreter.
func NewLabInterpreter() *LabInterpreter {
	interpreter := &LabInterpreter{
		referenceRanges: make(map[string]ReferenceRange),
		criticalValues:  make(map[string]CriticalValue),
	}
	interpreter.loadReferenceData()
	return interpreter
}

// loadReferenceData loads standard reference ranges.
func (l *LabInterpreter) loadReferenceData() {
	// Complete Blood Count (CBC)
	l.referenceRanges["hemoglobin"] = ReferenceRange{
		TestName:    "Hemoglobin",
		Unit:        "g/dL",
		MinNormal:   13.5, // adult male baseline
		MaxNormal:   17.5,
		MinCritical: 7.0,
		MaxCritical: 20.0,
		SexSpecific: true,
		AgeGroups: []AgeGroupRange{
			{AgeGroup: "pediatric", MinNormal: 11.0, MaxNormal: 15.5}, // 1–17y
			{AgeGroup: "elderly", MinNormal: 11.7, MaxNormal: 16.1},   // ≥65y
		},
	}

	l.referenceRanges["hematocrit"] = ReferenceRange{
		TestName:    "Hematocrit",
		Unit:        "%",
		MinNormal:   38.5, // adult male baseline
		MaxNormal:   50.0,
		MinCritical: 21.0,
		MaxCritical: 60.0,
		SexSpecific: true,
	}

	l.referenceRanges["wbc"] = ReferenceRange{
		TestName:    "White Blood Cell Count",
		Unit:        "x10^3/uL",
		MinNormal:   4.5,
		MaxNormal:   11.0,
		MinCritical: 2.0,
		MaxCritical: 30.0,
	}

	l.referenceRanges["platelets"] = ReferenceRange{
		TestName:    "Platelet Count",
		Unit:        "x10^3/uL",
		MinNormal:   150.0,
		MaxNormal:   400.0,
		MinCritical: 50.0,
		MaxCritical: 1000.0,
	}

	// Metabolic Panel
	l.referenceRanges["glucose"] = ReferenceRange{
		TestName:    "Glucose (fasting)",
		Unit:        "mg/dL",
		MinNormal:   70.0,
		MaxNormal:   100.0,
		MinCritical: 40.0,
		MaxCritical: 500.0,
	}

	l.referenceRanges["creatinine"] = ReferenceRange{
		TestName:    "Creatinine",
		Unit:        "mg/dL",
		MinNormal:   0.7, // adult male baseline
		MaxNormal:   1.3,
		MinCritical: 0.0,
		MaxCritical: 10.0,
		SexSpecific: true,
		AgeGroups: []AgeGroupRange{
			{AgeGroup: "pediatric", MinNormal: 0.3, MaxNormal: 1.0}, // 1–17y
			{AgeGroup: "elderly", MinNormal: 0.6, MaxNormal: 1.2},   // ≥65y
		},
	}

	l.referenceRanges["bun"] = ReferenceRange{
		TestName:    "Blood Urea Nitrogen",
		Unit:        "mg/dL",
		MinNormal:   7.0,
		MaxNormal:   20.0,
		MinCritical: 0.0,
		MaxCritical: 100.0,
	}

	l.referenceRanges["sodium"] = ReferenceRange{
		TestName:    "Sodium",
		Unit:        "mEq/L",
		MinNormal:   136.0,
		MaxNormal:   145.0,
		MinCritical: 120.0,
		MaxCritical: 160.0,
	}

	l.referenceRanges["potassium"] = ReferenceRange{
		TestName:    "Potassium",
		Unit:        "mEq/L",
		MinNormal:   3.5,
		MaxNormal:   5.0,
		MinCritical: 2.5,
		MaxCritical: 6.5,
	}

	l.referenceRanges["calcium"] = ReferenceRange{
		TestName:    "Calcium",
		Unit:        "mg/dL",
		MinNormal:   8.5,
		MaxNormal:   10.5,
		MinCritical: 6.0,
		MaxCritical: 13.0,
	}

	// Liver Function Tests
	l.referenceRanges["alt"] = ReferenceRange{
		TestName:    "ALT (SGPT)",
		Unit:        "U/L",
		MinNormal:   7.0,
		MaxNormal:   56.0,
		MinCritical: 0.0,
		MaxCritical: 1000.0,
	}

	l.referenceRanges["ast"] = ReferenceRange{
		TestName:    "AST (SGOT)",
		Unit:        "U/L",
		MinNormal:   10.0,
		MaxNormal:   40.0,
		MinCritical: 0.0,
		MaxCritical: 1000.0,
	}

	l.referenceRanges["alp"] = ReferenceRange{
		TestName:    "Alkaline Phosphatase",
		Unit:        "U/L",
		MinNormal:   44.0,
		MaxNormal:   147.0,
		MinCritical: 0.0,
		MaxCritical: 500.0,
	}

	l.referenceRanges["bilirubin"] = ReferenceRange{
		TestName:    "Bilirubin (total)",
		Unit:        "mg/dL",
		MinNormal:   0.1,
		MaxNormal:   1.2,
		MinCritical: 0.0,
		MaxCritical: 20.0,
	}

	// Coagulation
	l.referenceRanges["inr"] = ReferenceRange{
		TestName:    "INR",
		Unit:        "",
		MinNormal:   0.9,
		MaxNormal:   1.1,
		MinCritical: 0.0,
		MaxCritical: 9.0,
	}

	l.referenceRanges["ptt"] = ReferenceRange{
		TestName:    "PTT",
		Unit:        "seconds",
		MinNormal:   25.0,
		MaxNormal:   35.0,
		MinCritical: 0.0,
		MaxCritical: 100.0,
	}

	// Cardiac markers
	l.referenceRanges["troponin"] = ReferenceRange{
		TestName:    "Troponin I",
		Unit:        "ng/mL",
		MinNormal:   0.0,
		MaxNormal:   0.04,
		MinCritical: 0.0,
		MaxCritical: 10.0,
	}

	l.referenceRanges["bnp"] = ReferenceRange{
		TestName:    "BNP",
		Unit:        "pg/mL",
		MinNormal:   0.0,
		MaxNormal:   100.0,
		MinCritical: 0.0,
		MaxCritical: 5000.0,
	}

	// Inflammatory markers
	l.referenceRanges["crp"] = ReferenceRange{
		TestName:    "C-Reactive Protein",
		Unit:        "mg/L",
		MinNormal:   0.0,
		MaxNormal:   3.0,
		MinCritical: 0.0,
		MaxCritical: 500.0,
	}

	l.referenceRanges["esr"] = ReferenceRange{
		TestName:    "ESR",
		Unit:        "mm/hr",
		MinNormal:   0.0,
		MaxNormal:   20.0,
		MinCritical: 0.0,
		MaxCritical: 150.0,
	}

	// Thyroid function
	l.referenceRanges["tsh"] = ReferenceRange{
		TestName:    "TSH",
		Unit:        "mIU/L",
		MinNormal:   0.4,
		MaxNormal:   4.0,
		MinCritical: 0.0,
		MaxCritical: 100.0,
	}

	l.referenceRanges["free_t4"] = ReferenceRange{
		TestName:    "Free T4",
		Unit:        "ng/dL",
		MinNormal:   0.8,
		MaxNormal:   1.8,
		MinCritical: 0.0,
		MaxCritical: 10.0,
	}

	// Load critical values
	l.criticalValues["potassium"] = CriticalValue{
		TestName:     "Potassium",
		LowCritical:  2.5,
		HighCritical: 6.5,
		Action:       "Immediate ECG, cardiac monitoring, urgent correction",
	}

	l.criticalValues["sodium"] = CriticalValue{
		TestName:     "Sodium",
		LowCritical:  120.0,
		HighCritical: 160.0,
		Action:       "Urgent correction with careful monitoring",
	}

	l.criticalValues["glucose"] = CriticalValue{
		TestName:     "Glucose",
		LowCritical:  40.0,
		HighCritical: 500.0,
		Action:       "Immediate treatment for hypo/hyperglycemia",
	}

	l.criticalValues["calcium"] = CriticalValue{
		TestName:     "Calcium",
		LowCritical:  6.0,
		HighCritical: 13.0,
		Action:       "Urgent correction, cardiac monitoring",
	}

	l.criticalValues["hemoglobin"] = CriticalValue{
		TestName:     "Hemoglobin",
		LowCritical:  7.0,
		HighCritical: 20.0,
		Action:       "Consider transfusion if symptomatic",
	}
}

// InterpretLabResult interprets a single lab result.
func (l *LabInterpreter) InterpretLabResult(lab LabResult, age int, sex string) *LabInterpretation {
	testNameLower := strings.ToLower(lab.TestName)
	
	interpretation := &LabInterpretation{
		TestName: lab.TestName,
		Value:    lab.Value,
		Unit:     lab.Unit,
	}

	// Find reference range
	refRange, exists := l.referenceRanges[testNameLower]
	if !exists {
		interpretation.Flag = "unknown"
		interpretation.ClinicalSignificance = "No reference range available for this test"
		return interpretation
	}

	// Apply age-group and sex-specific adjustments.
	l.resolveReferenceRange(&refRange, age, sex)

	interpretation.ReferenceRange = fmt.Sprintf("%.1f-%.1f %s", refRange.MinNormal, refRange.MaxNormal, refRange.Unit)

	// Determine flag
	if lab.Value < refRange.MinCritical {
		interpretation.Flag = "critical_low"
		interpretation.Urgency = "emergency"
	} else if lab.Value > refRange.MaxCritical {
		interpretation.Flag = "critical_high"
		interpretation.Urgency = "emergency"
	} else if lab.Value < refRange.MinNormal {
		interpretation.Flag = "low"
		interpretation.Urgency = "urgent"
	} else if lab.Value > refRange.MaxNormal {
		interpretation.Flag = "high"
		interpretation.Urgency = "urgent"
	} else {
		interpretation.Flag = "normal"
		interpretation.Urgency = "routine"
	}

	// Add clinical significance and recommendations
	l.addClinicalContext(interpretation, testNameLower, age, sex)

	return interpretation
}

// resolveReferenceRange adjusts the normal range for the patient's age group
// and sex. Age ≤ 0 (unknown) is treated as an adult without sex adjustment.
func (l *LabInterpreter) resolveReferenceRange(rr *ReferenceRange, age int, sex string) {
	// Age-group override first (pediatric ranges differ most).
	if age > 0 && len(rr.AgeGroups) > 0 {
		group := ""
		switch {
		case age < 18:
			group = "pediatric"
		case age >= 65:
			group = "elderly"
		default:
			group = "adult"
		}
		for _, ag := range rr.AgeGroups {
			if ag.AgeGroup == group {
				rr.MinNormal = ag.MinNormal
				rr.MaxNormal = ag.MaxNormal
				break
			}
		}
	}

	// Sex-specific adjustment for adults (baseline data is male).
	if rr.SexSpecific && strings.EqualFold(sex, "female") {
		switch strings.ToLower(rr.TestName) {
		case "hemoglobin":
			rr.MinNormal = 12.0
			rr.MaxNormal = 15.5
		case "hematocrit":
			rr.MinNormal = 34.9
			rr.MaxNormal = 44.5
		case "creatinine":
			rr.MinNormal = 0.6
			rr.MaxNormal = 1.1
		}
	}
}

// addClinicalContext adds clinical significance and recommendations.
func (l *LabInterpreter) addClinicalContext(interp *LabInterpretation, testName string, age int, sex string) {
	switch testName {
	case "hemoglobin":
		if interp.Flag == "low" || interp.Flag == "critical_low" {
			interp.ClinicalSignificance = "Anemia - reduced oxygen-carrying capacity"
			interp.PossibleCauses = []string{
				"Iron deficiency",
				"Blood loss (GI, menstrual)",
				"Chronic disease",
				"B12/folate deficiency",
				"Bone marrow suppression",
			}
			interp.RecommendedActions = []string{
				"Check iron studies, B12, folate",
				"Reticulocyte count",
				"Peripheral blood smear",
				"Consider GI evaluation if iron deficiency",
			}
			interp.FollowUpTests = []string{"Iron studies", "B12", "Folate", "Reticulocyte count"}
		} else if interp.Flag == "high" {
			interp.ClinicalSignificance = "Polycythemia - increased blood viscosity"
			interp.PossibleCauses = []string{
				"Dehydration",
				"Polycythemia vera",
				"Chronic hypoxia",
				"Smoking",
			}
			interp.RecommendedActions = []string{
				"Check hydration status",
				"JAK2 mutation if polycythemia vera suspected",
				"Consider phlebotomy if symptomatic",
			}
		}

	case "potassium":
		if interp.Flag == "low" || interp.Flag == "critical_low" {
			interp.ClinicalSignificance = "Hypokalemia - risk of cardiac arrhythmias"
			interp.PossibleCauses = []string{
				"Diuretic use",
				"GI losses (vomiting, diarrhea)",
				"Renal losses",
				"Insulin therapy",
				"Alkalosis",
			}
			interp.RecommendedActions = []string{
				"ECG if severe",
				"Check magnesium level",
				"Oral or IV potassium replacement",
				"Review medications",
			}
			interp.FollowUpTests = []string{"ECG", "Magnesium", "Renal function"}
		} else if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Hyperkalemia - risk of cardiac arrest"
			interp.PossibleCauses = []string{
				"Renal failure",
				"ACE inhibitors/ARBs",
				"Potassium-sparing diuretics",
				"Cell lysis (rhabdomyolysis, tumor lysis)",
				"Acidosis",
			}
			interp.RecommendedActions = []string{
				"Immediate ECG",
				"Cardiac monitoring",
				"Calcium gluconate if ECG changes",
				"Insulin + glucose",
				"Consider dialysis if severe",
			}
			interp.FollowUpTests = []string{"ECG", "Renal function", "CK if rhabdomyolysis suspected"}
		}

	case "glucose":
		if interp.Flag == "low" || interp.Flag == "critical_low" {
			interp.ClinicalSignificance = "Hypoglycemia - neuroglycopenia risk"
			interp.PossibleCauses = []string{
				"Insulin/sulfonylurea overdose",
				"Insulinoma",
				"Adrenal insufficiency",
				"Severe liver disease",
				"Sepsis",
			}
			interp.RecommendedActions = []string{
				"Immediate glucose administration",
				"Check insulin and sulfonylurea levels",
				"Cortisol level if recurrent",
			}
		} else if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Hyperglycemia - risk of DKA/HHS"
			interp.PossibleCauses = []string{
				"Diabetes mellitus",
				"Stress hyperglycemia",
				"Medication-induced (steroids)",
				"DKA/HHS",
			}
			interp.RecommendedActions = []string{
				"Check ketones if glucose > 250",
				"Check HbA1c",
				"Assess for DKA/HHS symptoms",
				"Insulin therapy if needed",
			}
			interp.FollowUpTests = []string{"Ketones", "HbA1c", "Venous blood gas if DKA suspected"}
		}

	case "creatinine":
		if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Renal impairment - reduced GFR"
			interp.PossibleCauses = []string{
				"Acute kidney injury",
				"Chronic kidney disease",
				"Dehydration",
				"Nephrotoxic drugs",
				"Obstruction",
			}
			interp.RecommendedActions = []string{
				"Calculate eGFR",
				"Check BUN/creatinine ratio",
				"Urinalysis",
				"Renal ultrasound",
				"Review nephrotoxic medications",
			}
			interp.FollowUpTests = []string{"eGFR", "Urinalysis", "BUN", "Renal ultrasound"}
		}

	case "sodium":
		if interp.Flag == "low" || interp.Flag == "critical_low" {
			interp.ClinicalSignificance = "Hyponatremia - risk of cerebral edema"
			interp.PossibleCauses = []string{
				"SIADH",
				"Heart failure",
				"Liver cirrhosis",
				"Diuretic use",
				"Adrenal insufficiency",
			}
			interp.RecommendedActions = []string{
				"Check volume status",
				"Urine osmolality and sodium",
				"Correct slowly to avoid osmotic demyelination",
			}
		} else if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Hypernatremia - dehydration"
			interp.PossibleCauses = []string{
				"Dehydration",
				"Diabetes insipidus",
				"Excessive sodium intake",
				"Osmotic diuresis",
			}
			interp.RecommendedActions = []string{
				"Assess volume status",
				"Correct slowly with free water",
				"Check urine osmolality",
			}
		}

	case "troponin":
		if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Myocardial injury - possible MI"
			interp.PossibleCauses = []string{
				"Myocardial infarction",
				"Myocarditis",
				"Pulmonary embolism",
				"Heart failure",
				"Renal failure",
			}
			interp.RecommendedActions = []string{
				"Serial troponin measurements",
				"ECG",
				"Cardiology consultation",
				"Consider coronary angiography",
			}
			interp.FollowUpTests = []string{"Serial troponin", "ECG", "BNP", "Echocardiogram"}
			interp.Urgency = "emergency"
		}

	case "tsh":
		if interp.Flag == "low" {
			interp.ClinicalSignificance = "Possible hyperthyroidism"
			interp.PossibleCauses = []string{
				"Graves' disease",
				"Toxic nodular goiter",
				"Thyroiditis",
				"Exogenous thyroid hormone",
			}
			interp.RecommendedActions = []string{
				"Check free T4 and T3",
				"Thyroid antibodies",
				"Thyroid ultrasound",
			}
			interp.FollowUpTests = []string{"Free T4", "Free T3", "TSH receptor antibodies"}
		} else if interp.Flag == "high" {
			interp.ClinicalSignificance = "Possible hypothyroidism"
			interp.PossibleCauses = []string{
				"Hashimoto's thyroiditis",
				"Iodine deficiency",
				"Post-thyroidectomy",
				"Medication-induced",
			}
			interp.RecommendedActions = []string{
				"Check free T4",
				"Anti-TPO antibodies",
				"Consider levothyroxine therapy",
			}
			interp.FollowUpTests = []string{"Free T4", "Anti-TPO antibodies"}
		}

	case "inr":
		if interp.Flag == "high" || interp.Flag == "critical_high" {
			interp.ClinicalSignificance = "Coagulopathy - bleeding risk"
			interp.PossibleCauses = []string{
				"Warfarin overdose",
				"Liver disease",
				"Vitamin K deficiency",
				"DIC",
			}
			interp.RecommendedActions = []string{
				"Hold warfarin if applicable",
				"Vitamin K if INR > 5 or bleeding",
				"FFP if critical bleeding",
				"Check for bleeding",
			}
		}
	}
}

// InterpretLabResults interprets a list of lab results.
func (l *LabInterpreter) InterpretLabResults(labs []LabResult, age int, sex string) []*LabInterpretation {
	interpretations := make([]*LabInterpretation, len(labs))
	for i, lab := range labs {
		interpretations[i] = l.InterpretLabResult(lab, age, sex)
	}
	return interpretations
}
