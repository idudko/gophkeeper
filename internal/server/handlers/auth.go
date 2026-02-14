// Package handlers provides HTTP request handlers for GophKeeper server.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"encoding/hex"

	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/idudko/gophkeeper/internal/auth"
	"github.com/idudko/gophkeeper/internal/crypto"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/idudko/gophkeeper/pkg/server"
	"github.com/idudko/gophkeeper/pkg/storage"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	storage       storage.Storage
	authenticator *auth.Authenticator
	log           logger.Logger
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(
	storage storage.Storage,
	authenticator *auth.Authenticator,
	log logger.Logger,
) *AuthHandler {
	return &AuthHandler{
		storage:       storage,
		authenticator: authenticator,
		log:           log,
	}
}

// RegisterRequest represents a registration request.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8,max=128"`
}

// RefreshTokenRequest represents a token refresh request.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Register handles user registration.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Decode request
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.Email == "" {
		server.RespondError(w, http.StatusBadRequest, "email is required", "EMAIL_REQUIRED", log, r)
		return
	}
	if len(req.Password) < 8 {
		server.RespondError(w, http.StatusBadRequest, "password must be at least 8 characters", "PASSWORD_TOO_SHORT", log, r)
		return
	}

	log.Info("user registration attempt",
		logger.String("email", req.Email),
	)

	// Check if user already exists
	existingUser, err := h.storage.GetUserByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		server.RespondError(w, http.StatusConflict, "user already exists", "USER_EXISTS", log, r)
		return
	}

	// Hash password
	hashedPassword, err := crypto.HashPassword(req.Password, h.authenticator.PasswordCost)
	if err != nil {
		h.log.Error("failed to hash password",
			logger.Err(err),
			logger.String("email", req.Email),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to process request", "HASH_ERROR", log, r)
		return
	}

	// Generate unique encryption salt for this user
	saltBytes, err := crypto.GenerateRandomBytes(16) // 128-bit salt
	if err != nil {
		h.log.Error("failed to generate encryption salt",
			logger.Err(err),
			logger.String("email", req.Email),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to process request", "SALT_GENERATION_ERROR", log, r)
		return
	}
	encryptionSalt := hex.EncodeToString(saltBytes)

	// Create user with encryption salt
	user := &models.User{
		ID:             uuid.New().String(),
		Email:          req.Email,
		Password:       hashedPassword,
		EncryptionSalt: encryptionSalt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Store user with retry logic
	if err := h.createUserWithRetry(ctx, user, 3); err != nil {
		h.log.Error("failed to create user",
			logger.Err(err),
			logger.String("email", req.Email),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to create user", "CREATE_USER_ERROR", log, r)
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.authenticator.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate tokens",
			logger.Err(err),
			logger.String("user_id", user.ID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to generate tokens", "TOKEN_ERROR", log, r)
		return
	}

	log.Info("user registered successfully",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)

	// Respond with tokens and user info
	server.RespondJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data": map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_at":    time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			"user": map[string]any{
				"id":         user.ID,
				"email":      user.Email,
				"created_at": user.CreatedAt.UTC().Format(time.RFC3339),
			},
		},
	})
}

// Login handles user authentication.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Decode request
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.Email == "" || req.Password == "" {
		server.RespondError(w, http.StatusBadRequest, "email and password are required", "CREDENTIALS_REQUIRED", log, r)
		return
	}

	log.Info("user login attempt",
		logger.String("email", req.Email),
	)

	// Get user with retry logic
	user, err := h.getUserByEmailWithRetry(ctx, req.Email, 3)
	if err != nil {
		if err == models.ErrUserNotFound {
			server.RespondError(w, http.StatusUnauthorized, "invalid email or password", "INVALID_CREDENTIALS", log, r)
			return
		}
		h.log.Error("failed to get user",
			logger.Err(err),
			logger.String("email", req.Email),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to authenticate", "AUTH_ERROR", log, r)
		return
	}

	// Verify password
	if err := crypto.VerifyPassword(user.Password, req.Password); err != nil {
		server.RespondError(w, http.StatusUnauthorized, "invalid email or password", "INVALID_CREDENTIALS", log, r)
		return
	}

	// Generate tokens
	accessToken, refreshToken, err := h.authenticator.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		h.log.Error("failed to generate tokens",
			logger.Err(err),
			logger.String("user_id", user.ID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to generate tokens", "TOKEN_ERROR", log, r)
		return
	}

	log.Info("user logged in successfully",
		logger.String("user_id", user.ID),
		logger.String("email", user.Email),
	)

	// Respond with tokens and user info (including encryption salt for client-side key derivation)
	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"access_token":    accessToken,
			"refresh_token":   refreshToken,
			"expires_at":      time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			"encryption_salt": user.EncryptionSalt,
			"user": map[string]any{
				"id":         user.ID,
				"email":      user.Email,
				"created_at": user.CreatedAt.UTC().Format(time.RFC3339),
			},
		},
	})
}

