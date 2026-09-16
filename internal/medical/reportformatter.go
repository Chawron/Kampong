package medical

import (
	"fmt"
	"strings"
	"time"
)

// MedicalReportFormatter formats medical debate results into standard medical report formats.
type MedicalReportFormatter struct{}

// NewMedicalReportFormatter creates a new medical report formatter.
func NewMedicalReportFormatter() *MedicalReportFormatter {
	return &MedicalReportFormatter{}
}

// SOAPNote represents a SOAP note format.
type SOAPNote struct {
	Subjective  string `json:"subjective"`
	Objective   string `json:"objective"`
	Assessment  string `json:"assessment"`
	Plan        string `json:"plan"`
}

// DischargeSummary represents a discharge summary format.
type DischargeSummary struct {
	AdmissionDate    string   `json:"admission_date"`
	DischargeDate    string   `json:"discharge_date"`
	AdmissionDiagnosis string `json:"admission_diagnosis"`
	DischargeDiagnosis string `json:"discharge_diagnosis"`
	PrincipalProcedure string `json:"principal_procedure"`
	HospitalCourse   string   `json:"hospital_course"`
	DischargeCondition string `json:"discharge_condition"`
	DischargeMedications []string `json:"discharge_medications"`
	FollowUpInstructions string `json:"follow_up_instructions"`
}

// ConsultationReport represents a consultation report format.
type ConsultationReport struct {
	ConsultingService string `json:"consulting_service"`
	DateOfConsultation string `json:"date_of_consultation"`
	ReasonForConsultation string `json:"reason_for_consultation"`
	HistoryOfPresentIllness string `json:"history_of_present_illness"`
	PastMedicalHistory string `json:"past_medical_history"`
	PhysicalExamination string `json:"physical_examination"`
	LaboratoryData     string `json:"laboratory_data"`
	Impression         string `json:"impression"`
	Recommendations    []string `json:"recommendations"`
}

