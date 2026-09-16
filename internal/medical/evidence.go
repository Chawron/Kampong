package medical

// EvidenceScorer evaluates the quality of medical evidence.
type EvidenceScorer struct {
	levels map[int]EvidenceLevelDefinition
}

// EvidenceLevelDefinition defines an Oxford evidence level.
type EvidenceLevelDefinition struct {
	Level       int
	Name        string
	Description string
	StudyTypes  []string
	Weight      float64 // 0-1, how much to trust this level
}

// NewEvidenceScorer creates a new evidence scorer with Oxford levels.
func NewEvidenceScorer() *EvidenceScorer {
	return &EvidenceScorer{
		levels: map[int]EvidenceLevelDefinition{
			1: {
				Level:       1,
				Name:        "High Quality RCT",
				Description: "Systematic review of randomized controlled trials",
				StudyTypes:  []string{"systematic review", "meta-analysis", "RCT"},
				Weight:      1.0,
			},
			2: {
				Level:       2,
				Name:        "Individual RCT",
				Description: "Single randomized controlled trial",
				StudyTypes:  []string{"randomized controlled trial", "clinical trial"},
				Weight:      0.9,
			},
			3: {
				Level:       3,
				Name:        "Cohort Study",
				Description: "Cohort or case-control studies",
				StudyTypes:  []string{"cohort study", "case-control study", "longitudinal study"},
				Weight:      0.7,
			},
			4: {
				Level:       4,
				Name:        "Case Series",
				Description: "Case series, case reports",
				StudyTypes:  []string{"case series", "case report"},
				Weight:      0.5,
			},
			5: {
				Level:       5,
				Name:        "Expert Opinion",
				Description: "Expert opinion, editorial, letter",
				StudyTypes:  []string{"expert opinion", "editorial", "guideline", "consensus"},
				Weight:      0.3,
			},
		},
	}
}

// ScoreEvidence evaluates evidence based on study characteristics.
func (s *EvidenceScorer) ScoreEvidence(studyType string, sampleSize int, hasControlGroup bool, isPeerReviewed bool, hasConflictOfInterest bool) EvidenceLevel {
	// Determine base level from study type
	level := s.determineLevel(studyType)
	def := s.levels[level]

	// Adjust confidence based on additional factors
	confidence := def.Weight

	// Sample size adjustments
	if sampleSize > 1000 {
		confidence *= 1.1
	} else if sampleSize > 100 {
		confidence *= 1.05
	} else if sampleSize < 30 {
		confidence *= 0.8
	}

	// Control group bonus
	if hasControlGroup {
		confidence *= 1.1
	}

	// Peer review bonus
	if isPeerReviewed {
		confidence *= 1.05
	}

	// Conflict of interest penalty
	if hasConflictOfInterest {
		confidence *= 0.7
	}

	// Cap confidence at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Determine bias risk
	biasRisk := "low"
	if hasConflictOfInterest {
		biasRisk = "high"
	} else if sampleSize < 50 {
		biasRisk = "moderate"
	}

	return EvidenceLevel{
		Level:       level,
		Description: def.Description,
		StudyType:   studyType,
		SampleSize:  sampleSize,
		Confidence:  confidence,
		BiasRisk:    biasRisk,
	}
}

// determineLevel maps study type to evidence level.
func (s *EvidenceScorer) determineLevel(studyType string) int {
	studyType = toLower(studyType)

	// Level 1: Systematic reviews and meta-analyses
	if containsAny(studyType, []string{"systematic review", "meta-analysis"}) {
		return 1
	}

	// Level 2: RCTs
	if containsAny(studyType, []string{"randomized", "rct", "clinical trial"}) {
		return 2
	}

	// Level 3: Cohort and case-control studies
	if containsAny(studyType, []string{"cohort", "case-control", "longitudinal"}) {
		return 3
	}

	// Level 4: Case series and reports
	if containsAny(studyType, []string{"case series", "case report"}) {
		return 4
	}

	// Level 5: Expert opinion and guidelines
	if containsAny(studyType, []string{"expert opinion", "editorial", "guideline", "consensus"}) {
		return 5
	}

	// Default to level 5 if unknown
	return 5
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if containsIgnoreCase(s, substr) {
			return true
		}
	}
	return false
}

// FormatEvidenceLevel returns a human-readable evidence level string.
func FormatEvidenceLevel(level EvidenceLevel) string {
	return format("Level %d: %s (Confidence: %.0f%%, Bias Risk: %s)",
		level.Level, level.Description, level.Confidence*100, level.BiasRisk)
}

// format is a simple string formatter.
func format(template string, args ...interface{}) string {
	// Simple implementation - in production would use fmt.Sprintf
	result := template
	for i, arg := range args {
		placeholder := format("%%%d", i)
		result = replaceAll(result, placeholder, formatArg(arg))
	}
	return result
}

func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case int:
		return formatInt(v)
	case float64:
		return formatFloat(v)
	case string:
		return v
	default:
		return ""
	}
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func formatFloat(f float64) string {
	// Simple float formatting to 0 decimal places
	intPart := int(f)
	return formatInt(intPart)
}

func replaceAll(s, old, new string) string {
	result := ""
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}
