package dto

// ErrorResponse is the localized error envelope: a single machine-readable
// error object.
type ErrorResponse struct {
	// Error carries the localizable error details.
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes one localizable error. The client translates by Code
// and falls back to Template, substituting Args.
type ErrorDetail struct {
	// Code is the stable machine-readable error code, e.g. "page_not_found".
	Code string `json:"code"`
	// Message is the English human-readable message.
	Message string `json:"message"`
	// Template is the message template used for client-side localization.
	Template string `json:"template"`
	// Args are the values substituted into Template; omitted when empty.
	Args []string `json:"args,omitempty"`
}

// LegacyError is the flat error envelope used by middleware and older
// handlers: the error is a plain string rather than an ErrorDetail.
type LegacyError struct {
	// Error is the human-readable error message.
	Error string `json:"error"`
}
