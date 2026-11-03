package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/ben-anass/DouzTach/client/validation"
)

// Stable process exit codes so scripts can react to failures without parsing
// the human-readable message.
const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitNotSignedIn = 3
	ExitUnavailable = 4
	ExitNotFound    = 5
	ExitRateLimited = 6
	ExitServerError = 7
	ExitCancelled   = 130
)

// PendingError reports a feature that is blocked on the backend API contract.
// It unwraps to api.ErrContractPending so callers can use errors.Is.
type PendingError struct {
	Feature string
}

// Error implements error.
func (e *PendingError) Error() string {
	if e == nil {
		return api.ErrContractPending.Error()
	}
	return fmt.Sprintf("%s is not connected yet: %v", e.Feature, api.ErrContractPending)
}

// Unwrap exposes the shared sentinel.
func (e *PendingError) Unwrap() error { return api.ErrContractPending }

func pending(feature string) error { return &PendingError{Feature: feature} }

// errNotSignedIn returns the standard sign-in guidance.
func errNotSignedIn() error {
	return fmt.Errorf("%w; run 'douztech login'", api.ErrNotSignedIn)
}

// isPending reports whether err is a contract-pending error.
func isPending(err error) bool { return errors.Is(err, api.ErrContractPending) }

// ExitCode maps a command error to a stable process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	switch {
	case errors.Is(err, prompts.ErrCancelled):
		return ExitCancelled
	case errors.Is(err, api.ErrNotSignedIn):
		return ExitNotSignedIn
	case errors.Is(err, api.ErrContractPending), errors.Is(err, api.ErrNotConfigured):
		return ExitUnavailable
	case validation.IsFieldError(err):
		return ExitUsage
	}
	switch {
	case api.IsNotFound(err):
		return ExitNotFound
	case api.IsRateLimited(err):
		return ExitRateLimited
	case api.IsServerError(err):
		return ExitServerError
	case api.IsUnauthorized(err), api.IsForbidden(err):
		return ExitNotSignedIn
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		return ExitUsage
	}
	return ExitFailure
}

// ReportError writes a single user-facing error line. Pending-contract errors
// are reported as a warning with the reason, because the client is complete and
// the missing piece is the backend contract.
func ReportError(w io.Writer, s styles.Styles, err error) {
	if err == nil || w == nil {
		return
	}
	if isPending(err) {
		fmt.Fprintln(w, s.Warning.Render("! "+err.Error()))
		fmt.Fprintln(w, s.Help.Render("  The client is ready; the backend endpoints are still missing. Run 'douztech doctor' for details."))
		return
	}
	fmt.Fprintln(w, s.Error.Render("✗ "+err.Error()))
}