// FormatSOAPNote formats a medical case as a SOAP note.
func (f *MedicalReportFormatter) FormatSOAPNote(caseData *MedicalCase, debateResult *MedicalDebateResult) *SOAPNote {
	note := &SOAPNote{}

	// Subjective: Patient's symptoms and history
	var subjective strings.Builder
	subjective.WriteString("CHIEF COMPLAINT: ")
	if len(caseData.Symptoms) > 0 {
		subjective.WriteString(caseData.Symptoms[0].Name)
	}
	subjective.WriteString("\n\n")

	subjective.WriteString("HISTORY OF PRESENT ILLNESS: ")
	for _, symptom := range caseData.Symptoms {
		subjective.WriteString(fmt.Sprintf("%s (severity %d/10", symptom.Name, symptom.Severity))
		if symptom.Duration != "" {
			subjective.WriteString(fmt.Sprintf(", duration: %s", symptom.Duration))
		}
		subjective.WriteString("). ")
	}
	subjective.WriteString("\n\n")

	subjective.WriteString("PAST MEDICAL HISTORY: ")
	if len(caseData.MedicalHistory.PastConditions) > 0 {
		subjective.WriteString(strings.Join(caseData.MedicalHistory.PastConditions, ", "))
	}
	subjective.WriteString("\n\n")

	subjective.WriteString("MEDICATIONS: ")
	if len(caseData.CurrentMeds) > 0 {
		for _, med := range caseData.CurrentMeds {
			subjective.WriteString(fmt.Sprintf("%s %s %s; ", med.Name, med.Dose, med.Frequency))
		}
	}
	subjective.WriteString("\n\n")

	subjective.WriteString("ALLERGIES: ")
	if len(caseData.Allergies) > 0 {
		subjective.WriteString(strings.Join(caseData.Allergies, ", "))
	} else {
		subjective.WriteString("NKDA")
	}

	note.Subjective = subjective.String()

	// Objective: Vital signs and lab results
	var objective strings.Builder
	objective.WriteString("VITAL SIGNS:\n")
	if caseData.VitalSigns.Temperature > 0 {
		objective.WriteString(fmt.Sprintf("Temperature: %.1f°C\n", caseData.VitalSigns.Temperature))
	}
	if caseData.VitalSigns.HeartRate > 0 {
		objective.WriteString(fmt.Sprintf("Heart Rate: %d bpm\n", caseData.VitalSigns.HeartRate))
	}
	if caseData.VitalSigns.BloodPressure != "" {
		objective.WriteString(fmt.Sprintf("Blood Pressure: %s\n", caseData.VitalSigns.BloodPressure))
	}
	if caseData.VitalSigns.RespiratoryRate > 0 {
		objective.WriteString(fmt.Sprintf("Respiratory Rate: %d /min\n", caseData.VitalSigns.RespiratoryRate))
	}
	if caseData.VitalSigns.SpO2 > 0 {
		objective.WriteString(fmt.Sprintf("SpO2: %d%%\n", caseData.VitalSigns.SpO2))
	}
	objective.WriteString("\n")

	if len(caseData.LabResults) > 0 {
		objective.WriteString("LABORATORY RESULTS:\n")
		for _, lab := range caseData.LabResults {
			flag := ""
			if lab.Flag == "high" {
				flag = " (H)"
			} else if lab.Flag == "low" {
				flag = " (L)"
			}
			objective.WriteString(fmt.Sprintf("%s: %.2f %s%s (Ref: %.2f-%.2f)\n",
				lab.TestName, lab.Value, lab.Unit, flag, lab.ReferenceMin, lab.ReferenceMax))
		}
	}

	note.Objective = objective.String()

	// Assessment: Diagnosis from debate result
	var assessment strings.Builder
	assessment.WriteString("ASSESSMENT:\n")
	if debateResult != nil && len(debateResult.DifferentialDiagnosis) > 0 {
		for i, dx := range debateResult.DifferentialDiagnosis {
			assessment.WriteString(fmt.Sprintf("%d. %s (Probability: %.0f%%, Confidence: %s)\n",
				i+1, dx.Name, dx.Probability*100, dx.Confidence))
		}
	} else {
		assessment.WriteString("Diagnosis pending further evaluation.\n")
	}

	note.Assessment = assessment.String()

	// Plan: Treatment recommendations
	var plan strings.Builder
	plan.WriteString("PLAN:\n")
	if debateResult != nil {
		if len(debateResult.RecommendedWorkup) > 0 {
			plan.WriteString("Diagnostic Workup:\n")
			for _, workup := range debateResult.RecommendedWorkup {
				plan.WriteString(fmt.Sprintf("- %s\n", workup))
			}
			plan.WriteString("\n")
		}

		if debateResult.TreatmentPlan.Medications != nil {
			plan.WriteString("Medications:\n")
			for _, med := range debateResult.TreatmentPlan.Medications {
				plan.WriteString(fmt.Sprintf("- %s\n", med))
			}
			plan.WriteString("\n")
		}

		if len(debateResult.TreatmentPlan.FollowUp) > 0 {
			plan.WriteString(fmt.Sprintf("Follow-up: %s\n", debateResult.TreatmentPlan.FollowUp))
		}
	}

	note.Plan = plan.String()

	return note
}

