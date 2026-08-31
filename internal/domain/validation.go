package domain

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateCandidateInput checks core candidate attributes against domain rules.
func ValidateCandidateInput(name, email string, phone, linkedinURL *string) []ErrorDetail {
	var details []ErrorDetail

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		details = append(details, ErrorDetail{
			Field: "candidate_name",
			Issue: "Candidate name is required",
		})
	} else if len(trimmedName) > 255 {
		details = append(details, ErrorDetail{
			Field: "candidate_name",
			Issue: "Candidate name cannot exceed 255 characters",
		})
	}

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		details = append(details, ErrorDetail{
			Field: "candidate_email",
			Issue: "Candidate email is required",
		})
	} else if len(trimmedEmail) > 255 {
		details = append(details, ErrorDetail{
			Field: "candidate_email",
			Issue: "Candidate email cannot exceed 255 characters",
		})
	} else if !IsValidEmail(trimmedEmail) {
		details = append(details, ErrorDetail{
			Field: "candidate_email",
			Issue: "Candidate email must be a valid email address",
		})
	}

	if phone != nil {
		p := strings.TrimSpace(*phone)
		if p != "" && len(p) > 50 {
			details = append(details, ErrorDetail{
				Field: "candidate_phone",
				Issue: "Candidate phone cannot exceed 50 characters",
			})
		}
	}

	if linkedinURL != nil {
		u := strings.TrimSpace(*linkedinURL)
		if u != "" {
			if len(u) > 500 {
				details = append(details, ErrorDetail{
					Field: "linkedin_url",
					Issue: "LinkedIn URL cannot exceed 500 characters",
				})
			} else if !IsValidURL(u) {
				details = append(details, ErrorDetail{
					Field: "linkedin_url",
					Issue: "LinkedIn URL must be a valid URL",
				})
			}
		}
	}

	return details
}

// ValidateCustomAnswers validates dynamic custom form answers against job custom fields schema.
func ValidateCustomAnswers(fields []CustomField, answers map[string]interface{}) []ErrorDetail {
	var details []ErrorDetail
	if answers == nil {
		answers = make(map[string]interface{})
	}

	for _, field := range fields {
		val, exists := answers[field.ID]
		fieldKey := fmt.Sprintf("custom_answers.%s", field.ID)

		if field.Required {
			if !exists || val == nil || IsEmptyValue(val) {
				details = append(details, ErrorDetail{
					Field: fieldKey,
					Issue: fmt.Sprintf("%s is required", field.Label),
				})
				continue
			}
		}

		if exists && val != nil && !IsEmptyValue(val) {
			switch strings.ToLower(field.Type) {
			case "number":
				if !IsValidNumber(val) {
					details = append(details, ErrorDetail{
						Field: fieldKey,
						Issue: fmt.Sprintf("%s must be a valid number", field.Label),
					})
				}
			case "url":
				if !IsValidURL(val) {
					details = append(details, ErrorDetail{
						Field: fieldKey,
						Issue: fmt.Sprintf("%s must be a valid URL", field.Label),
					})
				}
			case "boolean":
				if !IsValidBoolean(val) {
					details = append(details, ErrorDetail{
						Field: fieldKey,
						Issue: fmt.Sprintf("%s must be a boolean", field.Label),
					})
				}
			case "select":
				if len(field.Options) > 0 && !IsValidOption(val, field.Options) {
					details = append(details, ErrorDetail{
						Field: fieldKey,
						Issue: fmt.Sprintf("%s must be one of: %s", field.Label, strings.Join(field.Options, ", ")),
					})
				}
			}
		}
	}

	return details
}

// IsEmptyValue returns true if a dynamic field value is nil or empty.
func IsEmptyValue(val interface{}) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []interface{}:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// IsValidEmail checks if an email string is well-formed.
func IsValidEmail(email string) bool {
	if !emailRegex.MatchString(email) {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// IsValidURL checks if a value is a valid HTTP or HTTPS URL.
func IsValidURL(val interface{}) bool {
	s, ok := val.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// IsValidNumber checks if a value is a valid numeric type or parsable number string.
func IsValidNumber(val interface{}) bool {
	switch v := val.(type) {
	case float64, float32, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return true
	case string:
		_, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return err == nil
	default:
		return false
	}
}

// IsValidBoolean checks if a value is a boolean or parsable boolean string.
func IsValidBoolean(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return true
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "false"
	default:
		return false
	}
}

// IsValidOption checks if a value matches one of the allowed options.
func IsValidOption(val interface{}, options []string) bool {
	strVal := strings.TrimSpace(fmt.Sprint(val))
	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(opt), strVal) {
			return true
		}
	}
	return false
}

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var customFieldIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// IsValidSlug verifies if a slug is url-safe lowercase alphanumeric with hyphens.
func IsValidSlug(slug string) bool {
	if len(slug) == 0 || len(slug) > 255 {
		return false
	}
	return slugRegex.MatchString(slug)
}

// GenerateSlug generates a clean, URL-safe slug from a title string.
func GenerateSlug(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	lastHyphen := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
		} else if !lastHyphen && b.Len() > 0 {
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		return "job"
	}
	if len(res) > 255 {
		res = res[:255]
		res = strings.TrimRight(res, "-")
	}
	return res
}

