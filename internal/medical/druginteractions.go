package medical

import (
	"strings"
)

// DrugInteractionChecker checks for drug interactions.
type DrugInteractionChecker struct {
	interactions map[string]map[string]DrugInteraction
	drugClasses  map[string]string
}

// DrugInteraction represents a drug-drug interaction.
type DrugInteraction struct {
	Drug1           string `json:"drug1"`
	Drug2           string `json:"drug2"`
	Severity        string `json:"severity"` // mild, moderate, severe, contraindicated
	Description     string `json:"description"`
	Mechanism       string `json:"mechanism"`
	Recommendation  string `json:"recommendation"`
	Monitoring      string `json:"monitoring"`
}

// DrugAllergy represents a drug allergy.
type DrugAllergy struct {
	Drug       string `json:"drug"`
	Allergen   string `json:"allergen"`
	Severity   string `json:"severity"`
	Reaction   string `json:"reaction"`
}

// DrugDiseaseInteraction represents a drug-disease contraindication.
type DrugDiseaseInteraction struct {
	Drug          string `json:"drug"`
	Disease       string `json:"disease"`
	Severity      string `json:"severity"`
	Description   string `json:"description"`
	Recommendation string `json:"recommendation"`
}

// InteractionCheckResult contains the results of an interaction check.
type InteractionCheckResult struct {
	DrugDrugInteractions      []DrugInteraction        `json:"drug_drug_interactions"`
	DrugAllergyInteractions   []DrugAllergy            `json:"drug_allergy_interactions"`
	DrugDiseaseInteractions   []DrugDiseaseInteraction `json:"drug_disease_interactions"`
	TotalInteractions         int                      `json:"total_interactions"`
	SevereInteractions        int                      `json:"severe_interactions"`
	Contraindications         int                      `json:"contraindications"`
}

// NewDrugInteractionChecker creates a new drug interaction checker.
func NewDrugInteractionChecker() *DrugInteractionChecker {
	checker := &DrugInteractionChecker{
		interactions: make(map[string]map[string]DrugInteraction),
		drugClasses:  make(map[string]string),
	}
	checker.loadInteractionDatabase()
	return checker
}

