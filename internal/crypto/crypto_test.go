// Package crypto provides tests for cryptographic operations.
package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEncryptor(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)
	assert.NotNil(t, encryptor)
	kd := encryptor.KeyDerivation()
	assert.NotNil(t, kd)
	assert.Equal(t, 3, kd.iterations)
	assert.Equal(t, 16, kd.saltLength)
	assert.Equal(t, 32, kd.keyLength)
}

func TestEncryptor_EncryptDecrypt(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)

	tests := []struct {
		name      string
		password  string
		plaintext []byte
	}{
		{
			name:      "simple text",
			password:  "test-password-123",
			plaintext: []byte("Hello, World!"),
		},
		{
			name:     "long text",
			password: "very-long-password-with-special-chars!@#$%",
			plaintext: []byte("This is a very long text that should be encrypted and decrypted correctly. " +
				"It contains multiple sentences and various characters: 1234567890 !@#$%^&*()"),
		},
		{
			name:      "binary data",
			password:  "binary-password",
			plaintext: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x10, 0x20, 0x30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := encryptor.Encrypt(tt.plaintext, tt.password)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			assert.NotEqual(t, string(tt.plaintext), ciphertext)

			// Decrypt
			decrypted, err := encryptor.Decrypt(ciphertext, tt.password)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptor_EncryptDecryptString(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)

	tests := []struct {
		name      string
		password  string
		plaintext string
	}{
		{
			name:      "simple string",
			password:  "test-password",
			plaintext: "Hello, World!",
		},
		{
			name:      "unicode string",
			password:  "password",
			plaintext: "Привет, мир! 你好世界! 🌍",
		},
		{
			name:      "special characters",
			password:  "secure",
			plaintext: "Special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt string
			ciphertext, err := encryptor.EncryptString(tt.plaintext, tt.password)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)
			assert.NotEqual(t, tt.plaintext, ciphertext)

			// Decrypt string
			decrypted, err := encryptor.DecryptString(ciphertext, tt.password)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestEncryptor_Errors(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)

	t.Run("empty password on encrypt", func(t *testing.T) {
		ciphertext, err := encryptor.Encrypt([]byte("plaintext"), "")
		assert.Error(t, err)
		assert.Empty(t, ciphertext)
		assert.Equal(t, ErrEmptyPassword, err)
	})

	t.Run("empty password on decrypt", func(t *testing.T) {
		decrypted, err := encryptor.Decrypt("valid-ciphertext", "")
		assert.Error(t, err)
		assert.Nil(t, decrypted)
		assert.Equal(t, ErrEmptyPassword, err)
	})

	t.Run("empty plaintext", func(t *testing.T) {
		ciphertext, err := encryptor.Encrypt([]byte(""), "password")
		assert.Error(t, err)
		assert.Empty(t, ciphertext)
		assert.Equal(t, ErrEmptyPlaintext, err)
	})

	t.Run("empty ciphertext", func(t *testing.T) {
		decrypted, err := encryptor.Decrypt("", "password")
		assert.Error(t, err)
		assert.Nil(t, decrypted)
		assert.Equal(t, ErrEmptyCiphertext, err)
	})

	t.Run("invalid base64", func(t *testing.T) {
		decrypted, err := encryptor.Decrypt("not-valid-base64!@#", "password")
		assert.Error(t, err)
		assert.Nil(t, decrypted)
	})

	t.Run("invalid ciphertext length", func(t *testing.T) {
		decrypted, err := encryptor.Decrypt("dG9vXNob3J0", "password")
		assert.Error(t, err)
		assert.Nil(t, decrypted)
	})

	t.Run("wrong password", func(t *testing.T) {
		plaintext := []byte("secret message")
		password := "correct-password"
		wrongPassword := "wrong-password"

		ciphertext, err := encryptor.Encrypt(plaintext, password)
		require.NoError(t, err)

		// Try to decrypt with wrong password
		decrypted, err := encryptor.Decrypt(ciphertext, wrongPassword)
		assert.Error(t, err)
		assert.Nil(t, decrypted)
	})
}

func TestEncryptor_DeterministicWithSameInput(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)

	password := "test-password"
	plaintext := []byte("Hello, World!")

	// Encrypt same data twice
	ciphertext1, err1 := encryptor.Encrypt(plaintext, password)
	ciphertext2, err2 := encryptor.Encrypt(plaintext, password)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEmpty(t, ciphertext1)
	assert.NotEmpty(t, ciphertext2)

	// Ciphertexts should be different (due to random nonce/salt)
	assert.NotEqual(t, ciphertext1, ciphertext2)

	// But both should decrypt to same plaintext
	decrypted1, err1 := encryptor.Decrypt(ciphertext1, password)
	decrypted2, err2 := encryptor.Decrypt(ciphertext2, password)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, plaintext, decrypted1)
	assert.Equal(t, plaintext, decrypted2)
}

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cost     int
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "secure-password-123",
			cost:     10,
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			cost:     10,
			wantErr:  true,
		},
		{
			name:     "invalid cost - too low",
			password: "password",
			cost:     3,
			wantErr:  true,
		},
		{
			name:     "invalid cost - too high",
			password: "password",
			cost:     50,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashed, err := HashPassword(tt.password, tt.cost)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, hashed)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, hashed)
				assert.NotEqual(t, tt.password, hashed)
				assert.Contains(t, hashed, "$2")
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cost     int
	}{
		{
			name:     "simple password",
			password: "password123",
			cost:     10,
		},
		{
			name:     "complex password",
			password: "P@ssw0rd!#$%^&*()",
			cost:     12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Hash password
			hashed, err := HashPassword(tt.password, tt.cost)
			require.NoError(t, err)

			// Verify with correct password
			err = VerifyPassword(hashed, tt.password)
			assert.NoError(t, err)

			// Verify with wrong password
			err = VerifyPassword(hashed, "wrong-password")
			assert.Error(t, err)
			assert.Equal(t, ErrPasswordMismatch, err)
		})
	}
}