// FormatDischargeSummary formats a medical case as a discharge summary.
func (f *MedicalReportFormatter) FormatDischargeSummary(caseData *MedicalCase, debateResult *MedicalDebateResult) *DischargeSummary {
	summary := &DischargeSummary{
		AdmissionDate:  time.Now().AddDate(0, 0, -7).Format("2006-01-02"),
		DischargeDate:  time.Now().Format("2006-01-02"),
	}

	// Admission/Discharge Diagnosis
	if debateResult != nil && len(debateResult.DifferentialDiagnosis) > 0 {
		summary.AdmissionDiagnosis = debateResult.DifferentialDiagnosis[0].Name
		summary.DischargeDiagnosis = debateResult.DifferentialDiagnosis[0].Name
	}

	// Hospital Course
	var course strings.Builder
	course.WriteString("The patient was admitted with ")
	if len(caseData.Symptoms) > 0 {
		course.WriteString(caseData.Symptoms[0].Name)
	}
	course.WriteString(". During the hospitalization, ")

	if debateResult != nil {
		course.WriteString("the medical team conducted a comprehensive evaluation including ")
		if len(debateResult.RecommendedWorkup) > 0 {
			course.WriteString(strings.Join(debateResult.RecommendedWorkup[:min(3, len(debateResult.RecommendedWorkup))], ", "))
		}
		course.WriteString(". ")

		if len(debateResult.DifferentialDiagnosis) > 0 {
			course.WriteString(fmt.Sprintf("The primary diagnosis was %s. ", debateResult.DifferentialDiagnosis[0].Name))
		}

		course.WriteString("The patient received appropriate treatment and showed clinical improvement. ")
	}

	course.WriteString("The patient was stable for discharge.")
	summary.HospitalCourse = course.String()

	// Discharge Condition
	summary.DischargeCondition = "Stable for discharge"

	// Discharge Medications
	if debateResult != nil && debateResult.TreatmentPlan.Medications != nil {
		summary.DischargeMedications = debateResult.TreatmentPlan.Medications
	}

	// Follow-up Instructions
	if debateResult != nil && debateResult.TreatmentPlan.FollowUp != "" {
		summary.FollowUpInstructions = debateResult.TreatmentPlan.FollowUp
	} else {
		summary.FollowUpInstructions = "Follow up with primary care physician in 1-2 weeks."
	}

	return summary
}

// FormatConsultationReport formats a medical case as a consultation report.
func (f *MedicalReportFormatter) FormatConsultationReport(caseData *MedicalCase, debateResult *MedicalDebateResult, consultingService string) *ConsultationReport {
	report := &ConsultationReport{
		ConsultingService:    consultingService,
		DateOfConsultation:   time.Now().Format("2006-01-02"),
	}

	// Reason for Consultation
	if len(caseData.Symptoms) > 0 {
		report.ReasonForConsultation = fmt.Sprintf("Evaluation and management of %s", caseData.Symptoms[0].Name)
	}

	// History of Present Illness
	var hpi strings.Builder
	for _, symptom := range caseData.Symptoms {
		hpi.WriteString(fmt.Sprintf("%s (severity %d/10", symptom.Name, symptom.Severity))
		if symptom.Duration != "" {
			hpi.WriteString(fmt.Sprintf(", duration: %s", symptom.Duration))
		}
		if symptom.Location != "" {
			hpi.WriteString(fmt.Sprintf(", location: %s", symptom.Location))
		}
		hpi.WriteString("). ")
	}
	report.HistoryOfPresentIllness = hpi.String()

	// Past Medical History
	if len(caseData.MedicalHistory.PastConditions) > 0 {
		report.PastMedicalHistory = strings.Join(caseData.MedicalHistory.PastConditions, ", ")
	}

	// Laboratory Data
	if len(caseData.LabResults) > 0 {
		var labs strings.Builder
		for _, lab := range caseData.LabResults {
			flag := ""
			if lab.Flag == "high" {
				flag = " (H)"
			} else if lab.Flag == "low" {
				flag = " (L)"
			}
			labs.WriteString(fmt.Sprintf("%s: %.2f %s%s; ", lab.TestName, lab.Value, lab.Unit, flag))
		}
		report.LaboratoryData = labs.String()
	}

	// Impression
	if debateResult != nil && len(debateResult.DifferentialDiagnosis) > 0 {
		var impression strings.Builder
		for i, dx := range debateResult.DifferentialDiagnosis {
			impression.WriteString(fmt.Sprintf("%d. %s (Probability: %.0f%%)\n", i+1, dx.Name, dx.Probability*100))
		}
		report.Impression = impression.String()
	}

	// Recommendations
	if debateResult != nil {
		var recommendations []string
		if len(debateResult.TreatmentPlan.Procedures) > 0 {
			recommendations = append(recommendations, debateResult.TreatmentPlan.Procedures...)
		}
		if len(debateResult.TreatmentPlan.Lifestyle) > 0 {
			recommendations = append(recommendations, debateResult.TreatmentPlan.Lifestyle...)
		}
		if len(debateResult.TreatmentPlan.Referrals) > 0 {
			recommendations = append(recommendations, debateResult.TreatmentPlan.Referrals...)
		}
		report.Recommendations = recommendations
	}

	return report
}

