package errors

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func NewFieldError(field, message string) *FieldError {
	return &FieldError{Field: field, Message: message}
}

type ValidationErrors struct {
	Errors []*FieldError `json:"fields"`
}

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{Errors: []*FieldError{}}
}

func (v *ValidationErrors) Add(field, message string) {
	v.Errors = append(v.Errors, NewFieldError(field, message))
}

func (v *ValidationErrors) Error() string {
	return "validation error"
}

func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}
