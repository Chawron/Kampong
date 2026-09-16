package medical

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kampong/debate/internal/llm"
)

// MedicalImageAnalyzer analyzes medical images using vision-capable LLMs.
type MedicalImageAnalyzer struct {
	client      *llm.Client
	uploadDir   string
}

// MedicalImage represents a medical image with metadata.
type MedicalImage struct {
	ID         string `json:"id"`
	FilePath   string `json:"file_path"`
	Modality   string `json:"modality"` // X-ray, CT, MRI, Ultrasound, etc.
	BodyPart   string `json:"body_part"`
	PatientID  string `json:"patient_id"`
	Base64Data string `json:"base64_data,omitempty"` // For API transmission
}

// ImageAnalysisResult contains the analysis of a medical image.
type ImageAnalysisResult struct {
	ImageID             string   `json:"image_id"`
	Modality            string   `json:"modality"`
	BodyPart            string   `json:"body_part"`
	Findings            []string `json:"findings"`
	Abnormalities       []string `json:"abnormalities"`
	Impression          string   `json:"impression"`
	DifferentialDiagnosis []string `json:"differential_diagnosis"`
	Recommendations     []string `json:"recommendations"`
	Urgency             string   `json:"urgency"`    // routine, urgent, emergency
	Confidence          string   `json:"confidence"` // high, moderate, low
}

// NewMedicalImageAnalyzer creates a new medical image analyzer.
func NewMedicalImageAnalyzer(client *llm.Client, uploadDir string) *MedicalImageAnalyzer {
	return &MedicalImageAnalyzer{
		client:    client,
		uploadDir: uploadDir,
	}
}

// ResolveUploadedFile safely resolves a previously uploaded file ID to its path.
// Only files directly inside uploadDir whose name starts with "<id>_" are returned.
func (m *MedicalImageAnalyzer) ResolveUploadedFile(fileID string) (string, error) {
	// Sanitize: IDs are UUIDs; reject anything suspicious.
	if fileID == "" || len(fileID) > 64 || strings.ContainsAny(fileID, "/\\..") {
		return "", fmt.Errorf("invalid image_id")
	}

	entries, err := os.ReadDir(m.uploadDir)
	if err != nil {
		return "", fmt.Errorf("upload directory unavailable")
	}

	prefix := fileID + "_"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) {
			return filepath.Join(m.uploadDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no uploaded file with id %q", fileID)
}

// AnalyzeImage analyzes a medical image using a vision-capable LLM.
func (m *MedicalImageAnalyzer) AnalyzeImage(ctx context.Context, image *MedicalImage, clinicalContext string) (*ImageAnalysisResult, error) {
	var imageData string
	var err error

	if image.Base64Data != "" {
		// Inline base64 data (already a data URL or raw base64).
		imageData = image.Base64Data
		if !strings.HasPrefix(imageData, "data:") {
			imageData = "data:image/jpeg;base64," + imageData
		}
	} else {
		imageData, err = m.loadImage(image.FilePath)
		if err != nil {
			return nil, fmt.Errorf("load image: %w", err)
		}
	}

	prompt := m.buildAnalysisPrompt(image, clinicalContext)

	messages := []llm.Message{
		{
			Role: "system",
			Content: `You are an expert radiologist analyzing medical imaging. Provide a structured analysis including:
1. Key findings (normal and abnormal)
2. Any abnormalities or concerning features
3. Overall impression
4. Differential diagnosis if abnormalities present
5. Recommendations for follow-up
6. Urgency level (routine, urgent, or emergency)

Be specific, use medical terminology, and consider the clinical context provided.
IMPORTANT: This is for educational purposes only. Always correlate with clinical findings and recommend professional radiologist review.`,
		},
		{
			Role: "user",
			Content: []llm.ContentPart{
				{
					Type: "text",
					Text: prompt,
				},
				{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: imageData,
					},
				},
			},
		},
	}

	response, err := m.client.Complete(ctx, messages, 0.3, 2048)
	if err != nil {
		return nil, fmt.Errorf("analyze image: %w", err)
	}

	result := m.parseAnalysisResponse(response, image)
	return result, nil
}