// IsValidEmploymentType checks if the employment type is one of the valid enum values.
func IsValidEmploymentType(empType EmploymentType) bool {
	switch empType {
	case EmploymentTypeFullTime, EmploymentTypePartTime, EmploymentTypeContract, EmploymentTypeInternship:
		return true
	default:
		return false
	}
}

// IsValidJobStatus checks if the status is one of the valid enum values.
func IsValidJobStatus(status JobStatus) bool {
	switch status {
	case JobStatusDraft, JobStatusPublished, JobStatusArchived:
		return true
	default:
		return false
	}
}

// IsValidStage checks if the stage is one of the valid enum values.
func IsValidStage(stage ApplicationStage) bool {
	switch stage {
	case ApplicationStageApplied, ApplicationStageScreening, ApplicationStageInterviewing,
		ApplicationStageOffer, ApplicationStageHired, ApplicationStageRejected:
		return true
	default:
		return false
	}
}

// ValidateCustomFields checks custom question field definitions.
func ValidateCustomFields(fields []CustomField) []ErrorDetail {
	var details []ErrorDetail
	seenIDs := make(map[string]bool)

	for i, field := range fields {
		fieldPrefix := fmt.Sprintf("custom_fields[%d]", i)
		trimmedID := strings.TrimSpace(field.ID)
		if trimmedID == "" {
			details = append(details, ErrorDetail{
				Field: fmt.Sprintf("%s.id", fieldPrefix),
				Issue: "Custom field id is required",
			})
		} else if !customFieldIDRegex.MatchString(trimmedID) {
			details = append(details, ErrorDetail{
				Field: fmt.Sprintf("%s.id", fieldPrefix),
				Issue: "Custom field id must contain only alphanumeric characters, underscores, or hyphens",
			})
		} else if seenIDs[trimmedID] {
			details = append(details, ErrorDetail{
				Field: fmt.Sprintf("%s.id", fieldPrefix),
				Issue: fmt.Sprintf("Duplicate custom field id: %s", trimmedID),
			})
		} else {
			seenIDs[trimmedID] = true
		}

		if strings.TrimSpace(field.Label) == "" {
			details = append(details, ErrorDetail{
				Field: fmt.Sprintf("%s.label", fieldPrefix),
				Issue: "Custom field label is required",
			})
		}

		fType := strings.ToLower(strings.TrimSpace(field.Type))
		switch fType {
		case "text", "number", "url", "select", "textarea", "boolean":
			if fType == "select" {
				if len(field.Options) == 0 {
					details = append(details, ErrorDetail{
						Field: fmt.Sprintf("%s.options", fieldPrefix),
						Issue: "Custom field of type 'select' must have at least one option",
					})
				} else {
					for optIdx, opt := range field.Options {
						if strings.TrimSpace(opt) == "" {
							details = append(details, ErrorDetail{
								Field: fmt.Sprintf("%s.options[%d]", fieldPrefix, optIdx),
								Issue: "Option value cannot be blank",
							})
						}
					}
				}
			}
		default:
			details = append(details, ErrorDetail{
				Field: fmt.Sprintf("%s.type", fieldPrefix),
				Issue: "Invalid field type. Must be one of: text, number, url, select, textarea, boolean",
			})
		}
	}

	return details
}

