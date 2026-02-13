// Package api provides response structures for the GophKeeper API.
package api

import (
	"time"

	"github.com/idudko/gophkeeper/internal/models"
)

// Response represents a generic API response structure.
type Response struct {
	Success bool          `json:"success"`
	Data    any           `json:"data,omitempty"`
	Error   *ErrorDetail  `json:"error,omitempty"`
	Meta    *ResponseMeta `json:"meta,omitempty"`
}

// ErrorDetail provides detailed error information.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details string `json:"details,omitempty"`
}

// ResponseMeta contains metadata about the response.
type ResponseMeta struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id,omitempty"`
	Version   string    `json:"version"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
	Total     int       `json:"total,omitempty"`
	HasMore   bool      `json:"has_more,omitempty"`
}

// RegisterResponse represents a successful registration response.
type RegisterResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginResponse represents a successful login response.
type LoginResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresAt    time.Time   `json:"expires_at"`
	User         UserSummary `json:"user"`
}

// RefreshTokenResponse represents a successful token refresh response.
type RefreshTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// UserSummary provides basic user information.
type UserSummary struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ItemResponse represents a single item response.
type ItemResponse struct {
	ID        string           `json:"id"`
	Type      models.DataType  `json:"type"`
	Version   int              `json:"version"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Data      ItemDataResponse `json:"data"`
}

// ItemDataResponse represents type-specific item data.
type ItemDataResponse struct {
	Password *models.PasswordData `json:"password,omitempty"`
	Text     *models.TextData     `json:"text,omitempty"`
	Binary   *models.BinaryData   `json:"binary,omitempty"`
	Card     *models.CardData     `json:"card,omitempty"`
}

// ItemsListResponse represents a list of items response.
type ItemsListResponse struct {
	Items      []ItemResponse `json:"items"`
	Total      int            `json:"total"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
	HasMore    bool           `json:"has_more"`
	LastSyncAt *time.Time     `json:"last_sync_at,omitempty"`
}

// SyncResponse represents a sync operation response.
type SyncResponse struct {
	ServerItems   []ItemResponse `json:"server_items"`
	Conflicts     []ConflictInfo `json:"conflicts,omitempty"`
	LastSyncAt    time.Time      `json:"last_sync_at"`
	SyncedCount   int            `json:"synced_count"`
	ConflictCount int            `json:"conflict_count"`
}

// ConflictInfo provides information about a sync conflict.
type ConflictInfo struct {
	ItemID         string          `json:"item_id"`
	ItemType       models.DataType `json:"item_type"`
	LocalVersion   int             `json:"local_version"`
	ServerVersion  int             `json:"server_version"`
	LocalData      any             `json:"local_data"`
	ServerData     any             `json:"server_data"`
	ResolutionHint string          `json:"resolution_hint,omitempty"`
}

// ItemCreatedResponse represents the response after creating an item.
type ItemCreatedResponse struct {
	ID        string          `json:"id"`
	Type      models.DataType `json:"type"`
	Version   int             `json:"version"`
	CreatedAt time.Time       `json:"created_at"`
}

// ItemUpdatedResponse represents the response after updating an item.
type ItemUpdatedResponse struct {
	ID        string          `json:"id"`
	Type      models.DataType `json:"type"`
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// ItemDeletedResponse represents the response after deleting an item.
type ItemDeletedResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// VersionResponse represents the version information response.
type VersionResponse struct {
	Version    string    `json:"version"`
	BuildDate  time.Time `json:"build_date"`
	GitCommit  string    `json:"git_commit,omitempty"`
	GoVersion  string    `json:"go_version"`
	Platform   string    `json:"platform"`
	APIVersion string    `json:"api_version"`
	Features   []string  `json:"features"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
	Database  DatabaseStatus    `json:"database"`
	Uptime    string            `json:"uptime"`
}

// DatabaseStatus represents the database connection status.
type DatabaseStatus struct {
	Status    string `json:"status"`
	Latency   string `json:"latency,omitempty"`
	Connected bool   `json:"connected"`
}

// ChangePasswordResponse represents the response after changing a password.
type ChangePasswordResponse struct {
	UpdatedAt time.Time `json:"updated_at"`
}

// ErrorResponse represents a standardized error response.
type ErrorResponse struct {
	Error     ErrorDetail `json:"error"`
	Timestamp time.Time   `json:"timestamp"`
	Path      string      `json:"path,omitempty"`
}

// ValidationErrorResponse represents a validation error response.
type ValidationErrorResponse struct {
	Error     ErrorDetail  `json:"error"`
	Fields    []FieldError `json:"fields"`
	Timestamp time.Time    `json:"timestamp"`
}

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// SuccessResponse is a convenience type for successful responses.
type SuccessResponse struct {
	Message   string    `json:"message,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ListResponse represents a generic paginated list response.
type ListResponse struct {
	Items   any  `json:"items"`
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// NewResponse creates a new generic response.
func NewResponse(success bool, data any, err *ErrorDetail) Response {
	return Response{
		Success: success,
		Data:    data,
		Error:   err,
	}
}

// NewError creates a new error detail.
func NewError(code, message, field string) *ErrorDetail {
	return &ErrorDetail{
		Code:    code,
		Message: message,
		Field:   field,
	}
}

// NewSuccessResponse creates a successful response with metadata.
func NewSuccessResponse(data any, meta *ResponseMeta) Response {
	return Response{
		Success: true,
		Data:    data,
		Meta:    meta,
	}
}

// NewErrorResponse creates an error response.
func NewErrorResponse(code, message string) Response {
	return Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}
