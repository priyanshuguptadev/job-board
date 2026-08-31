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

func TestGenerateSlugAndIsValidSlug(t *testing.T) {
	t.Run("generates valid slug from title", func(t *testing.T) {
		assert.Equal(t, "senior-go-engineer", domain.GenerateSlug("Senior Go Engineer"))
		assert.Equal(t, "staff-software-engineer-infra", domain.GenerateSlug("Staff Software Engineer (Infra)!"))
		assert.Equal(t, "product-manager", domain.GenerateSlug("Product   Manager"))
		assert.Equal(t, "job", domain.GenerateSlug("---!@#$$---"))
	})

	t.Run("validates slug format", func(t *testing.T) {
		assert.True(t, domain.IsValidSlug("senior-go-engineer"))
		assert.True(t, domain.IsValidSlug("eng-123"))
		assert.False(t, domain.IsValidSlug("Senior-Go-Engineer"))
		assert.False(t, domain.IsValidSlug("-invalid-"))
		assert.False(t, domain.IsValidSlug("invalid--slug"))
		assert.False(t, domain.IsValidSlug(""))
	})
}

func TestValidateJobInput(t *testing.T) {
	t.Run("valid job input passes", func(t *testing.T) {
		min := 100000.0
		max := 150000.0
		exp := "Senior"
		details := domain.ValidateJobInput(
			"Lead Engineer",
			"Engineering",
			"Remote",
			"Job description markdown",
			domain.EmploymentTypeFullTime,
			domain.JobStatusDraft,
			&exp,
			&min,
			&max,
			"USD",
			"lead-engineer",
			nil,
		)
		assert.Empty(t, details)
	})

	t.Run("missing required fields", func(t *testing.T) {
		details := domain.ValidateJobInput("", "", "", "", "", "", nil, nil, nil, "", "", nil)
		assert.Len(t, details, 4)
	})

	t.Run("salary_min greater than salary_max", func(t *testing.T) {
		min := 150000.0
		max := 100000.0
		details := domain.ValidateJobInput(
			"Lead Engineer",
			"Engineering",
			"Remote",
			"Job description markdown",
			domain.EmploymentTypeFullTime,
			domain.JobStatusDraft,
			nil,
			&min,
			&max,
			"USD",
			"",
			nil,
		)
		assert.Len(t, details, 1)
		assert.Equal(t, "salary_min", details[0].Field)
	})

	t.Run("invalid employment type and status", func(t *testing.T) {
		details := domain.ValidateJobInput(
			"Lead Engineer",
			"Engineering",
			"Remote",
			"Job description markdown",
			domain.EmploymentType("freelance"),
			domain.JobStatus("cancelled"),
			nil,
			nil,
			nil,
			"USD",
			"",
			nil,
		)
		assert.Len(t, details, 2)
	})
}

func TestValidateCustomFields(t *testing.T) {
	t.Run("valid custom fields pass", func(t *testing.T) {
		fields := []domain.CustomField{
			{
				ID:       "years_exp",
				Label:    "Years of experience",
				Type:     "number",
				Required: true,
			},
			{
				ID:       "team_pref",
				Label:    "Team Preference",
				Type:     "select",
				Options:  []string{"Core", "Platform"},
				Required: true,
			},
		}
		details := domain.ValidateCustomFields(fields)
		assert.Empty(t, details)
	})

	t.Run("invalid custom fields fail", func(t *testing.T) {
		fields := []domain.CustomField{
			{
				ID:       "",
				Label:    "",
				Type:     "unknown",
				Required: true,
			},
			{
				ID:       "duplicate",
				Label:    "Select field without options",
				Type:     "select",
				Options:  []string{},
				Required: true,
			},
			{
				ID:       "duplicate",
				Label:    "Dup ID",
				Type:     "text",
				Required: false,
			},
		}
		details := domain.ValidateCustomFields(fields)
		assert.NotEmpty(t, details)
	})
}

func TestValidateStageUpdate(t *testing.T) {
	t.Run("valid stage update", func(t *testing.T) {
		reason := "Not enough experience"
		details := domain.ValidateStageUpdate(domain.ApplicationStageRejected, &reason)
		assert.Empty(t, details)
	})

	t.Run("invalid stage", func(t *testing.T) {
		details := domain.ValidateStageUpdate(domain.ApplicationStage("pending"), nil)
		assert.Len(t, details, 1)
		assert.Equal(t, "stage", details[0].Field)
	})
}

func TestValidateNoteInput(t *testing.T) {
	t.Run("valid note input", func(t *testing.T) {
		details := domain.ValidateNoteInput("Recruiter", "Strong candidate")
		assert.Empty(t, details)
	})

	t.Run("empty author and text", func(t *testing.T) {
		details := domain.ValidateNoteInput("", "")
		assert.Len(t, details, 2)
	})
}

func TestValidateWebhookSubscriptionInput(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		token := "whsec_12345"
		details := domain.ValidateWebhookSubscriptionInput(
			"https://example.com/webhook",
			[]string{domain.EventJobPublished, domain.EventApplicationCreated},
			&token,
		)
		assert.Empty(t, details)
	})

	t.Run("valid input with wildcard event", func(t *testing.T) {
		details := domain.ValidateWebhookSubscriptionInput(
			"https://example.com/webhook",
			[]string{"*"},
			nil,
		)
		assert.Empty(t, details)
	})

	t.Run("missing target_url and empty events", func(t *testing.T) {
		details := domain.ValidateWebhookSubscriptionInput("", []string{}, nil)
		assert.Len(t, details, 2)
	})

	t.Run("invalid target_url and invalid event names", func(t *testing.T) {
		details := domain.ValidateWebhookSubscriptionInput("invalid-url", []string{"invalid.event", ""}, nil)
		assert.Len(t, details, 3)
	})
}
