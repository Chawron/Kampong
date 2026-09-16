package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

// FileStorage handles file uploads and storage.
type FileStorage struct {
	uploadDir string
}

// NewFileStorage creates a new file storage instance.
func NewFileStorage(uploadDir string) (*FileStorage, error) {
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}

	return &FileStorage{
		uploadDir: uploadDir,
	}, nil
}

// UploadedFile represents a stored file.
type UploadedFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	Type     string `json:"type"` // "image", "text", "pdf", "other"
}

// StoreFile saves an uploaded file and returns metadata.
func (fs *FileStorage) StoreFile(file *multipart.FileHeader) (*UploadedFile, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded file: %w", err)
	}
	defer src.Close()

	// Generate unique ID for the file
	id := generateID()
	
	// Determine file type and extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	mimeType := file.Header.Get("Content-Type")
	fileType := determineFileType(ext, mimeType)

	// Create destination path
	destPath := filepath.Join(fs.uploadDir, id+ext)

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the file
	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}

	return &UploadedFile{
		ID:       id,
		Name:     file.Filename,
		Path:     destPath,
		Size:     file.Size,
		MimeType: mimeType,
		Type:     fileType,
	}, nil
}

// GetFilePath returns the full path for a file ID.
func (fs *FileStorage) GetFilePath(id string, ext string) string {
	return filepath.Join(fs.uploadDir, id+ext)
}

// DeleteFile removes a file from storage.
func (fs *FileStorage) DeleteFile(id string, ext string) error {
	path := fs.GetFilePath(id, ext)
	return os.Remove(path)
}

// generateID creates a random 16-character hex ID.
func generateID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(bytes)
}

// determineFileType categorizes the file based on extension and MIME type.
func determineFileType(ext, mimeType string) string {
	// Image types
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	if imageExts[ext] || strings.HasPrefix(mimeType, "image/") {
		return "image"
	}

	// PDF type
	if ext == ".pdf" || mimeType == "application/pdf" {
		return "pdf"
	}

	// Text types
	textExts := map[string]bool{".txt": true, ".md": true, ".text": true, ".csv": true}
	if textExts[ext] || strings.HasPrefix(mimeType, "text/") {
		return "text"
	}

	return "other"
}
