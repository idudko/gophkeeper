// Package models provides data models and error types for GophKeeper password manager system.
package models

import "errors"

// Common error definitions for GophKeeper system.

var (
	// Authentication and authorization errors
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")

	// Data storage errors
	ErrItemNotFound    = errors.New("item not found")
	ErrVersionConflict = errors.New("version conflict")

	// Client validation errors
	ErrInvalidDataTypeChoice       = errors.New("неверный выбор. Допустимые значения: 1-4")
	ErrLoginRequiredForPassword    = errors.New("логин обязателен для типа password")
	ErrPasswordRequiredForPassword = errors.New("пароль обязателен для типа password")
	ErrTextRequired                = errors.New("текст обязателен для типа text")
	ErrFilePathRequired            = errors.New("путь к файлу обязателен для типа binary")
	ErrCardNumberRequired          = errors.New("номер карты обязателен для типа card")
	ErrCardHolderRequired          = errors.New("имя владельца обязательно для типа card")
	ErrCardExpiryRequired          = errors.New("срок действия обязателен для типа card")
	ErrCardCVVRequired             = errors.New("CVV код обязателен для типа card")

	// Client authentication errors
	ErrEmailRequiredForLogin       = errors.New("email обязателен. Используйте --email")
	ErrPasswordRequiredForLogin    = errors.New("пароль обязателен. Используйте --password")
	ErrEmailRequiredForRegister    = errors.New("email обязателен. Используйте --email")
	ErrPasswordRequiredForRegister = errors.New("пароль обязателен. Используйте --password")

	// Client storage errors
	ErrTokenNotFound          = errors.New("токен не найден. Выполните команду 'login' для авторизации")
	ErrMasterPasswordNotFound = errors.New("мастер-пароль не найден. Выполните команду 'login' или введите мастер-пароль")
	ErrEncryptionSaltNotFound = errors.New("salt шифрования не найден. Выполните команду 'login'")

	// Master password validation errors
	ErrMasterPasswordTooShort       = errors.New("мастер-пароль должен содержать минимум 12 символов")
	ErrMasterPasswordTooLong        = errors.New("мастер-пароль не должен превышать 64 символов")
	ErrMasterPasswordMissingUpper   = errors.New("мастер-пароль должен содержать минимум 1 заглавную букву")
	ErrMasterPasswordMissingLower   = errors.New("мастер-пароль должен содержать минимум 1 строчную букву")
	ErrMasterPasswordMissingDigit   = errors.New("мастер-пароль должен содержать минимум 1 цифру")
	ErrMasterPasswordMissingSpecial = errors.New("мастер-пароль должен содержать минимум 1 специальный символ (!@#$%^&* и т.д.)")
	ErrPasswordsMismatch            = errors.New("пароли не совпадают")
	ErrPasswordsMustBeDifferent     = errors.New("новый пароль должен отличаться от старого")
)

// AppError represents an application error with additional context.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error implements error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError.
func NewAppError(code, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Common error codes
const (
	// Authentication and authorization error codes
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeUserAlreadyExists  = "USER_ALREADY_EXISTS"
	CodeUserNotFound       = "USER_NOT_FOUND"
	CodeUserExists         = "USER_EXISTS"
	CodeTokenError         = "TOKEN_ERROR"
	CodeTokenExpired       = "TOKEN_EXPIRED"
	CodeTokenRequired      = "TOKEN_REQUIRED"
	CodeRefreshError       = "REFRESH_ERROR"

	// Request validation error codes
	CodeInvalidJSON         = "INVALID_JSON"
	CodeEmailRequired       = "EMAIL_REQUIRED"
	CodePasswordTooShort    = "PASSWORD_TOO_SHORT"
	CodeCredentialsRequired = "CREDENTIALS_REQUIRED"
	CodePasswordsRequired   = "PASSWORDS_REQUIRED"

	// Data validation error codes
	CodeInvalidType       = "INVALID_TYPE"
	CodeTypeRequired      = "TYPE_REQUIRED"
	CodeDataRequired      = "DATA_REQUIRED"
	CodeMultipleDataTypes = "MULTIPLE_DATA_TYPES"
	CodeMissingFields     = "MISSING_FIELDS"
	CodeItemIDRequired    = "ITEM_ID_REQUIRED"
	CodeInvalidItemID     = "INVALID_ITEM_ID"

	// Data operations error codes
	CodeCreateError     = "CREATE_ERROR"
	CodeCreateUserError = "CREATE_USER_ERROR"
	CodeGetItemsError   = "GET_ITEMS_ERROR"
	CodeGetItemError    = "GET_ITEM_ERROR"
	CodeUpdateError     = "UPDATE_ERROR"
	CodeDeleteError     = "DELETE_ERROR"
	CodeNotFound        = "NOT_FOUND"
	CodeVersionConflict = "VERSION_CONFLICT"

	// Internal service error codes
	CodeHashError           = "HASH_ERROR"
	CodeSaltGenerationError = "SALT_GENERATION_ERROR"
)
