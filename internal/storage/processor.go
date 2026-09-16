package storage

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kampong/debate/internal/llm"
)

// FileProcessor handles processing different file types for LLM consumption.
type FileProcessor struct {
	storage *FileStorage
}

// NewFileProcessor creates a new file processor.
func NewFileProcessor(storage *FileStorage) *FileProcessor {
	return &FileProcessor{storage: storage}
}

// ProcessedContent represents processed file content ready for LLM.
type ProcessedContent struct {
	Type    string // "text", "image_url"
	Text    string // For text content
	ImageURL string // For image content (base64 data URL)
}

// ProcessFile processes a file and returns content suitable for LLM input.
func (fp *FileProcessor) ProcessFile(file *UploadedFile) ([]ProcessedContent, error) {
	switch file.Type {
	case "image":
		return fp.processImage(file)
	case "text":
		return fp.processText(file)
	case "pdf":
		return fp.processPDF(file)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", file.Type)
	}
}

// processImage converts an image to base64 data URL for vision models.
func (fp *FileProcessor) processImage(file *UploadedFile) ([]ProcessedContent, error) {
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("read image file: %w", err)
	}

	// Determine MIME type
	mimeType := file.MimeType
	if mimeType == "" {
		mimeType = "image/jpeg" // default
	}

	// Create base64 data URL
	base64Data := base64.StdEncoding.EncodeToString(data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	return []ProcessedContent{
		{
			Type:     "image_url",
			ImageURL: dataURL,
		},
	}, nil
}

// processText reads a text file and returns its content.
func (fp *FileProcessor) processText(file *UploadedFile) ([]ProcessedContent, error) {
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("read text file: %w", err)
	}

	content := string(data)
	// Truncate if too long (limit to ~10k chars to avoid token limits)
	if len(content) > 10000 {
		content = content[:10000] + "\n\n[Content truncated...]"
	}

	return []ProcessedContent{
		{
			Type: "text",
			Text: fmt.Sprintf("[File: %s]\n%s", file.Name, content),
		},
	}, nil
}

// processPDF extracts text from a PDF file.
// Note: This is a simplified implementation. For production, use a proper PDF library.
func (fp *FileProcessor) processPDF(file *UploadedFile) ([]ProcessedContent, error) {
	// For now, just read the file as binary and indicate it's a PDF
	// In production, you'd use a library like pdfcpu or unipdf to extract text
	data, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("read PDF file: %w", err)
	}

	// Simple heuristic: check if it's actually a PDF
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		return nil, fmt.Errorf("file is not a valid PDF")
	}

	// For now, return a message indicating PDF processing is limited
	// In production, integrate a proper PDF text extraction library
	return []ProcessedContent{
		{
			Type: "text",
			Text: fmt.Sprintf("[PDF File: %s]\nNote: PDF text extraction is limited. File size: %d bytes. Consider converting to text or images for better processing.", file.Name, len(data)),
		},
	}, nil
}

// BuildMultimodalMessage converts processed content into LLM message format.
// This supports both text and image content for vision-capable models.
func BuildMultimodalMessage(role string, textPrompt string, files []ProcessedContent) llm.Message {
	// If no files or only text files, use simple text format
	hasImages := false
	for _, f := range files {
		if f.Type == "image_url" {
			hasImages = true
			break
		}
	}

	if !hasImages {
		// Simple text-only message
		var content strings.Builder
		content.WriteString(textPrompt)
		for _, f := range files {
			if f.Type == "text" {
				content.WriteString("\n\n")
				content.WriteString(f.Text)
			}
		}
		return llm.Message{
			Role:    role,
			Content: content.String(),
		}
	}

	// For multimodal (text + images), we need to use the OpenAI vision format
	// This requires extending the Message struct to support content arrays
	// For now, we'll create a text description with image references
	// In production, you'd extend the LLM client to support multimodal messages
	
	var content strings.Builder
	content.WriteString(textPrompt)
	
	imageCount := 0
	for _, f := range files {
		if f.Type == "text" {
			content.WriteString("\n\n")
			content.WriteString(f.Text)
		} else if f.Type == "image_url" {
			imageCount++
			content.WriteString(fmt.Sprintf("\n\n[Image %d attached - requires vision-capable model]", imageCount))
		}
	}

	return llm.Message{
		Role:    role,
		Content: content.String(),
	}
}

// ReadFileContent reads and returns the content of a file.
func ReadFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
