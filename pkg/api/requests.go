// Package api provides request structures for the GophKeeper API.
package api

import (
	"time"

	"github.com/idudko/gophkeeper/internal/models"
)

// RegisterRequest represents a user registration request.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email,min=6,max=255"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

// LoginRequest represents a user login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenRequest represents a token refresh request.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128"`
}

// CreateItemRequest represents a request to create a new item.
type CreateItemRequest struct {
	Type models.DataType `json:"type" validate:"required,oneof=password text binary card"`
	Meta string          `json:"meta" validate:"max=256"`
	Data ItemDataRequest `json:"data" validate:"required"`
}

// UpdateItemRequest represents a request to update an existing item.
type UpdateItemRequest struct {
	Version int              `json:"version" validate:"required,min=1"`
	Meta    *string          `json:"meta,omitempty" validate:"omitempty,max=256"`
	Data    *ItemDataRequest `json:"data,omitempty"`
}

// ItemDataRequest represents type-specific item data for creation/update.
type ItemDataRequest struct {
	Password *PasswordDataRequest `json:"password,omitempty"`
	Text     *TextDataRequest     `json:"text,omitempty"`
	Binary   *BinaryDataRequest   `json:"binary,omitempty"`
	Card     *CardDataRequest     `json:"card,omitempty"`
}

// PasswordDataRequest represents login/password data.
type PasswordDataRequest struct {
	Meta     string `json:"meta" validate:"max=256"`
	Login    string `json:"login" validate:"required,min=3,max=128"`
	Password string `json:"password" validate:"required"`
}

// TextDataRequest represents arbitrary text data.
type TextDataRequest struct {
	Meta string `json:"meta" validate:"max=256"`
	Data string `json:"data" validate:"required,max=1048576"` // 1MB max
}

// BinaryDataRequest represents binary data.
type BinaryDataRequest struct {
	Meta     string `json:"meta" validate:"max=256"`
	Filename string `json:"filename" validate:"required,min=1,max=255"`
	Data     []byte `json:"data" validate:"required,maxbytes=10485760"` // 10MB max
}

// CardDataRequest represents bank card information.
type CardDataRequest struct {
	Meta       string `json:"meta" validate:"max=256"`
	Number     string `json:"number" validate:"required,len=16,numeric"`
	HolderName string `json:"holder_name" validate:"required,min=2,max=64"`
	ExpiryDate string `json:"expiry_date" validate:"required,regexp=^(0[1-9]|1[0-2])/[0-9]{2}$"` // MM/YY
	CVV        string `json:"cvv" validate:"required,len=3,numeric"`
}

// GetItemsRequest represents a request to list items with optional filtering.
type GetItemsRequest struct {
	DataType    models.DataType `query:"type" validate:"omitempty,oneof=password text binary card"`
	Limit       int             `query:"limit" validate:"omitempty,min=1,max=1000"`
	Offset      int             `query:"offset" validate:"omitempty,min=0"`
	Since       *time.Time      `query:"since" validate:"omitempty"`
	Until       *time.Time      `query:"until" validate:"omitempty"`
	SearchQuery string          `query:"search" validate:"omitempty,max=256"`
	SortBy      string          `query:"sort_by" validate:"omitempty,oneof=created_at updated_at meta"`
	SortOrder   string          `query:"sort_order" validate:"omitempty,oneof=asc desc"`
}

// GetItemRequest represents a request to get a specific item.
type GetItemRequest struct {
	ItemID string `path:"item_id" validate:"required,uuid"`
}

// DeleteItemRequest represents a request to delete an item.
type DeleteItemRequest struct {
	ItemID string `path:"item_id" validate:"required,uuid"`
}

// SyncRequest represents a sync request from client to server.
type SyncRequest struct {
	LastSyncAt *time.Time        `json:"last_sync_at" validate:"omitempty"`
	Items      []SyncItemRequest `json:"items" validate:"required,dive"`
}

