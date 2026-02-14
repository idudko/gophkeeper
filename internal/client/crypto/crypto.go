// Package crypto provides client-side encryption functionality for GophKeeper.
// This package implements zero-knowledge encryption where:
// - Encryption keys are derived from user's master password
// - Server stores only encrypted data without ability to decrypt
// - Master password never leaves the client
package clientcrypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// KeyManager manages encryption keys derived from master password.
type KeyManager struct {
	masterPassword string
	salt           string
	encryptionKey  []byte
}

// NewKeyManager creates a new key manager with master password and server-provided salt.
func NewKeyManager(masterPassword, salt string) *KeyManager {
	km := &KeyManager{
		masterPassword: masterPassword,
		salt:           salt,
	}
	km.deriveKey()
	return km
}

// deriveKey derives an encryption key from master password using Argon2id.
// Parameters chosen for good security/performance balance on various devices.
func (km *KeyManager) deriveKey() {
	// Decode hex salt from server
	saltBytes, err := base64.StdEncoding.DecodeString(km.salt)
	if err != nil {
		// If salt is not base64, try hex decoding or use raw
		saltBytes = []byte(km.salt)
	}

	// Argon2id parameters:
	// - time: 3 iterations (resistant to brute force)
	// - memory: 64 MB (resistant to GPU attacks)
	// - threads: 4 (utilizes available CPU cores)
	// - keyLength: 32 bytes (256 bits for ChaCha20-Poly1305)
	km.encryptionKey = argon2.IDKey(
		[]byte(km.masterPassword),
		saltBytes,
		3,
		64*1024,
		4,
		32,
	)
}

// GetEncryptionKey returns the derived encryption key.
// In production, this key should be kept in memory and never logged.
func (km *KeyManager) GetEncryptionKey() []byte {
	if km.encryptionKey == nil {
		km.deriveKey()
	}
	return km.encryptionKey
}

// Encrypt encrypts plaintext data using ChaCha20-Poly1305 AEAD.
// The encrypted data is returned as base64-encoded string containing:
// [nonce (12 bytes)] [ciphertext].
// Salt is NOT included as it's server-provided and used for key derivation.
func (km *KeyManager) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", fmt.Errorf("plaintext cannot be empty")
	}

	key := km.GetEncryptionKey()

	// Create AEAD cipher with XChaCha20-Poly1305 for better nonce reuse safety
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate a random nonce
	nonce, err := generateRandomBytes(aead.NonceSize())
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate the plaintext
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// Combine nonce + ciphertext
	combined := make([]byte, 0, len(nonce)+len(ciphertext))
	combined = append(combined, nonce...)
	combined = append(combined, ciphertext...)

	// Encode as base64
	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
// The ciphertext should be a base64-encoded string containing:
// [nonce (12 bytes)] [ciphertext].
func (km *KeyManager) Decrypt(ciphertext string) ([]byte, error) {
	if ciphertext == "" {
		return nil, fmt.Errorf("ciphertext cannot be empty")
	}

	key := km.GetEncryptionKey()

	// Decode base64
	combined, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	nonceSize := aead.NonceSize()

	// Check minimum length: nonce + at least 1 byte ciphertext
	if len(combined) < nonceSize+1 {
		return nil, fmt.Errorf("invalid ciphertext length")
	}

	// Extract nonce and ciphertext
	nonce := combined[:nonceSize]
	ciphertextBytes := combined[nonceSize:]

	// Decrypt and authenticate
	plaintext, err := aead.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password?): %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns the base64-encoded ciphertext.
func (km *KeyManager) EncryptString(plaintext string) (string, error) {
	return km.Encrypt([]byte(plaintext))
}

