package pagedisplay

import "slices"

// invalidInputCodes lists the error codes that mean "the caller sent a value
// this package rejects". A new setting with its own validation adds its code
// here, so the HTTP layer maps it to 400 without learning about it.
var invalidInputCodes = []string{ErrCodeInvalidContentWidth}

// IsInvalidInput reports whether code is one of this package's invalid-input
// error codes (as opposed to a persist failure).
func IsInvalidInput(code string) bool { return slices.Contains(invalidInputCodes, code) }
