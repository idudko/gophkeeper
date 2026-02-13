// Package auth provides tests for authentication and authorization functionality.
package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret          = "test-secret-key-1234567890"
	testIssuer          = "gophkeeper-test"
	testAudience        = "gophkeeper-clients"
	accessDuration      = 15 * 60          // 15 minutes
	refreshDuration     = 7 * 24 * 60 * 60 // 7 days in seconds
	defaultPasswordCost = 10
	testUserID          = "test-user-id-12345"
	testEmail           = "test@example.com"
)

func TestNewAuthenticator(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	assert.NotNil(t, authenticator)
	assert.Equal(t, defaultPasswordCost, authenticator.PasswordCost)
}

func TestAuthenticator_GenerateAccessToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken("user-123", "test@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("empty user ID", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken("", "test@example.com")
		assert.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestAuthenticator_GenerateRefreshToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid token", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken("user-456", "test@example.com")
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("empty user ID", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken("", "test@example.com")
		assert.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestAuthenticator_ValidateAccessToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken("user-789", "test@example.com")
		require.NoError(t, err)

		claims, err := authenticator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user-789", claims.UserID)
		assert.Equal(t, "access", claims.TokenType)
	})

	t.Run("invalid token", func(t *testing.T) {
		claims, err := authenticator.ValidateAccessToken("invalid-token")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("empty token", func(t *testing.T) {
		claims, err := authenticator.ValidateAccessToken("")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestAuthenticator_ValidateRefreshToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid refresh token", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken("user-999", "test@example.com")
		require.NoError(t, err)

		claims, err := authenticator.ValidateRefreshToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user-999", claims.UserID)
		assert.Equal(t, "refresh", claims.TokenType)
	})

	t.Run("refresh token as access token", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken("user-888", "test@example.com")
		require.NoError(t, err)

		// Try to validate refresh token as access token
		claims, err := authenticator.ValidateAccessToken(token)
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

func TestAuthenticator_RefreshAccessToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid refresh token", func(t *testing.T) {
		refreshToken, err := authenticator.GenerateRefreshToken("user-666", "test@example.com")
		require.NoError(t, err)

		newAccessToken, err := authenticator.RefreshAccessToken(refreshToken)
		require.NoError(t, err)
		assert.NotEmpty(t, newAccessToken)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		newAccessToken, err := authenticator.RefreshAccessToken("invalid-token")
		assert.Error(t, err)
		assert.Empty(t, newAccessToken)
	})
}

func TestAuthenticator_TokenExpiry(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("access token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken("user-111", "test@example.com")
		require.NoError(t, err)

		claims, err := authenticator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.NotZero(t, claims.ExpiresAt)
	})
}

func TestAuthenticator_TokenClaims(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("access token claims", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken("user-333", "test@example.com")
		require.NoError(t, err)

		claims, err := authenticator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user-333", claims.UserID)
		assert.Equal(t, testIssuer, claims.Issuer)
		assert.Contains(t, claims.Audience, testAudience)
	})

	t.Run("refresh token claims", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken("user-444", "test@example.com")
		require.NoError(t, err)

		claims, err := authenticator.ValidateRefreshToken(token)
		require.NoError(t, err)
		assert.Equal(t, "user-444", claims.UserID)
		assert.Equal(t, testIssuer, claims.Issuer)
		assert.Contains(t, claims.Audience, testAudience)
	})
}

func TestAuthenticator_HashToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	token := "test-token-123456789"
	hash1, err := authenticator.HashToken(token)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)
	assert.NotEqual(t, token, hash1)

	// Hashing same token should produce same result
	hash2, err := authenticator.HashToken(token)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)
}

