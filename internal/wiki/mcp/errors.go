package mcp

import (
	stderrors "errors"
	"fmt"
	"strings"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// Tool error codes, following the <domain>_<reason> convention with the
// slice's domain prefix "mcp". They are part of the client contract: MCP
// clients branch on the code, never on the message.
const (
	// ErrCodeMCPInvalidInput: the tool's input failed validation (the 400
	// equivalent).
	ErrCodeMCPInvalidInput = "mcp_invalid_input"
	// ErrCodeMCPInternalError: an unexpected failure inside the tool (the 500
	// equivalent).
	ErrCodeMCPInternalError = "mcp_internal_error"
)

// ToolError is the client-facing form of a failed MCP tool call. Code and
// Message go to the client; Cause keeps the internal error for logging and
// must never reach a client response.
type ToolError struct {
	Code    string
	Message string
	Cause   error
}

func (e *ToolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// toolErrorFrom maps a use-case error onto a ToolError:
//   - *sharederrors.LocalizedError keeps its code and message — both are
//     client-facing by the error conventions.
//   - *sharederrors.ValidationErrors becomes mcp_invalid_input with the
//     field messages.
//   - anything else becomes mcp_internal_error with a generic message; the
//     internal detail stays in Cause, never in the client-visible parts.
func toolErrorFrom(err error) *ToolError {
	if err == nil {
		return nil
	}
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		return &ToolError{Code: loc.Code, Message: loc.Message, Cause: loc}
	}
	var vErr *sharederrors.ValidationErrors
	if stderrors.As(err, &vErr) {
		fieldMsgs := make([]string, 0, len(vErr.Errors))
		for _, fe := range vErr.Errors {
			fieldMsgs = append(fieldMsgs, fe.Field+": "+fe.Message)
		}
		return &ToolError{
			Code:    ErrCodeMCPInvalidInput,
			Message: strings.Join(fieldMsgs, "; "),
			Cause:   vErr,
		}
	}
	return &ToolError{Code: ErrCodeMCPInternalError, Message: "tool failed", Cause: err}
}