func TestVerifyPassword_Errors(t *testing.T) {
	t.Run("empty hash", func(t *testing.T) {
		err := VerifyPassword("", "password")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidInput, err)
	})

	t.Run("empty password", func(t *testing.T) {
		hashed, _ := HashPassword("password", 10)
		err := VerifyPassword(hashed, "")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidInput, err)
	})
}

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{
			name:    "16 bytes",
			length:  16,
			wantErr: false,
		},
		{
			name:    "32 bytes",
			length:  32,
			wantErr: false,
		},
		{
			name:    "64 bytes",
			length:  64,
			wantErr: false,
		},
		{
			name:    "zero length",
			length:  0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := GenerateRandomBytes(tt.length)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, bytes)
			} else {
				require.NoError(t, err)
				assert.Len(t, bytes, tt.length)
			}
		})
	}
}

func TestGenerateRandomBytes_Uniqueness(t *testing.T) {
	length := 32

	// Generate two random byte slices
	bytes1, err1 := GenerateRandomBytes(length)
	bytes2, err2 := GenerateRandomBytes(length)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Len(t, bytes1, length)
	assert.Len(t, bytes2, length)

	// They should be different
	assert.NotEqual(t, bytes1, bytes2)
}

func TestHashData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "simple data",
			data: []byte("Hello, World!"),
		},
		{
			name: "empty data",
			data: []byte(""),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashData(tt.data)
			assert.Len(t, hash, 32) // SHA-256 produces 32 bytes
		})
	}
}

func TestHashData_Deterministic(t *testing.T) {
	data := []byte("consistent data")

	// Hash same data twice
	hash1 := HashData(data)
	hash2 := HashData(data)

	// Should be same
	assert.Equal(t, hash1, hash2)
}

func TestHashString(t *testing.T) {
	tests := []struct {
		name string
		s    string
	}{
		{
			name: "simple string",
			s:    "Hello, World!",
		},
		{
			name: "empty string",
			s:    "",
		},
		{
			name: "unicode string",
			s:    "Привет, мир! 🌍",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashString(tt.s)
			assert.Len(t, hash, 32) // SHA-256 produces 32 bytes
		})
	}
}

func TestKeyDerivation(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)

	password := "test-password"
	salt, err := GenerateSalt(16)
	require.NoError(t, err)
	assert.Len(t, salt, 16)

	// Derive key twice with same inputs
	key1 := encryptor.keyDerivation.DeriveKey(password, salt)
	key2 := encryptor.keyDerivation.DeriveKey(password, salt)

	// Should be same (deterministic)
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 32)

	// Different salt should produce different key
	salt2, _ := GenerateSalt(16)
	key3 := encryptor.KeyDerivation().DeriveKey(password, salt2)

	assert.NotEqual(t, key1, key3)
}

func TestKeyDerivation_DefaultParameters(t *testing.T) {
	encryptor := NewEncryptor(3, 16, 32)
	kd := encryptor.KeyDerivation()

	assert.Equal(t, 3, kd.iterations)
	assert.Equal(t, 16, kd.saltLength)
	assert.Equal(t, 32, kd.keyLength)
}
