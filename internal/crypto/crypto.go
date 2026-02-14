// Package crypto provides cryptographic operations for the GophKeeper system including
// encryption, decryption, key derivation, and password hashing.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/chacha20poly1305"
)

// Encryptor provides encryption and decryption operations.
type Encryptor struct {
	keyDerivation *KeyDerivation
}

// NewEncryptor creates a new encryptor with the specified key derivation settings.
func NewEncryptor(iterations, saltLength, keyLength int) *Encryptor {
	return &Encryptor{
		keyDerivation: NewKeyDerivation(iterations, saltLength, keyLength),
	}
}

// Encrypt encrypts plaintext using ChaCha20-Poly1305 AEAD.
// The encrypted data is returned as base64-encoded string containing:
// [salt (16 bytes)] [nonce (12 bytes)] [ciphertext].
func (e *Encryptor) Encrypt(plaintext []byte, password string) (string, error) {
	if len(plaintext) == 0 {
		return "", ErrEmptyPlaintext
	}
	if password == "" {
		return "", ErrEmptyPassword
	}

	// Generate a random salt
	salt, err := GenerateRandomBytes(e.keyDerivation.saltLength)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key from password
	key := e.keyDerivation.DeriveKey(password, salt)

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate a random nonce
	nonce, err := GenerateRandomBytes(aead.NonceSize())
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate the plaintext
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// Combine salt + nonce + ciphertext
	combined := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	combined = append(combined, salt...)
	combined = append(combined, nonce...)
	combined = append(combined, ciphertext...)

	// Encode as base64
	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts ciphertext using ChaCha20-Poly1305 AEAD.
// The ciphertext should be a base64-encoded string containing:
// [salt (16 bytes)] [nonce (12 bytes)] [ciphertext].
func (e *Encryptor) Decrypt(ciphertext string, password string) ([]byte, error) {
	if ciphertext == "" {
		return nil, ErrEmptyCiphertext
	}
	if password == "" {
		return nil, ErrEmptyPassword
	}

	// Decode base64
	combined, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Check minimum length: salt + nonce + at least 1 byte ciphertext
	minLength := e.keyDerivation.saltLength + chacha20poly1305.NonceSizeX + 1
	if len(combined) < minLength {
		return nil, ErrInvalidCiphertextLength
	}

	// Extract salt, nonce, and ciphertext
	salt := combined[:e.keyDerivation.saltLength]
	nonce := combined[e.keyDerivation.saltLength : e.keyDerivation.saltLength+chacha20poly1305.NonceSizeX]
	ciphertextBytes := combined[e.keyDerivation.saltLength+chacha20poly1305.NonceSizeX:]

	// Derive encryption key from password
	key := e.keyDerivation.DeriveKey(password, salt)

	// Create AEAD cipher
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt and authenticate
	plaintext, err := aead.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns the base64-encoded ciphertext.
func (e *Encryptor) EncryptString(plaintext, password string) (string, error) {
	return e.Encrypt([]byte(plaintext), password)
}

// DecryptString decrypts a base64-encoded ciphertext and returns the plaintext string.
func (e *Encryptor) DecryptString(ciphertext, password string) (string, error) {
	plaintext, err := e.Decrypt(ciphertext, password)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// KeyDerivation returns the key derivation instance.
func (e *Encryptor) KeyDerivation() *KeyDerivation {
	return e.keyDerivation
}

// KeyDerivation handles password-based key derivation using Argon2id.
type KeyDerivation struct {
	iterations int
	saltLength int
	keyLength  int
	memory     uint32
	threads    uint8
}

// Iterations returns the number of iterations for key derivation.
func (kd *KeyDerivation) Iterations() int {
	return kd.iterations
}

// SaltLength returns the salt length in bytes.
func (kd *KeyDerivation) SaltLength() int {
	return kd.saltLength
}

// KeyLength returns the key length in bytes.
func (kd *KeyDerivation) KeyLength() int {
	return kd.keyLength
}

// NewKeyDerivation creates a new key derivation instance.
// Default parameters are set for security and performance balance.
func NewKeyDerivation(iterations, saltLength, keyLength int) *KeyDerivation {
	return &KeyDerivation{
		iterations: iterations,
		saltLength: saltLength,
		keyLength:  keyLength,
		memory:     64 * 1024, // 64 MB
		threads:    4,
	}
}

// DeriveKey derives a cryptographic key from a password using Argon2id.
// Argon2id is a hybrid of Argon2i and Argon2d, providing good resistance
// against both side-channel and GPU-based attacks.
func (kd *KeyDerivation) DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		uint32(kd.iterations),
		kd.memory,
		kd.threads,
		uint32(kd.keyLength),
	)
}

// HashPassword securely hashes a password using bcrypt.
// Bcrypt is specifically designed for password hashing and includes
// automatic salt generation and work factor (cost) to slow down brute force attacks.
func HashPassword(password string, cost int) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return "", ErrInvalidCost
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedPassword), nil
}

// VerifyPassword verifies that a password matches a bcrypt hash.
func VerifyPassword(hashedPassword, password string) error {
	if hashedPassword == "" || password == "" {
		return ErrInvalidInput
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		return ErrPasswordMismatch
	}

	return nil
}

// GenerateRandomBytes generates cryptographically secure random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// GenerateSalt generates a random salt for key derivation.
func GenerateSalt(length int) ([]byte, error) {
	return GenerateRandomBytes(length)
}

// GenerateNonce generates a random nonce for encryption.
func GenerateNonce(length int) ([]byte, error) {
	return GenerateRandomBytes(length)
}

// HashData generates a SHA-256 hash of the input data.
// Useful for generating unique identifiers and checksums.
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// HashString generates a SHA-256 hash of the input string.
func HashString(s string) []byte {
	return HashData([]byte(s))
}

// Errors
var (
	ErrEmptyPassword           = fmt.Errorf("password cannot be empty")
	ErrEmptyPlaintext          = fmt.Errorf("plaintext cannot be empty")
	ErrEmptyCiphertext         = fmt.Errorf("ciphertext cannot be empty")
	ErrInvalidCiphertextLength = fmt.Errorf("invalid ciphertext length")
	ErrPasswordMismatch        = fmt.Errorf("password does not match")
	ErrInvalidInput            = fmt.Errorf("invalid input")
	ErrInvalidCost             = fmt.Errorf("invalid bcrypt cost parameter")
)
