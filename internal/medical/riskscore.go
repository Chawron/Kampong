package medical

import (
	"fmt"
	"strings"

	"github.com/kampong/debate/internal/models"
)

// ComputeDynamicRiskScore calculates a composite risk score from all available data.
// This is a RULE-BASED computation (no LLM needed) combining:
// - Red flag severity (from redflags.go)
// - Drug interaction severity (from druginteractions.go)
// - Lab abnormality severity (from labinterpreter.go)
// - Age risk (elderly/very young = higher)
// - History risk (comorbidities)
func ComputeDynamicRiskScore(caseData *MedicalCase, redFlags []RedFlag, interactions *InteractionCheckResult, labInterps []LabInterpretation) *models.DynamicRiskScore {
	score := &models.DynamicRiskScore{}
	var factors []string

	// ── Red flag score (0–100) ──
	score.RedFlagScore = computeRedFlagScore(redFlags, &factors)

	// ── Drug risk score (0–100) ──
	score.DrugRiskScore = computeDrugRiskScore(interactions, &factors)

	// ── Lab risk score (0–100) ──
	score.LabRiskScore = computeLabRiskScore(labInterps, &factors)

	// ── Age risk score (0–100) ──
	score.AgeRiskScore = computeAgeRiskScore(caseData.PatientInfo.Age, &factors)

	// ── History risk score (0–100) ──
	score.HistoryRiskScore = computeHistoryRiskScore(caseData, &factors)

	// ── Weighted total ──
	// red_flag(30%) + drug(20%) + lab(25%) + age(10%) + history(15%)
	total := float64(score.RedFlagScore)*0.30 +
		float64(score.DrugRiskScore)*0.20 +
		float64(score.LabRiskScore)*0.25 +
		float64(score.AgeRiskScore)*0.10 +
		float64(score.HistoryRiskScore)*0.15
	score.TotalScore = int(total + 0.5) // round
	if score.TotalScore > 100 {
		score.TotalScore = 100
	}

	// ── Risk level ──
	switch {
	case score.TotalScore > 80:
		score.RiskLevel = "critical"
	case score.TotalScore > 60:
		score.RiskLevel = "high"
	case score.TotalScore > 40:
		score.RiskLevel = "moderate"
	default:
		score.RiskLevel = "low"
	}

	// ── Contributing factors ──
	score.ContributingFactors = factors

	// ── Immediate actions ──
	score.ImmediateActions = generateImmediateActions(score)

	return score
}

// computeRedFlagScore returns 0–100 based on the worst red flag severity.
func computeRedFlagScore(flags []RedFlag, factors *[]string) int {
	if len(flags) == 0 {
		return 0
	}

	maxScore := 0
	criticalCount := 0
	urgentCount := 0

	for _, rf := range flags {
		switch strings.ToLower(rf.Severity) {
		case "critical", "emergency":
			maxScore = 100
			criticalCount++
		case "urgent":
			if maxScore < 70 {
				maxScore = 70
			}
			urgentCount++
		case "important":
			if maxScore < 40 {
				maxScore = 40
			}
		}
	}

	if criticalCount > 0 {
		*factors = append(*factors, fmt.Sprintf("%d critical red flag(s) detected", criticalCount))
	}
	if urgentCount > 0 {
		*factors = append(*factors, fmt.Sprintf("%d urgent red flag(s) detected", urgentCount))
	}

	return maxScore
}

// computeDrugRiskScore returns 0–100 based on interaction severity.
func computeDrugRiskScore(result *InteractionCheckResult, factors *[]string) int {
	if result == nil {
		return 0
	}

	maxScore := 0

	// Check drug-drug interactions
	for _, di := range result.DrugDrugInteractions {
		switch strings.ToLower(di.Severity) {
		case "contraindicated":
			if maxScore < 100 {
				maxScore = 100
			}
			*factors = append(*factors, fmt.Sprintf("Contraindicated: %s + %s", di.Drug1, di.Drug2))
		case "severe":
			if maxScore < 100 {
				maxScore = 100
			}
			*factors = append(*factors, fmt.Sprintf("Severe interaction: %s + %s", di.Drug1, di.Drug2))
		case "moderate":
			if maxScore < 60 {
				maxScore = 60
			}
			*factors = append(*factors, fmt.Sprintf("Moderate interaction: %s + %s", di.Drug1, di.Drug2))
		case "mild":
			if maxScore < 30 {
				maxScore = 30
			}
		}
	}

	// Drug-allergy interactions are always severe
	if len(result.DrugAllergyInteractions) > 0 {
		maxScore = 100
		for _, ai := range result.DrugAllergyInteractions {
			*factors = append(*factors, fmt.Sprintf("Allergy risk: %s + %s", ai.Drug, ai.Allergen))
		}
	}

	// Drug-disease interactions
	for _, di := range result.DrugDiseaseInteractions {
		switch strings.ToLower(di.Severity) {
		case "severe", "contraindicated":
			if maxScore < 100 {
				maxScore = 100
			}
			*factors = append(*factors, fmt.Sprintf("Contraindication: %s in %s", di.Drug, di.Disease))
		case "moderate":
			if maxScore < 60 {
				maxScore = 60
			}
		}
	}

	return maxScore
}