// FormatMedicalReport formats a complete medical report with all sections.
func (f *MedicalReportFormatter) FormatMedicalReport(caseData *MedicalCase, debateResult *MedicalDebateResult) string {
	var sb strings.Builder

	sb.WriteString("# MEDICAL DEBATE REPORT\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", time.Now().Format(time.RFC3339)))

	// Patient Information
	sb.WriteString("## PATIENT INFORMATION\n\n")
	sb.WriteString(fmt.Sprintf("- **Age:** %d years\n", caseData.PatientInfo.Age))
	sb.WriteString(fmt.Sprintf("- **Sex:** %s\n", caseData.PatientInfo.Sex))
	if caseData.PatientInfo.BMI > 0 {
		sb.WriteString(fmt.Sprintf("- **BMI:** %.1f kg/m²\n", caseData.PatientInfo.BMI))
	}
	sb.WriteString("\n")

	// SOAP Note
	sb.WriteString("## SOAP NOTE\n\n")
	soap := f.FormatSOAPNote(caseData, debateResult)
	sb.WriteString("### Subjective\n")
	sb.WriteString(soap.Subjective + "\n\n")
	sb.WriteString("### Objective\n")
	sb.WriteString(soap.Objective + "\n\n")
	sb.WriteString("### Assessment\n")
	sb.WriteString(soap.Assessment + "\n\n")
	sb.WriteString("### Plan\n")
	sb.WriteString(soap.Plan + "\n\n")

	// Differential Diagnosis
	if debateResult != nil && len(debateResult.DifferentialDiagnosis) > 0 {
		sb.WriteString("## DIFFERENTIAL DIAGNOSIS\n\n")
		for i, dx := range debateResult.DifferentialDiagnosis {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, dx.Name))
			sb.WriteString(fmt.Sprintf("   - Probability: %.0f%%\n", dx.Probability*100))
			sb.WriteString(fmt.Sprintf("   - Confidence: %s\n", dx.Confidence))
			if dx.ICD10Code != "" {
				sb.WriteString(fmt.Sprintf("   - ICD-10: %s\n", dx.ICD10Code))
			}
			sb.WriteString("\n")
		}
	}

	// Red Flags
	if debateResult != nil && len(debateResult.RedFlags) > 0 {
		sb.WriteString("## RED FLAGS\n\n")
		for _, flag := range debateResult.RedFlags {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", flag.Symptom, flag.Severity, flag.Description))
			sb.WriteString(fmt.Sprintf("  - Action: %s\n", flag.Action))
		}
		sb.WriteString("\n")
	}

	// Evidence Summary
	if debateResult != nil && debateResult.EvidenceSummary != "" {
		sb.WriteString("## EVIDENCE SUMMARY\n\n")
		sb.WriteString(debateResult.EvidenceSummary + "\n\n")
	}

	// Disclaimer
	sb.WriteString("---\n\n")
	sb.WriteString("**DISCLAIMER:** This report is generated by an AI debate system for educational purposes only. ")
	sb.WriteString("It does not constitute medical advice. Always consult qualified healthcare professionals for actual medical decisions.\n")

	return sb.String()
}

// ExportToHTML exports a medical report to HTML format.
func (f *MedicalReportFormatter) ExportToHTML(markdownReport string) string {
	// Simple markdown to HTML conversion
	html := markdownReport
	html = strings.ReplaceAll(html, "# ", "<h1>")
	html = strings.ReplaceAll(html, "## ", "<h2>")
	html = strings.ReplaceAll(html, "### ", "<h3>")
	html = strings.ReplaceAll(html, "\n\n", "</p><p>")
	html = strings.ReplaceAll(html, "**", "<strong>")
	html = strings.ReplaceAll(html, "\n", "<br>")

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Medical Debate Report</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 20px; line-height: 1.6; }
        h1 { color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
        h2 { color: #34495e; margin-top: 30px; }
        h3 { color: #7f8c8d; }
        strong { color: #2c3e50; }
        p { margin: 10px 0; }
        .disclaimer { background: #fff3cd; border: 1px solid #ffc107; padding: 15px; border-radius: 5px; margin-top: 30px; }
    </style>
</head>
<body>
    <p>%s</p>
</body>
</html>`, html)
}