func TestAuthenticator_TokenFromDifferentSecret(t *testing.T) {
	auth1 := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	auth2 := NewAuthenticator(
		"different-secret",
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	token, err := auth1.GenerateAccessToken("user-555", "test@example.com")
	require.NoError(t, err)

	// Try to validate with different authenticator
	claims, err := auth2.ValidateAccessToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestHashSHA256(t *testing.T) {
	testString := "test-string-123"

	hash := HashSHA256(testString)
	assert.Len(t, hash, 32) // SHA-256 produces 32 bytes

	// Hashing same string should produce same result
	hash2 := HashSHA256(testString)
	assert.Equal(t, hash, hash2)
}

func TestEncodeBase64(t *testing.T) {
	testData := []byte("test-data-456")

	encoded := EncodeBase64(testData)
	assert.NotEmpty(t, encoded)
	assert.NotEqual(t, string(testData), encoded)

	decoded, err := DecodeBase64(encoded)
	require.NoError(t, err)
	assert.Equal(t, testData, decoded)
}

func TestDecodeBase64(t *testing.T) {
	t.Run("valid base64", func(t *testing.T) {
		testData := []byte("test-data-789")
		encoded := EncodeBase64(testData)

		decoded, err := DecodeBase64(encoded)
		require.NoError(t, err)
		assert.Equal(t, testData, decoded)
	})

	t.Run("invalid base64", func(t *testing.T) {
		decoded, err := DecodeBase64("not-valid-base64!@#")
		assert.Error(t, err)
		assert.Nil(t, decoded)
	})

	t.Run("empty string", func(t *testing.T) {
		decoded, err := DecodeBase64("")
		require.NoError(t, err)
		assert.Nil(t, decoded)
	})
}

func TestAuthenticator_ConcurrentTokenGeneration(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	// Generate tokens concurrently with different user IDs to ensure uniqueness
	results := make(chan string, 10)
	errors := make(chan error, 10)

	for i := range 10 {
		go func(index int) {
			userID := fmt.Sprintf("user-concurrent-%d", index) // <-- Объявляем внутри горутины
			token, err := authenticator.GenerateAccessToken(userID, "test@example.com")
			results <- token
			errors <- err
		}(i) // <-- Передаем i как параметр
	}

	// Collect results
	tokens := make([]string, 0, 10)
	for range 10 {
		token := <-results
		err := <-errors
		require.NoError(t, err)
		tokens = append(tokens, token)
	}

	// All tokens should be generated successfully
	assert.Len(t, tokens, 10)

	// All tokens should be unique (different user IDs)
	uniqueTokens := make(map[string]bool)
	for _, token := range tokens {
		assert.NotEmpty(t, token)
		assert.False(t, uniqueTokens[token], "Token should be unique")
		uniqueTokens[token] = true

		// Verify token is valid
		claims, err := authenticator.ValidateAccessToken(token)
		require.NoError(t, err)
		assert.NotEmpty(t, claims.UserID)
	}
}

func TestAuthenticator_GetPasswordCost(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	cost := authenticator.GetPasswordCost()
	assert.Equal(t, defaultPasswordCost, cost)
}

func TestAuthenticator_GetUserInfo(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid access token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken(testUserID, testEmail)
		require.NoError(t, err)

		userInfo, err := authenticator.GetUserInfo(token)
		require.NoError(t, err)
		assert.Equal(t, testUserID, userInfo.UserID)
		assert.Equal(t, testEmail, userInfo.Email)
	})

	t.Run("invalid token", func(t *testing.T) {
		userInfo, err := authenticator.GetUserInfo("invalid-token")
		assert.Error(t, err)
		assert.Nil(t, userInfo)
	})

	t.Run("empty token", func(t *testing.T) {
		userInfo, err := authenticator.GetUserInfo("")
		assert.Error(t, err)
		assert.Nil(t, userInfo)
	})

	t.Run("refresh token should fail", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken(testUserID, testEmail)
		require.NoError(t, err)

		userInfo, err := authenticator.GetUserInfo(token)
		assert.Error(t, err)
		assert.Nil(t, userInfo)
	})
}

func TestAuthenticator_GetTokenInfo(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("access token info", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken(testUserID, testEmail)
		require.NoError(t, err)

		tokenInfo, err := authenticator.GetTokenInfo(token)
		require.NoError(t, err)
		assert.Equal(t, testUserID, tokenInfo.UserID)
		assert.Equal(t, testEmail, tokenInfo.Email)
		assert.Equal(t, "access", tokenInfo.TokenType)
		assert.False(t, tokenInfo.ExpiresAt.IsZero())
		assert.False(t, tokenInfo.IssuedAt.IsZero())
		assert.True(t, tokenInfo.ExpiresAt.After(time.Now()))
	})

	t.Run("refresh token info", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken(testUserID, testEmail)
		require.NoError(t, err)

		tokenInfo, err := authenticator.GetTokenInfo(token)
		require.NoError(t, err)
		assert.Equal(t, testUserID, tokenInfo.UserID)
		assert.Equal(t, testEmail, tokenInfo.Email)
		assert.Equal(t, "refresh", tokenInfo.TokenType)
		assert.False(t, tokenInfo.ExpiresAt.IsZero())
		assert.False(t, tokenInfo.IssuedAt.IsZero())
		assert.True(t, tokenInfo.ExpiresAt.After(time.Now()))
	})

	t.Run("invalid token", func(t *testing.T) {
		tokenInfo, err := authenticator.GetTokenInfo("invalid-token")
		assert.Error(t, err)
		assert.Nil(t, tokenInfo)
	})

	t.Run("empty token", func(t *testing.T) {
		tokenInfo, err := authenticator.GetTokenInfo("")
		assert.Error(t, err)
		assert.Nil(t, tokenInfo)
	})
}

