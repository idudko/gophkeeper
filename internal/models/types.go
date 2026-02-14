// Package models provides data models for the GophKeeper password manager system.
package models

import "time"

// DataType represents the type of stored data.
type DataType string

const (
	// DataTypePassword represents login/password pairs.
	DataTypePassword DataType = "password"
	// DataTypeText represents arbitrary text data.
	DataTypeText DataType = "text"
	// DataTypeBinary represents arbitrary binary data.
	DataTypeBinary DataType = "binary"
	// DataTypeCard represents bank card data.
	DataTypeCard DataType = "card"
)

// User represents a user in the system.
type User struct {
	ID             string    `json:"id" db:"id"`
	Email          string    `json:"email" db:"email"`
	Password       string    `json:"-" db:"password"`        // Never include in JSON responses
	EncryptionSalt string    `json:"-" db:"encryption_salt"` // Salt for key derivation - never expose to clients
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// PasswordData represents a login/password pair.
type PasswordData struct {
	Meta     string `json:"meta" db:"meta"`         // Optional metadata (e.g., website name)
	Login    string `json:"login" db:"login"`       // Username or email
	Password string `json:"password" db:"password"` // Encrypted password
}

// TextData represents arbitrary text data.
type TextData struct {
	Meta string `json:"meta" db:"meta"` // Optional metadata
	Data string `json:"data" db:"data"` // Encrypted text content
}

// BinaryData represents arbitrary binary data.
type BinaryData struct {
	Meta     string `json:"meta" db:"meta"` // Optional metadata (e.g., file name)
	Filename string `json:"filename" db:"filename"`
	Data     []byte `json:"data" db:"data"` // Encrypted binary content
}

// CardData represents bank card information.
type CardData struct {
	Meta       string `json:"meta" db:"meta"`               // Optional metadata (e.g., "Personal card")
	Number     string `json:"number" db:"number"`           // Encrypted card number
	HolderName string `json:"holder_name" db:"holder_name"` // Encrypted cardholder name
	ExpiryDate string `json:"expiry_date" db:"expiry_date"` // Encrypted expiry date (MM/YY)
	CVV        string `json:"cvv" db:"cvv"`                 // Encrypted CVV code
}

// Item represents a stored item with its data and metadata.
type Item struct {
	ID        string     `json:"id" db:"id"`
	UserID    string     `json:"user_id" db:"user_id"`
	Type      DataType   `json:"type" db:"type"`
	Version   int        `json:"version" db:"version"`     // For conflict resolution
	Meta      string     `json:"meta,omitempty" db:"meta"` // Optional metadata about the item
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`

	// Type-specific data (only one will be populated based on Type)
	Password *PasswordData `json:"password,omitempty" db:"-"`
	Text     *TextData     `json:"text,omitempty" db:"-"`
	Binary   *BinaryData   `json:"binary,omitempty" db:"-"`
	Card     *CardData     `json:"card,omitempty" db:"-"`
}

// SyncRequest represents a synchronization request from client to server.
type SyncRequest struct {
	Items []Item `json:"items"`
}

// SyncResponse represents the server's response to a sync request.
type SyncResponse struct {
	Items      []Item    `json:"items"`     // Items from server
	Conflicts  []Item    `json:"conflicts"` // Items with conflicts
	LastSyncAt time.Time `json:"last_sync_at"`
}

// RegisterRequest represents a user registration request.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterResponse represents the response to a registration request.
type RegisterResponse struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

// LoginRequest represents a user login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents the response to a login request.
type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// ErrorResponse represents an error response from the API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