// loadInteractionDatabase loads the drug interaction database.
func (d *DrugInteractionChecker) loadInteractionDatabase() {
	// Warfarin interactions
	d.addInteraction("warfarin", "aspirin", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased bleeding risk",
		Mechanism:     "Pharmacodynamic synergy - both drugs impair hemostasis",
		Recommendation: "Avoid combination if possible. If necessary, use lowest effective doses and monitor INR closely.",
		Monitoring:    "Monitor INR frequently, watch for signs of bleeding",
	})

	d.addInteraction("warfarin", "ibuprofen", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased bleeding risk and GI toxicity",
		Mechanism:     "NSAIDs impair platelet function and may increase warfarin levels",
		Recommendation: "Avoid NSAIDs with warfarin. Use acetaminophen instead.",
		Monitoring:    "Monitor INR, watch for GI bleeding",
	})

	d.addInteraction("warfarin", "naproxen", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased bleeding risk and GI toxicity",
		Mechanism:     "NSAIDs impair platelet function and may increase warfarin levels",
		Recommendation: "Avoid NSAIDs with warfarin. Use acetaminophen instead.",
		Monitoring:    "Monitor INR, watch for GI bleeding",
	})

	// QT prolongation interactions
	d.addInteraction("amiodarone", "azithromycin", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased risk of QT prolongation and torsades de pointes",
		Mechanism:     "Both drugs prolong QT interval",
		Recommendation: "Avoid combination. If necessary, monitor ECG closely.",
		Monitoring:    "ECG monitoring, electrolytes",
	})

	d.addInteraction("amiodarone", "fluconazole", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased risk of QT prolongation",
		Mechanism:     "Both drugs prolong QT interval, fluconazole may increase amiodarone levels",
		Recommendation: "Avoid combination or monitor ECG closely",
		Monitoring:    "ECG monitoring",
	})

	d.addInteraction("amiodarone", "ciprofloxacin", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased risk of QT prolongation",
		Mechanism:     "Both drugs prolong QT interval",
		Recommendation: "Avoid combination or use alternative antibiotics",
		Monitoring:    "ECG monitoring",
	})

	// ACE inhibitor + potassium interactions
	d.addInteraction("lisinopril", "potassium", DrugInteraction{
		Severity:      "moderate",
		Description:   "Risk of hyperkalemia",
		Mechanism:     "ACE inhibitors reduce aldosterone, decreasing potassium excretion",
		Recommendation: "Monitor potassium levels closely. Avoid potassium supplements unless necessary.",
		Monitoring:    "Monitor serum potassium",
	})

	d.addInteraction("enalapril", "spironolactone", DrugInteraction{
		Severity:      "severe",
		Description:   "High risk of hyperkalemia",
		Mechanism:     "Both drugs increase potassium levels",
		Recommendation: "Use with caution. Monitor potassium frequently.",
		Monitoring:    "Frequent potassium monitoring",
	})

	// Statin interactions
	d.addInteraction("simvastatin", "clarithromycin", DrugInteraction{
		Severity:      "contraindicated",
		Description:   "Increased risk of rhabdomyolysis",
		Mechanism:     "Clarithromycin inhibits CYP3A4, increasing statin levels",
		Recommendation: "Contraindicated. Use alternative antibiotic or hold statin.",
		Monitoring:    "Watch for muscle pain, CK levels",
	})

	d.addInteraction("atorvastatin", "itraconazole", DrugInteraction{
		Severity:      "severe",
		Description:   "Increased risk of rhabdomyolysis",
		Mechanism:     "Itraconazole inhibits CYP3A4, increasing statin levels",
		Recommendation: "Avoid combination or use lowest statin dose",
		Monitoring:    "Monitor for muscle symptoms, CK levels",
	})

	// Metformin interactions
	d.addInteraction("metformin", "contrast dye", DrugInteraction{
		Severity:      "moderate",
		Description:   "Risk of lactic acidosis",
		Mechanism:     "Contrast dye may impair renal function, reducing metformin clearance",
		Recommendation: "Hold metformin 48 hours before and after contrast procedures",
		Monitoring:    "Renal function, hold metformin",
	})

	// Beta-blocker interactions
	d.addInteraction("metoprolol", "verapamil", DrugInteraction{
		Severity:      "severe",
		Description:   "Risk of severe bradycardia and heart block",
		Mechanism:     "Both drugs depress AV conduction",
		Recommendation: "Avoid combination. Use alternative agents.",
		Monitoring:    "Heart rate, ECG",
	})

	d.addInteraction("atenolol", "diltiazem", DrugInteraction{
		Severity:      "severe",
		Description:   "Risk of severe bradycardia and heart block",
		Mechanism:     "Both drugs depress AV conduction",
		Recommendation: "Avoid combination or monitor closely",
		Monitoring:    "Heart rate, ECG",
	})

	// SSRI interactions
	d.addInteraction("fluoxetine", "tramadol", DrugInteraction{
		Severity:      "severe",
		Description:   "Risk of serotonin syndrome",
		Mechanism:     "Both drugs increase serotonin levels",
		Recommendation: "Avoid combination or use lowest doses. Monitor for serotonin syndrome.",
		Monitoring:    "Watch for agitation, hyperthermia, tremor",
	})

	d.addInteraction("sertraline", "MAOIs", DrugInteraction{
		Severity:      "contraindicated",
		Description:   "Risk of fatal serotonin syndrome",
		Mechanism:     "MAOIs prevent serotonin breakdown, SSRIs increase serotonin",
		Recommendation: "Contraindicated. Wait 14 days between drugs.",
		Monitoring:    "Contraindicated",
	})

	// Load drug classes
	d.drugClasses["warfarin"] = "anticoagulant"
	d.drugClasses["aspirin"] = "nsaid"
	d.drugClasses["ibuprofen"] = "nsaid"
	d.drugClasses["naproxen"] = "nsaid"
	d.drugClasses["amiodarone"] = "antiarrhythmic"
	d.drugClasses["azithromycin"] = "macrolide"
	d.drugClasses["fluconazole"] = "azole_antifungal"
	d.drugClasses["ciprofloxacin"] = "fluoroquinolone"
	d.drugClasses["lisinopril"] = "ace_inhibitor"
	d.drugClasses["enalapril"] = "ace_inhibitor"
	d.drugClasses["simvastatin"] = "statin"
	d.drugClasses["atorvastatin"] = "statin"
	d.drugClasses["metformin"] = "biguanide"
	d.drugClasses["metoprolol"] = "beta_blocker"
	d.drugClasses["atenolol"] = "beta_blocker"
	d.drugClasses["fluoxetine"] = "ssri"
	d.drugClasses["sertraline"] = "ssri"
}