// loadImage loads an image file and converts it to base64 data URL.
func (m *MedicalImageAnalyzer) loadImage(filePath string) (string, error) {
	// SECURITY: ensure the file resides inside the uploads directory.
	absUpload, err := filepath.Abs(m.uploadDir)
	if err != nil {
		return "", fmt.Errorf("resolve upload dir: %w", err)
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve file path: %w", err)
	}
	if !strings.HasPrefix(absFile, absUpload+string(filepath.Separator)) {
		return "", fmt.Errorf("file path outside upload directory")
	}

	file, err := os.Open(absFile)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	case ".bmp":
		mimeType = "image/bmp"
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
	return dataURL, nil
}

// buildAnalysisPrompt creates a structured prompt tailored to the image modality.
func (m *MedicalImageAnalyzer) buildAnalysisPrompt(image *MedicalImage, clinicalContext string) string {
	var sb strings.Builder

	modality := strings.ToLower(image.Modality)

	sb.WriteString(fmt.Sprintf("Analyze this %s image of the %s.\n\n", image.Modality, image.BodyPart))

	if clinicalContext != "" {
		sb.WriteString("Clinical Context:\n")
		sb.WriteString(clinicalContext)
		sb.WriteString("\n\n")
	}

	// Tailor the analysis instructions based on modality
	switch {
	case strings.Contains(modality, "x-ray") || strings.Contains(modality, "xray"):
		sb.WriteString("Provide a radiologist-style X-ray analysis:\n")
		sb.WriteString("**Findings:**\n- Describe anatomical structures visible\n- Note any fractures, opacities, effusions, or abnormal densities\n\n")
		sb.WriteString("**Abnormalities:**\n- Detail any concerning features\n\n")
		sb.WriteString("**Impression:**\n- Overall radiological impression\n\n")
		sb.WriteString("**Differential Diagnosis:**\n- List possible diagnoses based on findings\n\n")
		sb.WriteString("**Recommendations:**\n- Follow-up imaging or clinical correlation needed\n\n")
		sb.WriteString("**Urgency:** routine, urgent, or emergency\n")

	case strings.Contains(modality, "ct") || strings.Contains(modality, "mri"):
		sb.WriteString("Provide a detailed cross-sectional imaging analysis:\n")
		sb.WriteString("**Findings:**\n- Describe structures in each visible plane/section\n- Note any masses, lesions, hemorrhage, or anatomical variants\n\n")
		sb.WriteString("**Abnormalities:**\n- Size, location, and characteristics of any lesions\n- Enhancement patterns if contrast was used\n\n")
		sb.WriteString("**Impression:**\n- Overall radiological impression\n\n")
		sb.WriteString("**Differential Diagnosis:**\n- Ranked list of possible diagnoses\n\n")
		sb.WriteString("**Recommendations:**\n- Additional imaging, biopsy, or follow-up\n\n")
		sb.WriteString("**Urgency:** routine, urgent, or emergency\n")

	case strings.Contains(modality, "ultrasound"):
		sb.WriteString("Provide an ultrasound analysis:\n")
		sb.WriteString("**Findings:**\n- Describe echogenicity, organ size, and structures visible\n- Note any cysts, stones, fluid collections, or masses\n\n")
		sb.WriteString("**Measurements:**\n- Any measurements visible (organ size, vessel diameter, etc.)\n\n")
		sb.WriteString("**Impression:**\n- Overall impression\n\n")
		sb.WriteString("**Recommendations:**\n- Follow-up or additional studies\n\n")
		sb.WriteString("**Urgency:** routine, urgent, or emergency\n")

	case strings.Contains(modality, "ecg") || strings.Contains(modality, "ekg"):
		sb.WriteString("Provide a systematic ECG interpretation:\n")
		sb.WriteString("**Rate:** Heart rate in bpm\n\n")
		sb.WriteString("**Rhythm:** Regular/irregular, sinus/non-sinus\n\n")
		sb.WriteString("**Intervals:** PR, QRS, QTc if measurable\n\n")
		sb.WriteString("**Morphology:** P wave, ST segment, T wave changes\n\n")
		sb.WriteString("**Impression:** Overall rhythm diagnosis and any abnormalities\n\n")
		sb.WriteString("**Urgency:** routine, urgent, or emergency\n")

	case strings.Contains(modality, "chart") || strings.Contains(modality, "graph"):
		sb.WriteString("Read and analyze this chart/graph:\n")
		sb.WriteString("**Data Shown:** What variables/axes are displayed\n\n")
		sb.WriteString("**Key Values:** Notable data points, peaks, valleys\n\n")
		sb.WriteString("**Trends:** Direction and significance of any trends\n\n")
		sb.WriteString("**Clinical Relevance:** What this data suggests medically\n\n")

	case strings.Contains(modality, "lab"):
		sb.WriteString("Extract and analyze all lab results:\n")
		sb.WriteString("**Test Results:** List each test with value, unit, reference range, and flag\n\n")
		sb.WriteString("**Abnormal Values:** Highlight high/low/critical results\n\n")
		sb.WriteString("**Pattern Analysis:** Are there related abnormal results suggesting a syndrome?\n\n")
		sb.WriteString("**Clinical Significance:** What these results suggest\n\n")
		sb.WriteString("**Recommendations:** Follow-up tests or actions needed\n")

	default:
		sb.WriteString("Provide your analysis in the following structured format:\n\n")
		sb.WriteString("**Findings:**\n- List all key findings (both normal and abnormal)\n\n")
		sb.WriteString("**Abnormalities:**\n- Describe any abnormalities or concerning features\n\n")
		sb.WriteString("**Impression:**\n- Overall impression of the study\n\n")
		sb.WriteString("**Differential Diagnosis:**\n- If abnormalities present, list differential diagnoses\n\n")
		sb.WriteString("**Recommendations:**\n- Suggested follow-up or additional studies\n\n")
		sb.WriteString("**Urgency:**\n- Rate as: routine, urgent, or emergency\n")
	}

	return sb.String()
}

