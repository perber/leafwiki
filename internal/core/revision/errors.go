package revision

import (
	"errors"
	"fmt"
)

type LocalizedError struct {
	Code     string
	Message  string
	Template string
	Args     []string
	Cause    error
}

func (e *LocalizedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *LocalizedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newLocalizedError(code, message, template string, cause error, args ...string) *LocalizedError {
	return &LocalizedError{
		Code:     code,
		Message:  message,
		Template: template,
		Args:     append([]string(nil), args...),
		Cause:    cause,
	}
}

func AsLocalizedError(err error) (*LocalizedError, bool) {
	var localized *LocalizedError
	if !errors.As(err, &localized) {
		return nil, false
	}
	return localized, true
}