// addInteraction adds a drug interaction to the database.
func (d *DrugInteractionChecker) addInteraction(drug1, drug2 string, interaction DrugInteraction) {
	drug1Lower := strings.ToLower(drug1)
	drug2Lower := strings.ToLower(drug2)

	interaction.Drug1 = drug1
	interaction.Drug2 = drug2

	if d.interactions[drug1Lower] == nil {
		d.interactions[drug1Lower] = make(map[string]DrugInteraction)
	}
	d.interactions[drug1Lower][drug2Lower] = interaction

	// Add reverse interaction
	if d.interactions[drug2Lower] == nil {
		d.interactions[drug2Lower] = make(map[string]DrugInteraction)
	}
	d.interactions[drug2Lower][drug1Lower] = interaction
}

// CheckInteractions checks for interactions between a list of medications.
func (d *DrugInteractionChecker) CheckInteractions(medications []Medication, allergies []string, conditions []string) *InteractionCheckResult {
	result := &InteractionCheckResult{}

	// Check drug-drug interactions
	for i := 0; i < len(medications); i++ {
		for j := i + 1; j < len(medications); j++ {
			drug1 := strings.ToLower(medications[i].Name)
			drug2 := strings.ToLower(medications[j].Name)

			if interaction, exists := d.interactions[drug1][drug2]; exists {
				result.DrugDrugInteractions = append(result.DrugDrugInteractions, interaction)
				result.TotalInteractions++

				if interaction.Severity == "severe" || interaction.Severity == "contraindicated" {
					result.SevereInteractions++
				}
				if interaction.Severity == "contraindicated" {
					result.Contraindications++
				}
			}
		}
	}

	// Check drug-allergy interactions
	for _, med := range medications {
		medLower := strings.ToLower(med.Name)
		for _, allergy := range allergies {
			allergyLower := strings.ToLower(allergy)
			if strings.Contains(medLower, allergyLower) || strings.Contains(allergyLower, medLower) {
				result.DrugAllergyInteractions = append(result.DrugAllergyInteractions, DrugAllergy{
					Drug:     med.Name,
					Allergen: allergy,
					Severity: "severe",
					Reaction: "Potential allergic reaction",
				})
				result.TotalInteractions++
				result.SevereInteractions++
			}
		}
	}

	// Check drug-disease interactions
	for _, med := range medications {
		medLower := strings.ToLower(med.Name)
		
		// ACE inhibitors + renal failure
		if d.drugClasses[medLower] == "ace_inhibitor" {
			for _, condition := range conditions {
				conditionLower := strings.ToLower(condition)
				if strings.Contains(conditionLower, "renal") || strings.Contains(conditionLower, "kidney") {
					result.DrugDiseaseInteractions = append(result.DrugDiseaseInteractions, DrugDiseaseInteraction{
						Drug:          med.Name,
						Disease:       condition,
						Severity:      "moderate",
						Description:   "ACE inhibitors may worsen renal function",
						Recommendation: "Monitor renal function closely. Consider dose adjustment.",
					})
					result.TotalInteractions++
				}
			}
		}

		// Beta-blockers + asthma
		if d.drugClasses[medLower] == "beta_blocker" {
			for _, condition := range conditions {
				conditionLower := strings.ToLower(condition)
				if strings.Contains(conditionLower, "asthma") || strings.Contains(conditionLower, "copd") {
					result.DrugDiseaseInteractions = append(result.DrugDiseaseInteractions, DrugDiseaseInteraction{
						Drug:          med.Name,
						Disease:       condition,
						Severity:      "severe",
						Description:   "Beta-blockers may cause bronchospasm",
						Recommendation: "Use cardioselective beta-blocker or alternative. Monitor respiratory status.",
					})
					result.TotalInteractions++
					result.SevereInteractions++
				}
			}
		}

		// Metformin + renal failure
		if d.drugClasses[medLower] == "biguanide" {
			for _, condition := range conditions {
				conditionLower := strings.ToLower(condition)
				if strings.Contains(conditionLower, "renal") || strings.Contains(conditionLower, "kidney") {
					result.DrugDiseaseInteractions = append(result.DrugDiseaseInteractions, DrugDiseaseInteraction{
						Drug:          med.Name,
						Disease:       condition,
						Severity:      "severe",
						Description:   "Metformin contraindicated in severe renal impairment",
						Recommendation: "Check eGFR. Hold if eGFR < 30. Monitor renal function.",
					})
					result.TotalInteractions++
					result.SevereInteractions++
				}
			}
		}
	}

	return result
}