// parseAnalysisResponse parses the LLM response into a structured result.
func (m *MedicalImageAnalyzer) parseAnalysisResponse(response string, image *MedicalImage) *ImageAnalysisResult {
	result := &ImageAnalysisResult{
		ImageID:  image.ID,
		Modality: image.Modality,
		BodyPart: image.BodyPart,
	}

	lines := strings.Split(response, "\n")
	currentSection := ""
	var impressionLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		clean := strings.TrimLeft(trimmed, "#*")
		clean = strings.TrimSpace(clean)
		lower := strings.ToLower(clean)

		isListItem := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "• ") ||
			strings.HasPrefix(trimmed, "* ") || isNumberedItem(trimmed)

		if !isListItem {
			if matchesSection(lower, "finding") {
				currentSection = "findings"
				if after := extractAfterHeader(clean); after != "" {
					result.Findings = append(result.Findings, after)
				}
				continue
			} else if matchesSection(lower, "abnormal") {
				currentSection = "abnormalities"
				if after := extractAfterHeader(clean); after != "" {
					result.Abnormalities = append(result.Abnormalities, after)
				}
				continue
			} else if matchesSection(lower, "impression", "conclusion", "summary") {
				currentSection = "impression"
				if after := extractAfterHeader(clean); after != "" {
					impressionLines = append(impressionLines, after)
				}
				continue
			} else if matchesSection(lower, "differential") {
				currentSection = "differential"
				if after := extractAfterHeader(clean); after != "" {
					result.DifferentialDiagnosis = append(result.DifferentialDiagnosis, after)
				}
				continue
			} else if matchesSection(lower, "recommendation", "suggestion", "follow-up", "follow up") {
				currentSection = "recommendations"
				if after := extractAfterHeader(clean); after != "" {
					result.Recommendations = append(result.Recommendations, after)
				}
				continue
			} else if matchesSection(lower, "urgency", "priority") {
				currentSection = "urgency"
				if after := extractAfterHeader(clean); after != "" {
					parseUrgency(after, result)
				}
				continue
			} else if matchesSection(lower, "confidence", "certainty") {
				currentSection = "confidence"
				if after := extractAfterHeader(clean); after != "" {
					parseConfidence(after, result)
				}
				continue
			}
		}

		itemText := extractListItem(trimmed)

		switch currentSection {
		case "findings":
			if itemText != "" {
				result.Findings = append(result.Findings, itemText)
			} else if !isListItem && trimmed != "" {
				result.Findings = append(result.Findings, trimmed)
			}
		case "abnormalities":
			if itemText != "" {
				result.Abnormalities = append(result.Abnormalities, itemText)
			} else if !isListItem && trimmed != "" {
				result.Abnormalities = append(result.Abnormalities, trimmed)
			}
		case "differential":
			if itemText != "" {
				result.DifferentialDiagnosis = append(result.DifferentialDiagnosis, itemText)
			} else if !isListItem && trimmed != "" {
				result.DifferentialDiagnosis = append(result.DifferentialDiagnosis, trimmed)
			}
		case "recommendations":
			if itemText != "" {
				result.Recommendations = append(result.Recommendations, itemText)
			} else if !isListItem && trimmed != "" {
				result.Recommendations = append(result.Recommendations, trimmed)
			}
		case "impression":
			if itemText != "" {
				impressionLines = append(impressionLines, itemText)
			} else if !isListItem && trimmed != "" {
				impressionLines = append(impressionLines, trimmed)
			}
		case "urgency":
			if itemText != "" {
				parseUrgency(itemText, result)
			} else {
				parseUrgency(trimmed, result)
			}
		case "confidence":
			if itemText != "" {
				parseConfidence(itemText, result)
			} else {
				parseConfidence(trimmed, result)
			}
		}
	}

	if len(impressionLines) > 0 {
		result.Impression = strings.Join(impressionLines, " ")
	}

	if result.Urgency == "" {
		if len(result.Abnormalities) > 0 {
			result.Urgency = "urgent"
		} else {
			result.Urgency = "routine"
		}
	}

	if result.Confidence == "" {
		if len(result.Findings) > 3 {
			result.Confidence = "high"
		} else if len(result.Findings) > 0 {
			result.Confidence = "moderate"
		} else {
			result.Confidence = "low"
		}
	}

	return result
}

