package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxResumeSize is the maximum allowed size for resume attachments (10MB).
	MaxResumeSize int64 = 10 * 1024 * 1024

	// Canonical MIME types
	ContentTypePDF  = "application/pdf"
	ContentTypeDOC  = "application/msword"
	ContentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
)

// Validation errors.
var (
	ErrFileEmpty            = errors.New("resume file cannot be empty")
	ErrFileTooLarge         = errors.New("file exceeds the maximum allowed size of 10MB")
	ErrUnsupportedMediaType = errors.New("resume upload is not an allowed MIME type (PDF, DOC, DOCX)")
	ErrInvalidFilename      = errors.New("invalid resume file name")
)

// Allowed MIME types mapped to their canonical forms.
var allowedMIMETypes = map[string]string{
	"application/pdf":         ContentTypePDF,
	"application/x-pdf":       ContentTypePDF,
	"application/msword":      ContentTypeDOC,
	"application/x-msword":    ContentTypeDOC,
	"application/vnd.ms-word": ContentTypeDOC,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ContentTypeDOCX,
	"application/zip":              ContentTypeDOCX,
	"application/x-zip-compressed": ContentTypeDOCX,
}

// Magic byte signatures.
var (
	magicPDF = []byte("%PDF")
	magicDOC = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	magicZIP = []byte{0x50, 0x4B, 0x03, 0x04} // DOCX is a zipped XML package
)

var unsafeCharRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// ValidationResult holds the result of validating a resume attachment.
type ValidationResult struct {
	SanitizedFilename string
	ContentType       string
	Size              int64
	Reader            io.Reader
}

// IsAllowedExtension returns true if the file extension is one of the supported resume formats (.pdf, .doc, .docx).
func IsAllowedExtension(ext string) bool {
	normalized := strings.ToLower(strings.TrimPrefix(ext, "."))
	switch normalized {
	case "pdf", "doc", "docx":
		return true
	default:
		return false
	}
}

// IsAllowedMimeType checks if a given MIME content type is permitted.
func IsAllowedMimeType(mimeType string) bool {
	cleanType := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	_, ok := allowedMIMETypes[cleanType]
	return ok
}

// SanitizeFilename cleans an uploaded filename, removing directory traversal components,
// control characters, and unsafe characters, while preserving the extension.
func SanitizeFilename(filename string) string {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "resume"
	}

	// Extract base filename (removes directory paths/traversals)
	base := filepath.Base(trimmed)
	base = strings.ReplaceAll(base, "\\", "/")
	if idx := strings.LastIndex(base, "/"); idx != -1 {
		base = base[idx+1:]
	}

	if base == "." || base == "/" || base == "" {
		return "resume"
	}

	ext := strings.ToLower(filepath.Ext(base))
	if ext == "." {
		ext = ""
	}
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))

	// Replace unsafe characters with underscores
	sanitized := unsafeCharRegex.ReplaceAllString(nameWithoutExt, "_")

	// Collapse consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}
	sanitized = strings.Trim(sanitized, "._- ")

	if sanitized == "" {
		sanitized = "resume"
	}

	// Limit base name length to prevent filesystem or database issues
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	return sanitized + ext
}

// GenerateResumeKey constructs the S3 storage object key for a candidate resume.
// Format: resumes/{job_id}/{application_id}/{sanitized_filename}
func GenerateResumeKey(jobID, appID, filename string) string {
	sanitized := SanitizeFilename(filename)
	cleanJobID := strings.TrimSpace(jobID)
	cleanAppID := strings.TrimSpace(appID)
	return fmt.Sprintf("resumes/%s/%s/%s", cleanJobID, cleanAppID, sanitized)
}

// ValidateResumeHeader validates filename, size, and declared content type headers.
func ValidateResumeHeader(filename string, size int64, headerContentType string) error {
	if size == 0 {
		return ErrFileEmpty
	}
	if size > MaxResumeSize {
		return ErrFileTooLarge
	}

	sanitized := SanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(sanitized))
	if !IsAllowedExtension(ext) {
		return ErrUnsupportedMediaType
	}

	if headerContentType != "" {
		cleanType := strings.ToLower(strings.TrimSpace(strings.Split(headerContentType, ";")[0]))
		// If content-type is octet-stream or form-data, we will rely on content sniff
		if cleanType != "application/octet-stream" && cleanType != "multipart/form-data" {
			if !IsAllowedMimeType(cleanType) {
				return ErrUnsupportedMediaType
			}
		}
	}

	return nil
}

// ValidateResume performs complete validation on the resume file:
// 1. Filename sanitization and extension check (.pdf, .doc, .docx).
// 2. File size bounds check (1 byte to 10MB).
// 3. Magic bytes sniffing to ensure the content matches the declared extension.
// It returns a ValidationResult with the sanitized name, detected canonical MIME type, size, and
// a new Reader containing the full content (including peeked bytes).
func ValidateResume(filename string, size int64, r io.Reader) (*ValidationResult, error) {
	if r == nil {
		return nil, ErrFileEmpty
	}

	if size == 0 {
		return nil, ErrFileEmpty
	}
	if size > MaxResumeSize {
		return nil, ErrFileTooLarge
	}

	sanitized := SanitizeFilename(filename)
	ext := strings.ToLower(filepath.Ext(sanitized))
	if !IsAllowedExtension(ext) {
		return nil, ErrUnsupportedMediaType
	}

	// Peek at the first 512 bytes for magic bytes sniffing
	headerBuf := make([]byte, 512)
	n, err := io.ReadFull(r, headerBuf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	if n == 0 {
		return nil, ErrFileEmpty
	}

	headerBytes := headerBuf[:n]

	// Verify magic bytes for corresponding file format
	var canonicalType string
	switch ext {
	case ".pdf":
		if !bytes.HasPrefix(headerBytes, magicPDF) {
			return nil, ErrUnsupportedMediaType
		}
		canonicalType = ContentTypePDF
	case ".doc":
		if !bytes.HasPrefix(headerBytes, magicDOC) {
			return nil, ErrUnsupportedMediaType
		}
		canonicalType = ContentTypeDOC
	case ".docx":
		if !bytes.HasPrefix(headerBytes, magicZIP) {
			return nil, ErrUnsupportedMediaType
		}
		canonicalType = ContentTypeDOCX
	default:
		return nil, ErrUnsupportedMediaType
	}

	// Reconstruct the reader with the peeked bytes prepended
	fullReader := io.MultiReader(bytes.NewReader(headerBytes), r)

	return &ValidationResult{
		SanitizedFilename: sanitized,
		ContentType:       canonicalType,
		Size:              size,
		Reader:            fullReader,
	}, nil
}