// SyncItemRequest represents an item in a sync request.
type SyncItemRequest struct {
	ID        string          `json:"id" validate:"required,uuid"`
	Type      models.DataType `json:"type" validate:"required,oneof=password text binary card"`
	Version   int             `json:"version" validate:"required,min=0"`
	CreatedAt time.Time       `json:"created_at" validate:"required"`
	UpdatedAt time.Time       `json:"updated_at" validate:"required"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
	Meta      string          `json:"meta" validate:"max=256"`
	Data      ItemDataRequest `json:"data" validate:"required"`
}

// ResolveConflictRequest represents a request to resolve a sync conflict.
type ResolveConflictRequest struct {
	ItemID     string             `json:"item_id" validate:"required,uuid"`
	Resolution ConflictResolution `json:"resolution" validate:"required,oneof=local server merge"`
	Data       *ItemDataRequest   `json:"data,omitempty"`
}

// ConflictResolution represents how to resolve a sync conflict.
type ConflictResolution string

const (
	// ResolutionUseLocal uses the client's version of the data.
	ResolutionUseLocal ConflictResolution = "local"
	// ResolutionUseServer uses the server's version of the data.
	ResolutionUseServer ConflictResolution = "server"
	// ResolutionMerge attempts to merge both versions.
	ResolutionMerge ConflictResolution = "merge"
)

// Validate performs basic validation on the request.
func (r *RegisterRequest) Validate() error {
	if r.Email == "" {
		return ErrEmailRequired
	}
	if len(r.Password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(r.Password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}

// Validate performs basic validation on the login request.
func (r *LoginRequest) Validate() error {
	if r.Email == "" {
		return ErrEmailRequired
	}
	if r.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}

// Validate performs validation on the create item request.
func (r *CreateItemRequest) Validate() error {
	if !isValidDataType(r.Type) {
		return ErrInvalidDataType
	}

	switch r.Type {
	case models.DataTypePassword:
		if r.Data.Password == nil || r.Data.Password.Login == "" || r.Data.Password.Password == "" {
			return ErrMissingRequiredField
		}
	case models.DataTypeText:
		if r.Data.Text == nil || r.Data.Text.Data == "" {
			return ErrMissingRequiredField
		}
	case models.DataTypeBinary:
		if r.Data.Binary == nil || r.Data.Binary.Filename == "" || len(r.Data.Binary.Data) == 0 {
			return ErrMissingRequiredField
		}
	case models.DataTypeCard:
		if r.Data.Card == nil || r.Data.Card.Number == "" || r.Data.Card.CVV == "" {
			return ErrMissingRequiredField
		}
	}

	return nil
}

// Validate performs validation on the update item request.
func (r *UpdateItemRequest) Validate() error {
	if r.Version <= 0 {
		return ErrInvalidVersion
	}

	if r.Data != nil {
		return r.Data.Validate()
	}

	return nil
}

// Validate performs validation on the item data request.
func (r *ItemDataRequest) Validate() error {
	count := 0
	if r.Password != nil {
		count++
	}
	if r.Text != nil {
		count++
	}
	if r.Binary != nil {
		count++
	}
	if r.Card != nil {
		count++
	}

	if count == 0 {
		return ErrMissingRequiredField
	}
	if count > 1 {
		return ErrMultipleDataTypes
	}

	return nil
}

// Validate performs validation on the password data request.
func (r *PasswordDataRequest) Validate() error {
	if r.Login == "" {
		return ErrLoginRequired
	}
	if r.Password == "" {
		return ErrPasswordRequired
	}
	return nil
}

// Validate performs validation on the text data request.
func (r *TextDataRequest) Validate() error {
	if r.Data == "" {
		return ErrDataRequired
	}
	return nil
}

// Validate performs validation on the binary data request.
func (r *BinaryDataRequest) Validate() error {
	if r.Filename == "" {
		return ErrFilenameRequired
	}
	if len(r.Data) == 0 {
		return ErrDataRequired
	}
	if len(r.Data) > MaxBinarySize {
		return ErrDataTooLarge
	}
	return nil
}

// Validate performs validation on the card data request.
func (r *CardDataRequest) Validate() error {
	if r.Number == "" || len(r.Number) != 16 {
		return ErrInvalidCardNumber
	}
	if r.HolderName == "" {
		return ErrHolderNameRequired
	}
	if r.CVV == "" || len(r.CVV) != 3 {
		return ErrInvalidCVV
	}
	return nil
}

// Validate performs validation on the sync request.
func (r *SyncRequest) Validate() error {
	for _, item := range r.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate performs validation on a sync item.
func (r *SyncItemRequest) Validate() error {
	if !isValidDataType(r.Type) {
		return ErrInvalidDataType
	}
	if r.Version < 0 {
		return ErrInvalidVersion
	}
	return r.Data.Validate()
}

// Validate performs validation on the conflict resolution request.
func (r *ResolveConflictRequest) Validate() error {
	if r.ItemID == "" {
		return ErrItemIDRequired
	}
	if r.Resolution == "" {
		return ErrResolutionRequired
	}

	if r.Resolution == ResolutionMerge && r.Data == nil {
		return ErrDataRequired
	}

	return nil
}

// Helper function to validate data types.
func isValidDataType(dt models.DataType) bool {
	switch dt {
	case models.DataTypePassword, models.DataTypeText, models.DataTypeBinary, models.DataTypeCard:
		return true
	default:
		return false
	}
}

// SetDefaults sets default values for request parameters.
func (r *GetItemsRequest) SetDefaults() {
	if r.Limit <= 0 {
		r.Limit = DefaultLimit
	}
	if r.Limit > MaxLimit {
		r.Limit = MaxLimit
	}
	if r.SortBy == "" {
		r.SortBy = "updated_at"
	}
	if r.SortOrder == "" {
		r.SortOrder = "desc"
	}
}
