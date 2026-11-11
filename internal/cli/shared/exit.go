package shared

import (
	"errors"
	"fmt"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
)

// Exit codes, so scripts can tell why a command failed
const (
	ExitOK          = 0
	ExitError       = 1 // Any other error
	ExitUsage       = 2 // Invalid arguments or flags
	ExitNotFound    = 3 // No application matches the argument
	ExitAmbiguous   = 4 // Several applications match the argument
	ExitApplyFailed = 5 // Some changes failed to apply or did not take effect
)

// ErrApplyFailed means some plan items failed to apply or did not take effect
var ErrApplyFailed = errors.New("failed to apply")

// UsageError is an error in how the command was invoked
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// Usagef returns a UsageError with a formatted message
func Usagef(format string, a ...any) error {
	return &UsageError{Err: fmt.Errorf(format, a...)}
}

// CheckResults returns an error wrapping ErrApplyFailed if any item failed
func CheckResults(results []domain.ApplyResult) error {
	failed := 0
	for _, r := range results {
		if r.Item.Status == domain.StatusFailed {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%w %d of %d item(s)", ErrApplyFailed, failed, len(results))
	}
	return nil
}

// ExitCode returns the process exit code for an error returned by a command
func ExitCode(err error) int {
	var usage *UsageError
	switch {
	case err == nil:
		return ExitOK
	case errors.As(err, &usage):
		return ExitUsage
	case errors.Is(err, domain.ErrAppNotFound):
		return ExitNotFound
	case errors.Is(err, domain.ErrAmbiguousApp):
		return ExitAmbiguous
	case errors.Is(err, ErrApplyFailed):
		return ExitApplyFailed
	default:
		return ExitError
	}
}

// errorCodes names each exit code in the error of the JSON or YAML output
var errorCodes = map[int]string{
	ExitError:       "error",
	ExitUsage:       "usage",
	ExitNotFound:    "not_found",
	ExitAmbiguous:   "ambiguous",
	ExitApplyFailed: "apply_failed",
}

// ErrorInfo describes err for the JSON or YAML output
func ErrorInfo(err error) *formatter.ErrorInfo {
	code := ExitCode(err)
	return &formatter.ErrorInfo{Code: errorCodes[code], ExitCode: code, Message: err.Error()}
}