// RefreshToken handles access token refresh.
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Decode request
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.RefreshToken == "" {
		server.RespondError(w, http.StatusBadRequest, "refresh token is required", "TOKEN_REQUIRED", log, r)
		return
	}

	log.Info("token refresh attempt")

	// Validate refresh token
	claims, err := h.authenticator.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		code := "INVALID_TOKEN"
		if err == auth.ErrTokenExpired {
			code = "TOKEN_EXPIRED"
		}
		server.RespondError(w, http.StatusUnauthorized, err.Error(), code, log, r)
		return
	}

	// Generate new access token with retry logic
	accessToken, err := h.generateAccessTokenWithRetry(claims.UserID, claims.Email, 2)
	if err != nil {
		h.log.Error("failed to generate new access token",
			logger.Err(err),
			logger.String("user_id", claims.UserID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to refresh token", "REFRESH_ERROR", log, r)
		return
	}

	log.Info("token refreshed successfully",
		logger.String("user_id", claims.UserID),
	)

	// Respond with new access token
	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"access_token":  accessToken,
			"refresh_token": req.RefreshToken, // Keep the same refresh token
			"expires_at":    time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		},
	})
}

// ChangePassword handles password change requests.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context (set by auth middleware)
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Decode request
	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.CurrentPassword == "" || req.NewPassword == "" {
		server.RespondError(w, http.StatusBadRequest, "current and new passwords are required", "PASSWORDS_REQUIRED", log, r)
		return
	}
	if len(req.NewPassword) < 8 {
		server.RespondError(w, http.StatusBadRequest, "new password must be at least 8 characters", "PASSWORD_TOO_SHORT", log, r)
		return
	}

	log.Info("password change attempt",
		logger.String("user_id", userID),
	)

	// Get user with retry logic
	user, err := h.getUserByIDWithRetry(ctx, userID, 3)
	if err != nil {
		log.Error("failed to get user",
			logger.Err(err),
			logger.String("user_id", userID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to get user", "GET_USER_ERROR", log, r)
		return
	}

	// Verify current password
	if err := crypto.VerifyPassword(user.Password, req.CurrentPassword); err != nil {
		server.RespondError(w, http.StatusUnauthorized, "current password is incorrect", "INVALID_PASSWORD", log, r)
		return
	}

	// Hash new password
	hashedPassword, err := crypto.HashPassword(req.NewPassword, h.authenticator.PasswordCost)
	if err != nil {
		log.Error("failed to hash password",
			logger.Err(err),
			logger.String("user_id", user.ID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to process request", "HASH_ERROR", log, r)
		return
	}

	// Update user password with retry logic
	user.Password = hashedPassword
	if err := h.updateUserWithRetry(ctx, user, 3); err != nil {
		log.Error("failed to update user password",
			logger.Err(err),
			logger.String("user_id", user.ID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to update password", "UPDATE_ERROR", log, r)
		return
	}

	log.Info("password changed successfully",
		logger.String("user_id", user.ID),
	)

	// Respond with success
	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Retry helper methods

// getUserByEmailWithRetry retrieves user by email with retry logic.
func (h *AuthHandler) getUserByEmailWithRetry(ctx context.Context, email string, maxRetries int) (*models.User, error) {
	var user *models.User
	var err error

	for i := range maxRetries {
		user, err = h.storage.GetUserByEmail(ctx, email)
		if err == nil {
			return user, nil
		}
		if err == models.ErrUserNotFound {
			return nil, err
		}
		// Retry on connection errors
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return nil, err
}

// getUserByIDWithRetry retrieves user by ID with retry logic.
func (h *AuthHandler) getUserByIDWithRetry(ctx context.Context, id string, maxRetries int) (*models.User, error) {
	var user *models.User
	var err error

	for i := range maxRetries {
		user, err = h.storage.GetUserByID(ctx, id)
		if err == nil {
			return user, nil
		}
		if err == models.ErrUserNotFound {
			return nil, err
		}
		// Retry on connection errors
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return nil, err
}

func (h *AuthHandler) createUserWithRetry(ctx context.Context, user *models.User, maxRetries int) error {
	var err error

	for i := range maxRetries {
		err = h.storage.CreateUser(ctx, user)
		if err == nil {
			return nil
		}
		if err == models.ErrUserAlreadyExists {
			return err
		}
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}
	return err
}

func (h *AuthHandler) updateUserWithRetry(ctx context.Context, user *models.User, maxRetries int) error {
	var err error

	for i := range maxRetries {
		err = h.storage.UpdateUser(ctx, user)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}
	return err
}

func (h *AuthHandler) generateAccessTokenWithRetry(userID, email string, maxRetries int) (string, error) {
	var token string
	var err error

	for i := range maxRetries {
		token, err = h.authenticator.GenerateAccessToken(userID, email)
		if err == nil {
			return token, nil
		}
		time.Sleep(time.Duration(i+1) * 50 * time.Millisecond)
	}
	return "", err
}

// HTTP Client with retry support for external calls
func newRetryableHTTPClient(log logger.Logger) *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 5 * time.Second
	client.Logger = nil // Set to log to enable retry logging

	// Custom check retry
	client.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		// Retry on network errors
		if err != nil {
			return true, nil
		}
		// Retry on 5xx errors
		if resp.StatusCode >= 500 {
			return true, nil
		}
		// Retry on 429 (Too Many Requests)
		if resp.StatusCode == http.StatusTooManyRequests {
			return true, nil
		}
		return false, nil
	}

	return client
}