func matchesSection(lower string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func extractAfterHeader(clean string) string {
	idx := strings.Index(clean, ":")
	if idx < 0 || idx >= len(clean)-1 {
		return ""
	}
	after := strings.TrimSpace(clean[idx+1:])
	after = strings.TrimPrefix(after, "**")
	after = strings.TrimPrefix(after, "*")
	return strings.TrimSpace(after)
}

func extractListItem(line string) string {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "• ") || strings.HasPrefix(line, "* ") {
		return strings.TrimSpace(line[2:])
	}
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if (ch == '.' || ch == ')') && i+1 < len(line) && line[i+1] == ' ' {
			return strings.TrimSpace(line[i+2:])
		}
		break
	}
	return ""
}

func isNumberedItem(line string) bool {
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if (ch == '.' || ch == ')') && i+1 < len(line) && line[i+1] == ' ' {
			return true
		}
		return false
	}
	return false
}

func parseUrgency(text string, result *ImageAnalysisResult) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "emergency") || strings.Contains(lower, "critical") || strings.Contains(lower, "stat") {
		result.Urgency = "emergency"
	} else if strings.Contains(lower, "urgent") || strings.Contains(lower, "priority") || strings.Contains(lower, "soon") {
		result.Urgency = "urgent"
	} else if strings.Contains(lower, "routine") || strings.Contains(lower, "non-urgent") || strings.Contains(lower, "normal") {
		result.Urgency = "routine"
	}
}

func parseConfidence(text string, result *ImageAnalysisResult) {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "high") || strings.Contains(lower, "definite") || strings.Contains(lower, "clear") {
		result.Confidence = "high"
	} else if strings.Contains(lower, "moderate") || strings.Contains(lower, "probable") || strings.Contains(lower, "likely") {
		result.Confidence = "moderate"
	} else if strings.Contains(lower, "low") || strings.Contains(lower, "possible") || strings.Contains(lower, "uncertain") || strings.Contains(lower, "limited") {
		result.Confidence = "low"
	}
}
