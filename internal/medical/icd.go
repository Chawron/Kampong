package medical

import (
	"strings"
)

// ICDCode represents an ICD-10 or ICD-11 diagnosis code.
type ICDCode struct {
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Chapter     string   `json:"chapter"`
	Keywords    []string `json:"keywords"`
	ParentCode  string   `json:"parent_code,omitempty"`
	ICDVersion  string   `json:"icd_version"` // "10" or "11"
}

// ICDCoder provides ICD code matching and lookup.
type ICDCoder struct {
	codes []ICDCode
}

// NewICDCoder creates a new ICD coder with loaded codes.
func NewICDCoder() *ICDCoder {
	coder := &ICDCoder{
		codes: []ICDCode{},
	}
	coder.loadCodes()
	return coder
}

// loadCodes loads ICD-10 codes into the coder.
func (c *ICDCoder) loadCodes() {
	// Cardiovascular diseases (I00-I99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "I10", Description: "Essential (primary) hypertension", Category: "Hypertension", Chapter: "Circulatory", Keywords: []string{"hypertension", "high blood pressure", "htn", "elevated bp"}, ICDVersion: "10"},
		{Code: "I11", Description: "Hypertensive heart disease", Category: "Hypertension", Chapter: "Circulatory", Keywords: []string{"hypertensive heart", "lvh", "left ventricular hypertrophy"}, ICDVersion: "10"},
		{Code: "I21", Description: "Acute myocardial infarction", Category: "Ischemic heart disease", Chapter: "Circulatory", Keywords: []string{"mi", "myocardial infarction", "heart attack", "ami", "stem", "nstemi"}, ICDVersion: "10"},
		{Code: "I25", Description: "Chronic ischemic heart disease", Category: "Ischemic heart disease", Chapter: "Circulatory", Keywords: []string{"cad", "coronary artery disease", "angina", "ischemic heart"}, ICDVersion: "10"},
		{Code: "I50", Description: "Heart failure", Category: "Heart failure", Chapter: "Circulatory", Keywords: []string{"heart failure", "hf", "chf", "congestive heart failure", "cardiomyopathy"}, ICDVersion: "10"},
		{Code: "I48", Description: "Atrial fibrillation and flutter", Category: "Arrhythmia", Chapter: "Circulatory", Keywords: []string{"afib", "atrial fibrillation", "aflutter"}, ICDVersion: "10"},
		{Code: "I63", Description: "Cerebral infarction", Category: "Cerebrovascular", Chapter: "Circulatory", Keywords: []string{"stroke", "cerebral infarction", "ischemic stroke", "cvd"}, ICDVersion: "10"},
		{Code: "I80", Description: "Phlebitis and thrombophlebitis", Category: "Venous", Chapter: "Circulatory", Keywords: []string{"dvt", "deep vein thrombosis", "thrombophlebitis"}, ICDVersion: "10"},
		{Code: "I26", Description: "Pulmonary embolism", Category: "Pulmonary vascular", Chapter: "Circulatory", Keywords: []string{"pe", "pulmonary embolism"}, ICDVersion: "10"},
	}...)

	// Endocrine diseases (E00-E89)
	c.codes = append(c.codes, []ICDCode{
		{Code: "E11", Description: "Type 2 diabetes mellitus", Category: "Diabetes", Chapter: "Endocrine", Keywords: []string{"type 2 diabetes", "t2dm", "diabetes mellitus type 2", "niddm"}, ICDVersion: "10"},
		{Code: "E10", Description: "Type 1 diabetes mellitus", Category: "Diabetes", Chapter: "Endocrine", Keywords: []string{"type 1 diabetes", "t1dm", "diabetes mellitus type 1", "iddm"}, ICDVersion: "10"},
		{Code: "E03", Description: "Other hypothyroidism", Category: "Thyroid", Chapter: "Endocrine", Keywords: []string{"hypothyroidism", "low thyroid", "hashimoto"}, ICDVersion: "10"},
		{Code: "E05", Description: "Thyrotoxicosis (hyperthyroidism)", Category: "Thyroid", Chapter: "Endocrine", Keywords: []string{"hyperthyroidism", "thyrotoxicosis", "graves", "overactive thyroid"}, ICDVersion: "10"},
		{Code: "E78", Description: "Disorders of lipoprotein metabolism", Category: "Lipid", Chapter: "Endocrine", Keywords: []string{"hyperlipidemia", "high cholesterol", "dyslipidemia", "hypercholesterolemia"}, ICDVersion: "10"},
		{Code: "E66", Description: "Obesity", Category: "Nutrition", Chapter: "Endocrine", Keywords: []string{"obesity", "overweight", "obese", "bmi"}, ICDVersion: "10"},
	}...)

	// Respiratory diseases (J00-J99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "J18", Description: "Pneumonia, unspecified organism", Category: "Lower respiratory", Chapter: "Respiratory", Keywords: []string{"pneumonia", "lung infection", "bronchopneumonia"}, ICDVersion: "10"},
		{Code: "J45", Description: "Asthma", Category: "Obstructive", Chapter: "Respiratory", Keywords: []string{"asthma", "reactive airway", "wheezing", "bronchospasm"}, ICDVersion: "10"},
		{Code: "J44", Description: "Other chronic obstructive pulmonary disease", Category: "Obstructive", Chapter: "Respiratory", Keywords: []string{"copd", "chronic obstructive pulmonary disease", "emphysema", "chronic bronchitis"}, ICDVersion: "10"},
		{Code: "J06", Description: "Acute upper respiratory infections", Category: "Upper respiratory", Chapter: "Respiratory", Keywords: []string{"uri", "upper respiratory infection", "common cold", "nasopharyngitis"}, ICDVersion: "10"},
		{Code: "J20", Description: "Acute bronchitis", Category: "Lower respiratory", Chapter: "Respiratory", Keywords: []string{"acute bronchitis", "bronchitis"}, ICDVersion: "10"},
	}...)

	// Neoplasms (C00-D49)
	c.codes = append(c.codes, []ICDCode{
		{Code: "C34", Description: "Malignant neoplasm of bronchus and lung", Category: "Respiratory cancer", Chapter: "Neoplasms", Keywords: []string{"lung cancer", "bronchogenic carcinoma", "pulmonary malignancy"}, ICDVersion: "10"},
		{Code: "C50", Description: "Malignant neoplasm of breast", Category: "Breast cancer", Chapter: "Neoplasms", Keywords: []string{"breast cancer", "breast malignancy", "mammary carcinoma"}, ICDVersion: "10"},
		{Code: "C18", Description: "Malignant neoplasm of colon", Category: "GI cancer", Chapter: "Neoplasms", Keywords: []string{"colon cancer", "colorectal cancer", "bowel cancer"}, ICDVersion: "10"},
		{Code: "C61", Description: "Malignant neoplasm of prostate", Category: "Prostate cancer", Chapter: "Neoplasms", Keywords: []string{"prostate cancer", "prostate malignancy"}, ICDVersion: "10"},
	}...)

	// Mental disorders (F01-F99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "F32", Description: "Depressive episode", Category: "Mood", Chapter: "Mental", Keywords: []string{"depression", "major depression", "depressive episode", "mdd"}, ICDVersion: "10"},
		{Code: "F41", Description: "Other anxiety disorders", Category: "Anxiety", Chapter: "Mental", Keywords: []string{"anxiety", "anxiety disorder", "gad", "generalized anxiety"}, ICDVersion: "10"},
		{Code: "F10", Description: "Mental and behavioral disorders due to alcohol", Category: "Substance", Chapter: "Mental", Keywords: []string{"alcohol", "alcoholism", "alcohol use disorder", "aud"}, ICDVersion: "10"},
	}...)

	// Digestive diseases (K00-K93)
	c.codes = append(c.codes, []ICDCode{
		{Code: "K21", Description: "Gastro-esophageal reflux disease", Category: "Esophageal", Chapter: "Digestive", Keywords: []string{"gerd", "gastroesophageal reflux", "acid reflux", "heartburn"}, ICDVersion: "10"},
		{Code: "K25", Description: "Gastric ulcer", Category: "Ulcer", Chapter: "Digestive", Keywords: []string{"gastric ulcer", "stomach ulcer", "peptic ulcer"}, ICDVersion: "10"},
		{Code: "K80", Description: "Cholelithiasis", Category: "Gallbladder", Chapter: "Digestive", Keywords: []string{"gallstones", "cholelithiasis", "gallbladder stones"}, ICDVersion: "10"},
		{Code: "K70", Description: "Alcoholic liver disease", Category: "Liver", Chapter: "Digestive", Keywords: []string{"alcoholic liver", "alcoholic hepatitis", "cirrhosis alcohol"}, ICDVersion: "10"},
		{Code: "K74", Description: "Fibrosis and cirrhosis of liver", Category: "Liver", Chapter: "Digestive", Keywords: []string{"cirrhosis", "liver fibrosis", "hepatic cirrhosis"}, ICDVersion: "10"},
	}...)

	// Genitourinary diseases (N00-N99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "N18", Description: "Chronic kidney disease", Category: "Kidney", Chapter: "Genitourinary", Keywords: []string{"ckd", "chronic kidney disease", "renal failure", "kidney disease"}, ICDVersion: "10"},
		{Code: "N39", Description: "Other disorders of urinary system", Category: "Urinary", Chapter: "Genitourinary", Keywords: []string{"uti", "urinary tract infection", "cystitis"}, ICDVersion: "10"},
		{Code: "N40", Description: "Hyperplasia of prostate", Category: "Prostate", Chapter: "Genitourinary", Keywords: []string{"bph", "benign prostatic hyperplasia", "enlarged prostate"}, ICDVersion: "10"},
	}...)

	// Musculoskeletal diseases (M00-M99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "M54", Description: "Dorsalgia", Category: "Back", Chapter: "Musculoskeletal", Keywords: []string{"back pain", "dorsalgia", "low back pain", "lbp"}, ICDVersion: "10"},
		{Code: "M17", Description: "Osteoarthritis of knee", Category: "Joint", Chapter: "Musculoskeletal", Keywords: []string{"knee osteoarthritis", "knee oa", "degenerative knee"}, ICDVersion: "10"},
		{Code: "M19", Description: "Other and unspecified osteoarthritis", Category: "Joint", Chapter: "Musculoskeletal", Keywords: []string{"osteoarthritis", "oa", "degenerative joint disease", "djd"}, ICDVersion: "10"},
		{Code: "M81", Description: "Osteoporosis without current pathological fracture", Category: "Bone", Chapter: "Musculoskeletal", Keywords: []string{"osteoporosis", "low bone density"}, ICDVersion: "10"},
	}...)

	// Symptoms and signs (R00-R99)
	c.codes = append(c.codes, []ICDCode{
		{Code: "R07", Description: "Pain in throat and chest", Category: "Pain", Chapter: "Symptoms", Keywords: []string{"chest pain", "chest discomfort", "throat pain"}, ICDVersion: "10"},
		{Code: "R10", Description: "Abdominal and pelvic pain", Category: "Pain", Chapter: "Symptoms", Keywords: []string{"abdominal pain", "stomach pain", "belly pain"}, ICDVersion: "10"},
		{Code: "R51", Description: "Headache", Category: "Pain", Chapter: "Symptoms", Keywords: []string{"headache", "cephalgia", "migraine"}, ICDVersion: "10"},
		{Code: "R05", Description: "Cough", Category: "Respiratory symptom", Chapter: "Symptoms", Keywords: []string{"cough"}, ICDVersion: "10"},
		{Code: "R50", Description: "Fever of other and unknown origin", Category: "General", Chapter: "Symptoms", Keywords: []string{"fever", "pyrexia", "elevated temperature"}, ICDVersion: "10"},
	}...)
}

