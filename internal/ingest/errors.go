package ingest

import (
	"errors"
	"fmt"

	"samebits.com/evidra/pkg/evidence"
)

// Code is a stable ingest service error code for callers.
type Code string

const (
	ErrCodeInvalidInput       Code = "invalid_input"
	ErrCodeNotFound           Code = "not_found"
	ErrCodeInternal           Code = "internal_error"
	ErrCodeNoSignerConfigured Code = "no_signer_configured"
)

// Error is a typed ingest error used by caller layers.
type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorCode extracts the typed ingest error code from err.
func ErrorCode(err error) Code {
	var ingestErr *Error
	if errors.As(err, &ingestErr) {
		return ingestErr.Code
	}
	return ""
}

func wrapError(code Code, message string, err error) error {
	return &Error{Code: code, Message: message, Err: err}
}

func requiredSigner(signer evidence.Signer) error {
	if signer == nil {
		return wrapError(ErrCodeNoSignerConfigured, "evidence signer is required", fmt.Errorf("nil signer"))
	}
	return nil
}