// DecryptString decrypts a base64-encoded ciphertext and returns the plaintext string.
func (km *KeyManager) DecryptString(ciphertext string) (string, error) {
	plaintext, err := km.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptItem encrypts all sensitive fields of an item based on its type.
func (km *KeyManager) EncryptItem(item *Item) error {
	switch item.Type {
	case DataTypePassword:
		if item.Password != nil {
			if item.Password.Login != "" {
				encrypted, err := km.EncryptString(item.Password.Login)
				if err != nil {
					return fmt.Errorf("failed to encrypt login: %w", err)
				}
				item.Password.Login = encrypted
			}
			if item.Password.Password != "" {
				encrypted, err := km.EncryptString(item.Password.Password)
				if err != nil {
					return fmt.Errorf("failed to encrypt password: %w", err)
				}
				item.Password.Password = encrypted
			}
		}
	case DataTypeText:
		if item.Text != nil {
			if item.Text.Data != "" {
				encrypted, err := km.EncryptString(item.Text.Data)
				if err != nil {
					return fmt.Errorf("failed to encrypt text data: %w", err)
				}
				item.Text.Data = encrypted
			}
		}
	case DataTypeBinary:
		if item.Binary != nil {
			if len(item.Binary.Data) > 0 {
				encrypted, err := km.Encrypt(item.Binary.Data)
				if err != nil {
					return fmt.Errorf("failed to encrypt binary data: %w", err)
				}
				item.Binary.Data = []byte(encrypted)
			}
		}
	case DataTypeCard:
		if item.Card != nil {
			if item.Card.Number != "" {
				encrypted, err := km.EncryptString(item.Card.Number)
				if err != nil {
					return fmt.Errorf("failed to encrypt card number: %w", err)
				}
				item.Card.Number = encrypted
			}
			if item.Card.HolderName != "" {
				encrypted, err := km.EncryptString(item.Card.HolderName)
				if err != nil {
					return fmt.Errorf("failed to encrypt card holder: %w", err)
				}
				item.Card.HolderName = encrypted
			}
			if item.Card.ExpiryDate != "" {
				encrypted, err := km.EncryptString(item.Card.ExpiryDate)
				if err != nil {
					return fmt.Errorf("failed to encrypt card expiry: %w", err)
				}
				item.Card.ExpiryDate = encrypted
			}
			if item.Card.CVV != "" {
				encrypted, err := km.EncryptString(item.Card.CVV)
				if err != nil {
					return fmt.Errorf("failed to encrypt card CVV: %w", err)
				}
				item.Card.CVV = encrypted
			}
		}
	}
	return nil
}

// DecryptItem decrypts all sensitive fields of an item based on its type.
func (km *KeyManager) DecryptItem(item *Item) error {
	switch item.Type {
	case DataTypePassword:
		if item.Password != nil {
			if item.Password.Login != "" {
				decrypted, err := km.DecryptString(item.Password.Login)
				if err != nil {
					return fmt.Errorf("failed to decrypt login: %w", err)
				}
				item.Password.Login = decrypted
			}
			if item.Password.Password != "" {
				decrypted, err := km.DecryptString(item.Password.Password)
				if err != nil {
					return fmt.Errorf("failed to decrypt password: %w", err)
				}
				item.Password.Password = decrypted
			}
		}
	case DataTypeText:
		if item.Text != nil {
			if item.Text.Data != "" {
				decrypted, err := km.DecryptString(item.Text.Data)
				if err != nil {
					return fmt.Errorf("failed to decrypt text data: %w", err)
				}
				item.Text.Data = decrypted
			}
		}
	case DataTypeBinary:
		if item.Binary != nil {
			if len(item.Binary.Data) > 0 {
				decrypted, err := km.Decrypt(string(item.Binary.Data))
				if err != nil {
					return fmt.Errorf("failed to decrypt binary data: %w", err)
				}
				item.Binary.Data = []byte(decrypted)
			}
		}
	case DataTypeCard:
		if item.Card != nil {
			if item.Card.Number != "" {
				decrypted, err := km.DecryptString(item.Card.Number)
				if err != nil {
					return fmt.Errorf("failed to decrypt card number: %w", err)
				}
				item.Card.Number = decrypted
			}
			if item.Card.HolderName != "" {
				decrypted, err := km.DecryptString(item.Card.HolderName)
				if err != nil {
					return fmt.Errorf("failed to decrypt card holder: %w", err)
				}
				item.Card.HolderName = decrypted
			}
			if item.Card.ExpiryDate != "" {
				decrypted, err := km.DecryptString(item.Card.ExpiryDate)
				if err != nil {
					return fmt.Errorf("failed to decrypt card expiry: %w", err)
				}
				item.Card.ExpiryDate = decrypted
			}
			if item.Card.CVV != "" {
				decrypted, err := km.DecryptString(item.Card.CVV)
				if err != nil {
					return fmt.Errorf("failed to decrypt card CVV: %w", err)
				}
				item.Card.CVV = decrypted
			}
		}
	}
	return nil
}

// Helper types to avoid circular dependency with models package

// DataType represents the type of stored data.
type DataType string

const (
	DataTypePassword DataType = "password"
	DataTypeText     DataType = "text"
	DataTypeBinary   DataType = "binary"
	DataTypeCard     DataType = "card"
)

// PasswordData represents a login/password pair.
type PasswordData struct {
	Meta     string `json:"meta"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

// TextData represents arbitrary text data.
type TextData struct {
	Meta string `json:"meta"`
	Data string `json:"data"`
}

// BinaryData represents arbitrary binary data.
type BinaryData struct {
	Meta     string `json:"meta"`
	Filename string `json:"filename"`
	Data     []byte `json:"data"`
}

// CardData represents bank card information.
type CardData struct {
	Meta       string `json:"meta"`
	Number     string `json:"number"`
	HolderName string `json:"holder_name"`
	ExpiryDate string `json:"expiry_date"`
	CVV        string `json:"cvv"`
}

// Item represents a stored item with its data and metadata.
type Item struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Type      DataType `json:"type"`
	Version   int      `json:"version"`
	Meta      string   `json:"meta,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	DeletedAt *string  `json:"deleted_at,omitempty"`

	Password *PasswordData `json:"password,omitempty"`
	Text     *TextData     `json:"text,omitempty"`
	Binary   *BinaryData   `json:"binary,omitempty"`
	Card     *CardData     `json:"card,omitempty"`
}

// generateRandomBytes generates cryptographically secure random bytes.
// This is a helper function that uses crypto/rand for security.
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}
