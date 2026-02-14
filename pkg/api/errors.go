// Package api provides error definitions and utilities for the GophKeeper API.
package api

import "errors"

// Validation error definitions for API requests.

var (
	// Email validation errors
	ErrEmailRequired = errors.New("email is required")
	ErrEmailInvalid  = errors.New("invalid email format")
	ErrEmailTooLong  = errors.New("email is too long (max 255 characters)")

	// Password validation errors
	ErrPasswordRequired = errors.New("password is required")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password must not exceed 128 characters")

	// Item validation errors
	ErrInvalidDataType      = errors.New("invalid data type, must be one of: password, text, binary, card")
	ErrInvalidVersion       = errors.New("invalid version number")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrMultipleDataTypes    = errors.New("only one data type can be specified per item")

	// Password data validation errors
	ErrLoginRequired = errors.New("login is required")

	// Text data validation errors
	ErrDataRequired = errors.New("data is required")
	ErrDataTooLarge = errors.New("data size exceeds maximum limit")

	// Binary data validation errors
	ErrFilenameRequired = errors.New("filename is required")

	// Card data validation errors
	ErrInvalidCardNumber  = errors.New("invalid card number (must be 16 digits)")
	ErrHolderNameRequired = errors.New("cardholder name is required")
	ErrInvalidExpiryDate  = errors.New("invalid expiry date (must be in MM/YY format)")
	ErrInvalidCVV         = errors.New("invalid CVV (must be 3 digits)")

	// Sync and conflict validation errors
	ErrItemIDRequired     = errors.New("item ID is required")
	ErrResolutionRequired = errors.New("resolution is required")

	// General validation errors
	ErrInvalidLimit     = errors.New("invalid limit value (must be between 1 and 1000)")
	ErrInvalidOffset    = errors.New("invalid offset value (must be non-negative)")
	ErrInvalidSortBy    = errors.New("invalid sort field (must be one of: created_at, updated_at, meta)")
	ErrInvalidSortOrder = errors.New("invalid sort order (must be 'asc' or 'desc')")
	ErrInvalidUUID      = errors.New("invalid UUID format")

	// Request processing errors
	ErrInvalidJSON            = errors.New("invalid JSON format")
	ErrRequestTooLarge        = errors.New("request body too large")
	ErrUnsupportedContentType = errors.New("unsupported content type")
)

// ValidationError represents a field validation error with context.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// ValidationErrors represents a collection of validation errors.
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

// Error implements the error interface.
func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return "validation failed: " + e.Errors[0].Error()
}

// HasErrors returns true if there are validation errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Add adds a new validation error.
func (e *ValidationErrors) Add(field, message, code string) {
	e.Errors = append(e.Errors, ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	})
}

// AddError adds a ValidationError to the collection.
func (e *ValidationErrors) AddError(err ValidationError) {
	e.Errors = append(e.Errors, err)
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message, code string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}

// NewValidationErrors creates a new ValidationErrors collection.
func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make([]ValidationError, 0),
	}
}

// Common validation error codes
const (
	CodeRequired    = "REQUIRED"
	CodeInvalid     = "INVALID"
	CodeTooShort    = "TOO_SHORT"
	CodeTooLong     = "TOO_LONG"
	CodeFormat      = "INVALID_FORMAT"
	CodeOutOfRange  = "OUT_OF_RANGE"
	CodeAlreadyUsed = "ALREADY_USED"
	CodeNotAllowed  = "NOT_ALLOWED"
)