func TestAuthenticator_HashTokenWithRealToken(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("hash real generated token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken(testUserID, testEmail)
		require.NoError(t, err)

		hash, err := authenticator.HashToken(token)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Len(t, hash, 44)
	})

	t.Run("different tokens have different hashes", func(t *testing.T) {
		token1, _ := authenticator.GenerateAccessToken(testUserID, testEmail)
		token2, _ := authenticator.GenerateAccessToken("user2", "user2@example.com")

		hash1, err1 := authenticator.HashToken(token1)
		hash2, err2 := authenticator.HashToken(token2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestAuthenticator_GenerateTokenPair(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid token pair", func(t *testing.T) {
		accessToken, refreshToken, err := authenticator.GenerateTokenPair(testUserID, testEmail)
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
		assert.NotEqual(t, accessToken, refreshToken)

		// Validate access token
		accessClaims, err := authenticator.ValidateAccessToken(accessToken)
		require.NoError(t, err)
		assert.Equal(t, testUserID, accessClaims.UserID)
		assert.Equal(t, testEmail, accessClaims.Email)
		assert.Equal(t, "access", accessClaims.TokenType)

		// Validate refresh token
		refreshClaims, err := authenticator.ValidateRefreshToken(refreshToken)
		require.NoError(t, err)
		assert.Equal(t, testUserID, refreshClaims.UserID)
		assert.Equal(t, testEmail, refreshClaims.Email)
		assert.Equal(t, "refresh", refreshClaims.TokenType)
	})

	t.Run("empty user ID", func(t *testing.T) {
		accessToken, refreshToken, err := authenticator.GenerateTokenPair("", testEmail)
		assert.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("empty email", func(t *testing.T) {
		accessToken, refreshToken, err := authenticator.GenerateTokenPair(testUserID, "")
		assert.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})
}

func TestAuthenticator_ExtractUserID(t *testing.T) {
	authenticator := NewAuthenticator(
		testSecret,
		testIssuer,
		testAudience,
		accessDuration,
		refreshDuration,
		defaultPasswordCost,
	)

	t.Run("valid token", func(t *testing.T) {
		token, err := authenticator.GenerateAccessToken(testUserID, testEmail)
		require.NoError(t, err)

		userID, err := authenticator.ExtractUserID(token)
		require.NoError(t, err)
		assert.Equal(t, testUserID, userID)
	})

	t.Run("invalid token", func(t *testing.T) {
		userID, err := authenticator.ExtractUserID("invalid-token")
		assert.Error(t, err)
		assert.Empty(t, userID)
	})

	t.Run("empty token", func(t *testing.T) {
		userID, err := authenticator.ExtractUserID("")
		assert.Error(t, err)
		assert.Empty(t, userID)
	})

	t.Run("token from different secret", func(t *testing.T) {
		otherAuthenticator := NewAuthenticator(
			"different-secret",
			testIssuer,
			testAudience,
			accessDuration,
			refreshDuration,
			defaultPasswordCost,
		)

		token, err := otherAuthenticator.GenerateAccessToken("other-user", "REDACTED__N144__")
		require.NoError(t, err)

		// Can still extract user ID without signature validation
		userID, err := authenticator.ExtractUserID(token)
		require.NoError(t, err)
		assert.Equal(t, "other-user", userID)
	})

	t.Run("refresh token", func(t *testing.T) {
		token, err := authenticator.GenerateRefreshToken(testUserID, testEmail)
		require.NoError(t, err)

		userID, err := authenticator.ExtractUserID(token)
		require.NoError(t, err)
		assert.Equal(t, testUserID, userID)
	})
}