// MatchCodes finds ICD codes that match a medical case.
func (c *ICDCoder) MatchCodes(caseData *MedicalCase) []ICDCode {
	var matched []ICDCode
	seen := make(map[string]bool)

	// Extract keywords from case
	keywords := c.extractKeywords(caseData)

	// Score each code
	for _, code := range c.codes {
		score := c.calculateMatchScore(code, keywords)
		if score > 0 && !seen[code.Code] {
			matched = append(matched, code)
			seen[code.Code] = true
		}
	}

	return matched
}

// extractKeywords extracts relevant keywords from a medical case.
func (c *ICDCoder) extractKeywords(caseData *MedicalCase) []string {
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

	return keywords
}

// calculateMatchScore calculates how well an ICD code matches the case keywords.
func (c *ICDCoder) calculateMatchScore(code ICDCode, keywords []string) int {
	score := 0

	for _, codeKeyword := range code.Keywords {
		for _, caseKeyword := range keywords {
			if strings.Contains(caseKeyword, codeKeyword) || strings.Contains(codeKeyword, caseKeyword) {
				score++
				break
			}
		}
	}

	return score
}

// LookupCode finds an ICD code by code string.
func (c *ICDCoder) LookupCode(code string) *ICDCode {
	codeUpper := strings.ToUpper(code)
	for i := range c.codes {
		if c.codes[i].Code == codeUpper {
			return &c.codes[i]
		}
	}
	return nil
}
