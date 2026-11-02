package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrContractPending means the client is implemented but the backend REST
	// contract (paths, payloads, schemas) has not been supplied yet. It is
	// never replaced with a simulated success.
	ErrContractPending = errors.New("backend API contract is not available yet (waiting for nani)")

	// ErrNotConfigured means no API base URL has been configured.
	ErrNotConfigured = errors.New("backend API URL is not configured")

	// ErrNotSignedIn means an operation needs an authenticated session.
	ErrNotSignedIn = errors.New("not signed in")

	// ErrEmptyResponse means a successful response had no JSON body when one
	// was required.
	ErrEmptyResponse = errors.New("API returned an empty response")

	// ErrResponseTooLarge protects the client from unbounded response bodies.
	ErrResponseTooLarge = errors.New("API response is too large")
)

// APIError is a structured error returned by the backend or by the transport
// while talking to it.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Code       string
	Message    string
	Details    json.RawMessage
	RequestID  string
	RetryAfter time.Duration
}

// Error implements error without ever including credentials or request bodies.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	if e.Method != "" || e.Path != "" {
		fmt.Fprintf(&b, "%s %s: ", e.Method, e.Path)
	}
	if e.StatusCode > 0 {
		fmt.Fprintf(&b, "%d %s", e.StatusCode, http.StatusText(e.StatusCode))
		if e.Message != "" {
			b.WriteString(": ")
		}
	}
	b.WriteString(e.Message)
	if e.Code != "" {
		fmt.Fprintf(&b, " (code=%s)", e.Code)
	}
	if e.RequestID != "" {
		fmt.Fprintf(&b, " [request_id=%s]", e.RequestID)
	}
	return strings.TrimSpace(b.String())
}

// IsStatus reports whether err is an APIError with the given HTTP status.
func IsStatus(err error, statusCode int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == statusCode
}

// IsUnauthorized reports a 401 response.
func IsUnauthorized(err error) bool { return IsStatus(err, http.StatusUnauthorized) }

// IsForbidden reports a 403 response.
func IsForbidden(err error) bool { return IsStatus(err, http.StatusForbidden) }

// IsNotFound reports a 404 response.
func IsNotFound(err error) bool { return IsStatus(err, http.StatusNotFound) }

// IsConflict reports a 409 response.
func IsConflict(err error) bool { return IsStatus(err, http.StatusConflict) }

// IsRateLimited reports a 429 response.
func IsRateLimited(err error) bool { return IsStatus(err, http.StatusTooManyRequests) }

// IsServerError reports any 5xx response.
func IsServerError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode >= 500 && apiErr.StatusCode <= 599
}

// IsContractPending reports whether err is caused by the missing contract.
func IsContractPending(err error) bool { return errors.Is(err, ErrContractPending) }

// IsNotConfigured reports whether err is caused by an unset API URL.
func IsNotConfigured(err error) bool { return errors.Is(err, ErrNotConfigured) }

// parseAPIError converts a non-2xx response body into an APIError. It accepts
// several common error envelopes because the final backend schema is not yet
// published; it never assumes that parsing succeeds.
func parseAPIError(statusCode int, method, path string, header http.Header, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		Method:     method,
		Path:       path,
		Message:    http.StatusText(statusCode),
	}
	if header != nil {
		apiErr.RequestID = firstNonEmpty(
			header.Get("X-Request-ID"),
			header.Get("X-Request-Id"),
			header.Get("Request-ID"),
		)
		if raw := header.Get("Retry-After"); raw != "" {
			if seconds, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && seconds >= 0 {
				apiErr.RetryAfter = time.Duration(seconds) * time.Second
			} else if when, err := http.ParseTime(raw); err == nil {
				if d := time.Until(when); d > 0 {
					apiErr.RetryAfter = d
				}
			}
		}
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return apiErr
	}

	var envelope struct {
		Error     json.RawMessage `json:"error"`
		Message   string          `json:"message"`
		Code      string          `json:"code"`
		RequestID string          `json:"request_id"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.RequestID != "" {
			apiErr.RequestID = envelope.RequestID
		}
		if envelope.Code != "" {
			apiErr.Code = envelope.Code
		}
		if envelope.Message != "" {
			apiErr.Message = envelope.Message
		}
		if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
			var nested struct {
				Code      string          `json:"code"`
				Message   string          `json:"message"`
				Details   json.RawMessage `json:"details"`
				RequestID string          `json:"request_id"`
			}
			if err := json.Unmarshal(envelope.Error, &nested); err == nil {
				if nested.Code != "" {
					apiErr.Code = nested.Code
				}
				if nested.Message != "" {
					apiErr.Message = nested.Message
				}
				if len(nested.Details) > 0 {
					apiErr.Details = nested.Details
				}
				if nested.RequestID != "" {
					apiErr.RequestID = nested.RequestID
				}
			} else {
				var text string
				if err := json.Unmarshal(envelope.Error, &text); err == nil && text != "" {
					apiErr.Message = text
				}
			}
		}
		if apiErr.Message != "" {
			return apiErr
		}
	}

	// The body was not a recognised JSON envelope. Use a short plain-text
	// excerpt; it helps humans debug without dumping an unbounded HTML page.
	apiErr.Message = truncateRunes(trimmed, 500)
	return apiErr
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateRunes(value string, max int) string {
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "…"
}