// computeLabRiskScore returns 0–100 based on the worst lab abnormality.
func computeLabRiskScore(interps []LabInterpretation, factors *[]string) int {
	maxScore := 0
	criticalCount := 0
	abnormalCount := 0

	for _, interp := range interps {
		switch interp.Flag {
		case "critical_low", "critical_high":
			if maxScore < 100 {
				maxScore = 100
			}
			criticalCount++
			*factors = append(*factors, fmt.Sprintf("Critical: %s = %.2f %s", interp.TestName, interp.Value, interp.Unit))
		case "low", "high":
			if maxScore < 60 {
				maxScore = 60
			}
			abnormalCount++
		}
	}

	if criticalCount > 0 {
		*factors = append(*factors, fmt.Sprintf("%d critical lab value(s)", criticalCount))
	}
	if abnormalCount > 0 {
		*factors = append(*factors, fmt.Sprintf("%d abnormal lab value(s)", abnormalCount))
	}

	return maxScore
}

// computeAgeRiskScore returns 0–100 based on patient age.
// Age 0 or negative means the age was not provided — treat it as unknown
// rather than classifying the patient as an infant.
func computeAgeRiskScore(age int, factors *[]string) int {
	switch {
	case age <= 0:
		// Age not provided: no age-based risk contribution.
		return 0
	case age > 80:
		*factors = append(*factors, fmt.Sprintf("Advanced age: %d years (>80)", age))
		return 80
	case age < 2:
		*factors = append(*factors, fmt.Sprintf("Infant: %d years (<2)", age))
		return 80
	case age > 65:
		*factors = append(*factors, fmt.Sprintf("Elderly: %d years (>65)", age))
		return 50
	case age < 12:
		*factors = append(*factors, fmt.Sprintf("Pediatric: %d years (<12)", age))
		return 50
	default:
		return 10
	}
}

// computeHistoryRiskScore returns 0–100 based on comorbidity count.
func computeHistoryRiskScore(caseData *MedicalCase, factors *[]string) int {
	count := len(caseData.MedicalHistory.PastConditions)

	if count == 0 {
		return 0
	}

	// 0→0, 1→20, 2→40, 3→60, 4→80, 5+→100
	score := count * 20
	if score > 100 {
		score = 100
	}

	*factors = append(*factors, fmt.Sprintf("%d comorbidity/comorbidities in history", count))

	// High-risk conditions get extra weight
	highRiskConditions := []string{
		"diabetes", "heart failure", "chronic kidney disease", "copd",
		"liver cirrhosis", "cancer", "immunocompromised", "hiv",
		"transplant", "stroke", "myocardial infarction",
	}

	highRiskCount := 0
	for _, cond := range caseData.MedicalHistory.PastConditions {
		condLower := strings.ToLower(cond)
		for _, hrc := range highRiskConditions {
			if strings.Contains(condLower, hrc) {
				highRiskCount++
				break
			}
		}
	}

	if highRiskCount > 0 {
		*factors = append(*factors, fmt.Sprintf("%d high-risk condition(s) in history", highRiskCount))
		// Boost score by 10 per high-risk condition, capped at 100
		score += highRiskCount * 10
		if score > 100 {
			score = 100
		}
	}

	return score
}

// generateImmediateActions returns action items based on the risk level.
func generateImmediateActions(score *models.DynamicRiskScore) []string {
	var actions []string

	switch score.RiskLevel {
	case "critical":
		actions = append(actions, "Immediate physician evaluation required")
		if score.RedFlagScore >= 100 {
			actions = append(actions, "Activate emergency response")
		}
		if score.DrugRiskScore >= 100 {
			actions = append(actions, "Review and discontinue contraindicated medications immediately")
		}
		if score.LabRiskScore >= 100 {
			actions = append(actions, "Stat repeat labs and clinical correlation")
		}
	case "high":
		actions = append(actions, "Urgent physician review within 1 hour")
		if score.DrugRiskScore >= 60 {
			actions = append(actions, "Review medication interactions and adjust")
		}
		if score.LabRiskScore >= 60 {
			actions = append(actions, "Repeat abnormal labs and correlate clinically")
		}
	case "moderate":
		actions = append(actions, "Schedule physician review within 24 hours")
		actions = append(actions, "Monitor vital signs closely")
	case "low":
		actions = append(actions, "Routine follow-up as scheduled")
	}

	return actions
}
