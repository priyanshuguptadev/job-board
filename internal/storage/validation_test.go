package storage

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAllowedExtension(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".pdf", true},
		{"pdf", true},
		{".PDF", true},
		{".doc", true},
		{"DOC", true},
		{".docx", true},
		{"docx", true},
		{".DOCX", true},
		{".exe", false},
		{".txt", false},
		{".jpg", false},
		{".png", false},
		{".zip", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("ext_"+tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAllowedExtension(tt.ext))
		})
	}
}

func TestIsAllowedMimeType(t *testing.T) {
	tests := []struct {
		mime     string
		expected bool
	}{
		{"application/pdf", true},
		{"application/x-pdf", true},
		{"application/pdf; charset=utf-8", true},
		{"application/msword", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/zip", true},
		{"image/png", false},
		{"application/javascript", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("mime_"+tt.mime, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAllowedMimeType(tt.mime))
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"resume.pdf", "resume.pdf"},
		{"My Resume (2024).pdf", "My_Resume_2024.pdf"},
		{"../../etc/passwd.pdf", "passwd.pdf"},
		{"..\\..\\windows\\system32\\calc.pdf", "calc.pdf"},
		{"john@doe#resume$.docx", "john_doe_resume.docx"},
		{"__test___file__.doc", "test_file.doc"},
		{"   spaced_name.pdf   ", "spaced_name.pdf"},
		{strings.Repeat("a", 150) + ".pdf", strings.Repeat("a", 100) + ".pdf"},
		{"", "resume"},
		{".pdf", "resume.pdf"},
		{"$$$$.pdf", "resume.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeFilename(tt.input))
		})
	}
}

func TestGenerateResumeKey(t *testing.T) {
	key := GenerateResumeKey("job-123", "app-456", "John Doe's Resume.pdf")
	assert.Equal(t, "resumes/job-123/app-456/John_Doe_s_Resume.pdf", key)
}

func TestValidateResumeHeader(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		size        int64
		contentType string
		wantErr     error
	}{
		{
			name:        "valid pdf header",
			filename:    "resume.pdf",
			size:        1024,
			contentType: "application/pdf",
			wantErr:     nil,
		},
		{
			name:        "valid docx header",
			filename:    "resume.docx",
			size:        2048,
			contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			wantErr:     nil,
		},
		{
			name:        "valid doc header",
			filename:    "resume.doc",
			size:        512,
			contentType: "application/msword",
			wantErr:     nil,
		},
		{
			name:        "empty file",
			filename:    "resume.pdf",
			size:        0,
			contentType: "application/pdf",
			wantErr:     ErrFileEmpty,
		},
		{
			name:        "oversized file",
			filename:    "resume.pdf",
			size:        MaxResumeSize + 1,
			contentType: "application/pdf",
			wantErr:     ErrFileTooLarge,
		},
		{
			name:        "unsupported extension",
			filename:    "resume.exe",
			size:        1024,
			contentType: "application/pdf",
			wantErr:     ErrUnsupportedMediaType,
		},
		{
			name:        "unsupported content type",
			filename:    "resume.pdf",
			size:        1024,
			contentType: "image/jpeg",
			wantErr:     ErrUnsupportedMediaType,
		},
		{
			name:        "generic binary octet-stream allowed in header pre-check",
			filename:    "resume.pdf",
			size:        1024,
			contentType: "application/octet-stream",
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResumeHeader(tt.filename, tt.size, tt.contentType)
			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateResume(t *testing.T) {
	pdfContent := append([]byte("%PDF-1.7\n"), []byte("sample pdf body content here")...)
	docContent := append([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}, []byte("legacy word doc stream")...)
	docxContent := append([]byte{0x50, 0x4B, 0x03, 0x04}, []byte("word/document.xml inside zip package")...)

	tests := []struct {
		name            string
		filename        string
		content         []byte
		size            int64
		reader          io.Reader
		wantErr         error
		wantContentType string
		wantFilename    string
	}{
		{
			name:            "valid pdf upload",
			filename:        "My Resume.pdf",
			content:         pdfContent,
			size:            int64(len(pdfContent)),
			wantErr:         nil,
			wantContentType: ContentTypePDF,
			wantFilename:    "My_Resume.pdf",
		},
		{
			name:            "valid doc upload",
			filename:        "resume.doc",
			content:         docContent,
			size:            int64(len(docContent)),
			wantErr:         nil,
			wantContentType: ContentTypeDOC,
			wantFilename:    "resume.doc",
		},
		{
			name:            "valid docx upload",
			filename:        "CV_2024.docx",
			content:         docxContent,
			size:            int64(len(docxContent)),
			wantErr:         nil,
			wantContentType: ContentTypeDOCX,
			wantFilename:    "CV_2024.docx",
		},
		{
			name:     "nil reader",
			filename: "resume.pdf",
			size:     100,
			reader:   nil,
			wantErr:  ErrFileEmpty,
		},
		{
			name:     "zero size",
			filename: "resume.pdf",
			content:  []byte{},
			size:     0,
			wantErr:  ErrFileEmpty,
		},
		{
			name:     "file exceeds 10MB limit",
			filename: "resume.pdf",
			content:  pdfContent,
			size:     MaxResumeSize + 1,
			wantErr:  ErrFileTooLarge,
		},
		{
			name:     "disallowed extension",
			filename: "malware.exe",
			content:  []byte("MZ\x90\x00"),
			size:     4,
			wantErr:  ErrUnsupportedMediaType,
		},
		{
			name:     "fake pdf with text content",
			filename: "fake.pdf",
			content:  []byte("This is just plain text masquerading as a PDF"),
			size:     46,
			wantErr:  ErrUnsupportedMediaType,
		},
		{
			name:     "fake doc with elf header",
			filename: "fake.doc",
			content:  []byte("\x7fELFfakebinary"),
			size:     14,
			wantErr:  ErrUnsupportedMediaType,
		},
		{
			name:     "fake docx with png content",
			filename: "fake.docx",
			content:  []byte("\x89PNG\r\n\x1a\n"),
			size:     8,
			wantErr:  ErrUnsupportedMediaType,
		},
		{
			name:     "empty reader at EOF",
			filename: "empty.pdf",
			content:  []byte{},
			size:     10, // declared 10 but stream is empty
			wantErr:  ErrFileEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r io.Reader = tt.reader
			if r == nil && tt.content != nil {
				r = bytes.NewReader(tt.content)
			}

			result, err := ValidateResume(tt.filename, tt.size, r)
			if tt.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantContentType, result.ContentType)
				assert.Equal(t, tt.wantFilename, result.SanitizedFilename)
				assert.Equal(t, tt.size, result.Size)

				// Verify that reading from result.Reader returns the complete original content
				readAll, err := io.ReadAll(result.Reader)
				require.NoError(t, err)
				assert.Equal(t, tt.content, readAll)
			}
		})
	}
}
