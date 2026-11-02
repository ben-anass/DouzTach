// Package validation contains reusable, user-facing input validation for the
// DouzTach CLI. It is deliberately independent of the backend so the same rules
// can be used by commands, prompts, and tests.
package validation

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxNameLength        = 80
	MaxEmailLength       = 254
	MaxPasswordLength    = 128
	MinPasswordLength    = 8
	MaxTitleLength       = 200
	MaxDescriptionLength = 5000
	MaxMessageLength     = 4000
	MaxIDLength          = 128
	MinTimeout           = time.Second
	MaxTimeout           = 5 * time.Minute
)

// FieldError describes a validation failure for a specific user-facing field.
// It intentionally contains no secrets.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s %s", e.Field, e.Message)
}

func fieldError(field, message string) error {
	return &FieldError{Field: field, Message: message}
}

// Required rejects blank values.
func Required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fieldError(field, "is required")
	}
	return nil
}

// Email validates a plain email address. Display-name forms such as
// "Jane <jane@example.com>" are rejected.
func Email(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	if utf8.RuneCountInString(value) > MaxEmailLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxEmailLength))
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || !strings.Contains(addr.Address, "@") {
		return fieldError(field, "must be a valid email address")
	}
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return fieldError(field, "must be a valid email address")
	}
	return nil
}

// Password applies the client-side password policy. The backend may enforce
// additional rules; this check exists to catch obvious mistakes before a
// request is sent.
func Password(field, value string) error {
	if value == "" {
		return fieldError(field, "is required")
	}
	if utf8.RuneCountInString(value) < MinPasswordLength {
		return fieldError(field, fmt.Sprintf("must be at least %d characters", MinPasswordLength))
	}
	if utf8.RuneCountInString(value) > MaxPasswordLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxPasswordLength))
	}
	var hasLetter, hasDigit bool
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsSpace(r):
			// Spaces are allowed, but a password made only of spaces is
			// already rejected by the length/letter/digit checks.
		}
	}
	if !hasLetter || !hasDigit {
		return fieldError(field, "must contain at least one letter and one number")
	}
	return nil
}

// LoginPassword only requires a non-empty value. Existing accounts may predate
// a newer password policy, so login must not reject them client-side.
func LoginPassword(field, value string) error {
	if value == "" {
		return fieldError(field, "is required")
	}
	return nil
}

// PersonName validates a display name.
func PersonName(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	n := utf8.RuneCountInString(value)
	if n < 2 {
		return fieldError(field, "must be at least 2 characters")
	}
	if n > MaxNameLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxNameLength))
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || r == '-' || r == '\'' || r == '.' {
			continue
		}
		return fieldError(field, "contains an unsupported character")
	}
	return nil
}

// WorkspaceName validates a workspace name.
func WorkspaceName(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	n := utf8.RuneCountInString(value)
	if n < 2 {
		return fieldError(field, "must be at least 2 characters")
	}
	if n > MaxNameLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxNameLength))
	}
	return nil
}

// TaskTitle validates a task title.
func TaskTitle(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	n := utf8.RuneCountInString(value)
	if n < 3 {
		return fieldError(field, "must be at least 3 characters")
	}
	if n > MaxTitleLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxTitleLength))
	}
	return nil
}

// Description validates an optional task or workspace description.
func Description(field, value string) error {
	if utf8.RuneCountInString(value) > MaxDescriptionLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxDescriptionLength))
	}
	return nil
}

// MessageBody validates a chat message body.
func MessageBody(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	if utf8.RuneCountInString(value) > MaxMessageLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxMessageLength))
	}
	return nil
}

// ID validates an opaque identifier supplied by the user. IDs are not assumed
// to be numeric or UUIDs because the backend contract is not finalised.
func ID(field, value string) error {
	value = strings.TrimSpace(value)
	if err := Required(field, value); err != nil {
		return err
	}
	if utf8.RuneCountInString(value) > MaxIDLength {
		return fieldError(field, fmt.Sprintf("must be at most %d characters", MaxIDLength))
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fieldError(field, "must not contain spaces or control characters")
		}
	}
	return nil
}

// BaseURL validates and normalises a backend API base URL. Credentials in the
// URL are rejected so they cannot leak through logs or process listings.
func BaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fieldError("API URL", "must be a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fieldError("API URL", "must use http or https")
	}
	if u.Host == "" {
		return "", fieldError("API URL", "must include a host")
	}
	if u.User != nil {
		return "", fieldError("API URL", "must not include credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fieldError("API URL", "must not include a query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

// Timeout validates a request timeout.
func Timeout(value time.Duration) error {
	if value < MinTimeout || value > MaxTimeout {
		return fieldError("timeout", fmt.Sprintf("must be between %s and %s", MinTimeout, MaxTimeout))
	}
	return nil
}

// ParseDuration parses a timeout string and validates its range.
func ParseDuration(field, value string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fieldError(field, "must be a duration such as 15s or 1m")
	}
	if err := Timeout(d); err != nil {
		return 0, err
	}
	return d, nil
}

// Date parses either YYYY-MM-DD or RFC3339 into a local date. Date-only values
// are interpreted as the end of that day so a task due today is not overdue.
func Date(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return t.Add(23*time.Hour + 59*time.Minute + 59*time.Second), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fieldError(field, "must use YYYY-MM-DD or RFC3339 format")
}

// Enum normalises a value against a list of allowed values, case-insensitively,
// and returns the canonical allowed spelling.
func Enum(field, value string, allowed []string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fieldError(field, "is required")
	}
	for _, candidate := range allowed {
		if strings.EqualFold(value, candidate) {
			return candidate, nil
		}
	}
	return "", fieldError(field, fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")))
}

// Check runs a list of validators and returns the first error.
func Check(validators ...error) error {
	for _, err := range validators {
		if err != nil {
			return err
		}
	}
	return nil
}

// IsFieldError reports whether err is a FieldError.
func IsFieldError(err error) bool {
	var fe *FieldError
	return errors.As(err, &fe)
}