// ValidateJobInput checks all attributes when creating a job.
func ValidateJobInput(
	title, department, location, descriptionMarkdown string,
	empType EmploymentType,
	status JobStatus,
	experienceLevel *string,
	salaryMin, salaryMax *float64,
	salaryCurrency string,
	slug string,
	customFields []CustomField,
) []ErrorDetail {
	var details []ErrorDetail

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		details = append(details, ErrorDetail{
			Field: "title",
			Issue: "Title is required",
		})
	} else if len(trimmedTitle) > 255 {
		details = append(details, ErrorDetail{
			Field: "title",
			Issue: "Title cannot exceed 255 characters",
		})
	}

	trimmedDept := strings.TrimSpace(department)
	if trimmedDept == "" {
		details = append(details, ErrorDetail{
			Field: "department",
			Issue: "Department is required",
		})
	} else if len(trimmedDept) > 100 {
		details = append(details, ErrorDetail{
			Field: "department",
			Issue: "Department cannot exceed 100 characters",
		})
	}

	trimmedLoc := strings.TrimSpace(location)
	if trimmedLoc == "" {
		details = append(details, ErrorDetail{
			Field: "location",
			Issue: "Location is required",
		})
	} else if len(trimmedLoc) > 150 {
		details = append(details, ErrorDetail{
			Field: "location",
			Issue: "Location cannot exceed 150 characters",
		})
	}

	if strings.TrimSpace(descriptionMarkdown) == "" {
		details = append(details, ErrorDetail{
			Field: "description_markdown",
			Issue: "Description markdown is required",
		})
	}

	if empType != "" && !IsValidEmploymentType(empType) {
		details = append(details, ErrorDetail{
			Field: "employment_type",
			Issue: "Invalid employment_type. Must be one of: full_time, part_time, contract, internship",
		})
	}

	if status != "" && !IsValidJobStatus(status) {
		details = append(details, ErrorDetail{
			Field: "status",
			Issue: "Invalid status. Must be one of: draft, published, archived",
		})
	}

	if experienceLevel != nil {
		if len(strings.TrimSpace(*experienceLevel)) > 50 {
			details = append(details, ErrorDetail{
				Field: "experience_level",
				Issue: "Experience level cannot exceed 50 characters",
			})
		}
	}

	if salaryCurrency != "" && len(strings.TrimSpace(salaryCurrency)) > 3 {
		details = append(details, ErrorDetail{
			Field: "salary_currency",
			Issue: "Salary currency code cannot exceed 3 characters",
		})
	}

	if salaryMin != nil && *salaryMin < 0 {
		details = append(details, ErrorDetail{
			Field: "salary_min",
			Issue: "Salary min must be greater than or equal to 0",
		})
	}

	if salaryMax != nil && *salaryMax < 0 {
		details = append(details, ErrorDetail{
			Field: "salary_max",
			Issue: "Salary max must be greater than or equal to 0",
		})
	}

	if salaryMin != nil && salaryMax != nil && *salaryMin > *salaryMax {
		details = append(details, ErrorDetail{
			Field: "salary_min",
			Issue: "Salary min cannot exceed salary max",
		})
	}

	if slug != "" && !IsValidSlug(slug) {
		details = append(details, ErrorDetail{
			Field: "slug",
			Issue: "Slug must be a URL-safe lowercase alphanumeric string with hyphens (max 255 chars)",
		})
	}

	if len(customFields) > 0 {
		cfDetails := ValidateCustomFields(customFields)
		if len(cfDetails) > 0 {
			details = append(details, cfDetails...)
		}
	}

	return details
}

// ValidateStageUpdate validates candidate stage transition input.
func ValidateStageUpdate(stage ApplicationStage, rejectedReason *string) []ErrorDetail {
	var details []ErrorDetail

	if !IsValidStage(stage) {
		details = append(details, ErrorDetail{
			Field: "stage",
			Issue: "Invalid stage. Must be one of: applied, screening, interviewing, offer, hired, rejected",
		})
	}

	if rejectedReason != nil && len(strings.TrimSpace(*rejectedReason)) > 255 {
		details = append(details, ErrorDetail{
			Field: "rejected_reason",
			Issue: "Rejected reason cannot exceed 255 characters",
		})
	}

	return details
}

// ValidateNoteInput checks author name and note text for application notes.
func ValidateNoteInput(authorName, noteText string) []ErrorDetail {
	var details []ErrorDetail

	trimmedAuthor := strings.TrimSpace(authorName)
	if trimmedAuthor == "" {
		details = append(details, ErrorDetail{
			Field: "author_name",
			Issue: "Author name is required",
		})
	} else if len(trimmedAuthor) > 255 {
		details = append(details, ErrorDetail{
			Field: "author_name",
			Issue: "Author name cannot exceed 255 characters",
		})
	}

	if strings.TrimSpace(noteText) == "" {
		details = append(details, ErrorDetail{
			Field: "note_text",
			Issue: "Note text is required",
		})
	}

	return details
}

// ValidateWebhookSubscriptionInput validates target URL and event subscription list.
func ValidateWebhookSubscriptionInput(targetURL string, events []string, secretToken *string) []ErrorDetail {
	var details []ErrorDetail

	trimmedURL := strings.TrimSpace(targetURL)
	if trimmedURL == "" {
		details = append(details, ErrorDetail{
			Field: "target_url",
			Issue: "target_url is required",
		})
	} else if len(trimmedURL) > 1024 {
		details = append(details, ErrorDetail{
			Field: "target_url",
			Issue: "target_url cannot exceed 1024 characters",
		})
	} else if !IsValidURL(trimmedURL) {
		details = append(details, ErrorDetail{
			Field: "target_url",
			Issue: "target_url must be a valid HTTP or HTTPS URL",
		})
	}

	if len(events) == 0 {
		details = append(details, ErrorDetail{
			Field: "events",
			Issue: "events array must contain at least one event",
		})
	} else {
		for i, event := range events {
			trimmedEvent := strings.TrimSpace(event)
			if trimmedEvent == "" {
				details = append(details, ErrorDetail{
					Field: fmt.Sprintf("events[%d]", i),
					Issue: "Event name cannot be blank",
				})
			} else if !IsValidWebhookEvent(trimmedEvent) {
				details = append(details, ErrorDetail{
					Field: fmt.Sprintf("events[%d]", i),
					Issue: fmt.Sprintf("Invalid event '%s'. Must be one of: %s, *", trimmedEvent, strings.Join(AllWebhookEvents(), ", ")),
				})
			}
		}
	}

	if secretToken != nil {
		trimmedSecret := strings.TrimSpace(*secretToken)
		if trimmedSecret != "" && len(trimmedSecret) > 255 {
			details = append(details, ErrorDetail{
				Field: "secret_token",
				Issue: "secret_token cannot exceed 255 characters",
			})
		}
	}

	return details
}
