// Package clierror provides structured CLI error types with per-class exit codes.
package clierror

import (
	"fmt"
	"io"
)

// ErrorCode identifies the class of error and maps to a stable exit code.
type ErrorCode int

const (
	CodeOK           ErrorCode = 0
	CodeNotFound     ErrorCode = 1
	CodeUnauthorized ErrorCode = 2
	CodeForbidden    ErrorCode = 3
	CodeBadRequest   ErrorCode = 4
	CodeConflict     ErrorCode = 5
	CodeServerError  ErrorCode = 6
	CodeNotReady     ErrorCode = 7
	CodeUsage        ErrorCode = 8
)

// CLIError is a structured error with a stable exit code and a directive hint.
type CLIError struct {
	Code    ErrorCode
	Message string
	Hint    string
}

// New constructs a CLIError.
func New(code ErrorCode, message, hint string) *CLIError {
	return &CLIError{Code: code, Message: message, Hint: hint}
}

// Error implements the error interface.
func (e *CLIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s %s", e.Message, e.Hint)
	}
	return e.Message
}

// Render writes a directive error line to w.
// Write errors are intentionally ignored — if the error writer itself fails there is nothing useful we can do.
func (e *CLIError) Render(w io.Writer) {
	if e.Hint != "" {
		_, _ = fmt.Fprintf(w, "Error: %s %s\n", e.Message, e.Hint)
	} else {
		_, _ = fmt.Fprintf(w, "Error: %s\n", e.Message)
	}
}

// ExitCode returns the integer exit code for this error class.
func (e *CLIError) ExitCode() int {
	return int(e.Code)
}

// HandleErr renders err to w and returns an exit code.
// Returns 0 for nil, ExitCode() for *CLIError, and 6 for all other errors.
func HandleErr(err error, w io.Writer) int {
	if err == nil {
		return 0
	}
	if ce, ok := err.(*CLIError); ok {
		ce.Render(w)
		return ce.ExitCode()
	}
	// Write errors on diagnostic output are intentionally ignored.
	_, _ = fmt.Fprintf(w, "Error: %s\n", err.Error())
	return int(CodeServerError)
}
