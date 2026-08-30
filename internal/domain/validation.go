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
