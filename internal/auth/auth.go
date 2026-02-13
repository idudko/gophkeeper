// Package auth provides authentication and authorization functionality for the GophKeeper system,
// including JWT token generation, validation, and user authentication.
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/idudko/gophkeeper/internal/crypto"
)

// Claims represents the JWT claims structure.
type Claims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// Authenticator handles JWT token generation and validation.
type Authenticator struct {
	secret               []byte
	issuer               string
	audience             string
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
	PasswordCost         int
}

// NewAuthenticator creates a new JWT authenticator.
func NewAuthenticator(secret, issuer, audience string, accessDuration, refreshDuration time.Duration, passwordCost int) *Authenticator {
	return &Authenticator{
		secret:               []byte(secret),
		issuer:               issuer,
		audience:             audience,
		accessTokenDuration:  accessDuration * time.Second,
		refreshTokenDuration: refreshDuration * time.Second,
		PasswordCost:         passwordCost,
	}
}

// GenerateAccessToken generates a JWT access token for the given user.
func (a *Authenticator) GenerateAccessToken(userID, email string) (string, error) {
	now := time.Now()

	// Validate inputs
	if userID == "" {
		return "", ErrEmptyToken
	}
	if email == "" {
		return "", ErrEmptyToken
	}

	claims := &Claims{
		UserID:    userID,
		Email:     email,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   userID,
			Audience:  []string{a.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.accessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

func (a *Authenticator) GenerateRefreshToken(userID, email string) (string, error) {
	now := time.Now()

	// Validate inputs
	if userID == "" {
		return "", ErrEmptyToken
	}
	if email == "" {
		return "", ErrEmptyToken
	}

	claims := &Claims{
		UserID:    userID,
		Email:     email,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   userID,
			Audience:  []string{a.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.refreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secret)
}

// GenerateRefreshToken generates a JWT refresh token for the given user.

// GenerateTokenPair generates both access and refresh tokens.
func (a *Authenticator) GenerateTokenPair(userID, email string) (accessToken, refreshToken string, err error) {
	accessToken, err = a.GenerateAccessToken(userID, email)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = a.GenerateRefreshToken(userID, email)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (a *Authenticator) ValidateToken(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, ErrEmptyToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ValidateAccessToken validates an access token and returns the claims.
func (a *Authenticator) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "access" {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims.
func (a *Authenticator) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token using a refresh token.
func (a *Authenticator) RefreshAccessToken(refreshTokenString string) (newAccessToken string, err error) {
	claims, err := a.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	return a.GenerateAccessToken(claims.UserID, claims.Email)
}

// ExtractUserID extracts the user ID from a token without validating the signature.
// This is useful for logging purposes only, not for authentication.
func (a *Authenticator) ExtractUserID(tokenString string) (string, error) {
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", ErrInvalidClaims
	}

	return claims.UserID, nil
}

// UserInfo represents user information extracted from a token.
type UserInfo struct {
	UserID string
	Email  string
}

// GetUserInfo extracts user information from an access token.
func (a *Authenticator) GetUserInfo(accessToken string) (*UserInfo, error) {
	claims, err := a.ValidateAccessToken(accessToken)
	if err != nil {
		return nil, err
	}

	return &UserInfo{
		UserID: claims.UserID,
		Email:  claims.Email,
	}, nil
}

// TokenInfo provides metadata about a token.
type TokenInfo struct {
	UserID    string
	Email     string
	TokenType string
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// GetTokenInfo extracts detailed information about a token.
func (a *Authenticator) GetTokenInfo(tokenString string) (*TokenInfo, error) {
	claims, err := a.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	return &TokenInfo{
		UserID:    claims.UserID,
		Email:     claims.Email,
		TokenType: claims.TokenType,
		ExpiresAt: claims.ExpiresAt.Time,
		IssuedAt:  claims.IssuedAt.Time,
	}, nil
}

// HashToken hashes a token for storage in the database.
// Tokens should never be stored in plain text.
func (a *Authenticator) HashToken(token string) (string, error) {
	// Use SHA-256 for deterministic hashing
	// In production, you might want to use a different approach
	hash := HashSHA256(token)
	return EncodeBase64(hash), nil
}

// HashPassword hashes a password using bcrypt with the configured cost.
func (a *Authenticator) HashPassword(password string) (string, error) {
	return HashPasswordBcrypt(password, a.PasswordCost)
}

// VerifyPassword verifies that a password matches a bcrypt hash.
func (a *Authenticator) VerifyPassword(hashedPassword, password string) error {
	return VerifyPasswordBcrypt(hashedPassword, password)
}

// GetPasswordCost returns the bcrypt cost configured for password hashing.
func (a *Authenticator) GetPasswordCost() int {
	return a.PasswordCost
}

// HashPasswordBcrypt hashes a password using bcrypt with the specified cost.
func HashPasswordBcrypt(password string, cost int) (string, error) {
	if cost < 4 {
		cost = 4
	}
	if cost > 31 {
		cost = 31
	}

	// Use bcrypt package directly (imported separately)
	// This is a wrapper for compatibility with the crypto package
	return HashBcrypt(password, cost)
}

// VerifyPasswordBcrypt verifies that a password matches a bcrypt hash.
func VerifyPasswordBcrypt(hashedPassword, password string) error {
	return VerifyBcrypt(hashedPassword, password)
}

// HashSHA256 returns SHA-256 hash of input string.
func HashSHA256(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// EncodeBase64 encodes bytes to a base64 string.
func EncodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeBase64 decodes a base64 string to bytes.
func DecodeBase64(s string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, nil
	}
	return decoded, nil
}

// Errors
var (
	ErrEmptyToken       = errors.New("token is empty")
	ErrInvalidToken     = errors.New("token is invalid")
	ErrTokenExpired     = errors.New("token has expired")
	ErrInvalidClaims    = errors.New("token has invalid claims")
	ErrInvalidTokenType = errors.New("token has invalid type")
)

// The following are helper functions that should be imported from the crypto package
// They are defined here for compatibility and to avoid circular imports

// HashBcrypt hashes a password using bcrypt.
func HashBcrypt(password string, cost int) (string, error) {
	return crypto.HashPassword(password, cost)
}

// VerifyBcrypt verifies that a password matches a bcrypt hash.
func VerifyBcrypt(hashedPassword, password string) error {
	return crypto.VerifyPassword(hashedPassword, password)
}
