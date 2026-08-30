package domain_test

import (
	"testing"

	"github.com/priyanshuguptadev/job-board/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestValidateCandidateInput(t *testing.T) {
	t.Run("valid input passes", func(t *testing.T) {
		phone := "+1234567890"
		url := "https://linkedin.com/in/test"
		details := domain.ValidateCandidateInput("John Doe", "john@example.com", &phone, &url)
		assert.Empty(t, details)
	})

	t.Run("missing required name and email", func(t *testing.T) {
		details := domain.ValidateCandidateInput("", "", nil, nil)
		assert.Len(t, details, 2)
		assert.Equal(t, "candidate_name", details[0].Field)
		assert.Equal(t, "candidate_email", details[1].Field)
	})

	t.Run("invalid email format", func(t *testing.T) {
		details := domain.ValidateCandidateInput("John Doe", "not-an-email", nil, nil)
		assert.Len(t, details, 1)
		assert.Equal(t, "candidate_email", details[0].Field)
	})

	t.Run("invalid linkedin url", func(t *testing.T) {
		badURL := "not a url"
		details := domain.ValidateCandidateInput("John Doe", "john@example.com", nil, &badURL)
		assert.Len(t, details, 1)
		assert.Equal(t, "linkedin_url", details[0].Field)
	})
}

func TestValidateCustomAnswers(t *testing.T) {
	fields := []domain.CustomField{
		{
			ID:       "years_exp",
			Label:    "Years of experience",
			Type:     "number",
			Required: true,
		},
		{
			ID:       "website",
			Label:    "Portfolio URL",
			Type:     "url",
			Required: false,
		},
		{
			ID:       "role",
			Label:    "Primary Role",
			Type:     "select",
			Options:  []string{"Frontend", "Backend", "Fullstack"},
			Required: true,
		},
		{
			ID:       "remote",
			Label:    "Open to Remote",
			Type:     "boolean",
			Required: false,
		},
	}

	t.Run("valid custom answers pass", func(t *testing.T) {
		answers := map[string]interface{}{
			"years_exp": 5,
			"website":   "https://example.com",
			"role":      "Backend",
			"remote":    true,
		}
		details := domain.ValidateCustomAnswers(fields, answers)
		assert.Empty(t, details)
	})

	t.Run("missing required custom fields", func(t *testing.T) {
		answers := map[string]interface{}{}
		details := domain.ValidateCustomAnswers(fields, answers)
		assert.Len(t, details, 2)
		assert.Equal(t, "custom_answers.years_exp", details[0].Field)
		assert.Equal(t, "custom_answers.role", details[1].Field)
	})

	t.Run("invalid types for custom fields", func(t *testing.T) {
		answers := map[string]interface{}{
			"years_exp": "five",
			"website":   "httpx://bad url",
			"role":      "DevOps", // not in options
			"remote":    "maybe",  // not a boolean
		}
		details := domain.ValidateCustomAnswers(fields, answers)
		assert.Len(t, details, 4)
	})
}
