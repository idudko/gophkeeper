// Package middleware provides HTTP middleware for GophKeeper server.
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/idudko/gophkeeper/internal/auth"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/idudko/gophkeeper/pkg/server"
)

// Context key definitions
type contextKey string

const (
	userIDKey    contextKey = "userID"
	emailKey     contextKey = "email"
	tokenKey     contextKey = "token"
	tokenTypeKey contextKey = "tokenType"
)

// AuthMiddleware creates middleware that validates JWT access tokens and adds user info to context.
func AuthMiddleware(authenticator *auth.Authenticator, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get(api.HeaderAuthorization)
			if authHeader == "" {
				server.RespondError(w, http.StatusUnauthorized, "missing authorization header", "AUTH_HEADER_MISSING", log, r)
				return
			}

			// Check Bearer format
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				server.RespondError(w, http.StatusUnauthorized, "invalid authorization header format", "INVALID_AUTH_FORMAT", log, r)
				return
			}

			token := tokenParts[1]

			// Validate token
			claims, err := authenticator.ValidateAccessToken(token)
			if err != nil {
				code := "INVALID_TOKEN"
				if err == auth.ErrTokenExpired {
					code = "TOKEN_EXPIRED"
				}
				server.RespondError(w, http.StatusUnauthorized, err.Error(), code, log, r)
				return
			}

			// Add user info to request context
			ctx := r.Context()
			ctx = contextWithUserID(ctx, claims.UserID)
			ctx = contextWithEmail(ctx, claims.Email)
			ctx = contextWithToken(ctx, token)

			// Continue with modified context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware creates middleware that validates JWT tokens if present but allows unauthenticated requests.
func OptionalAuthMiddleware(authenticator *auth.Authenticator, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(api.HeaderAuthorization)
			if authHeader == "" {
				// No token provided, continue without user context
				next.ServeHTTP(w, r)
				return
			}

			// Check Bearer format
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := tokenParts[1]

			// Validate token (optional - if invalid, continue without user context)
			claims, err := authenticator.ValidateAccessToken(token)
			if err != nil {
				// Token invalid but continue processing
				next.ServeHTTP(w, r)
				return
			}

			// Add user info to request context
			ctx := r.Context()
			ctx = contextWithUserID(ctx, claims.UserID)
			ctx = contextWithEmail(ctx, claims.Email)
			ctx = contextWithToken(ctx, token)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RefreshTokenMiddleware creates middleware that validates refresh tokens.
func RefreshTokenMiddleware(authenticator *auth.Authenticator, log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get(api.HeaderAuthorization)
			if authHeader == "" {
				server.RespondError(w, http.StatusUnauthorized, "missing authorization header", "AUTH_HEADER_MISSING", log, r)
				return
			}

			// Check Bearer format
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
				server.RespondError(w, http.StatusUnauthorized, "invalid authorization header format", "INVALID_AUTH_FORMAT", log, r)
				return
			}

			token := tokenParts[1]

			// Validate refresh token
			claims, err := authenticator.ValidateRefreshToken(token)
			if err != nil {
				code := "INVALID_TOKEN"
				if err == auth.ErrTokenExpired {
					code = "REFRESH_TOKEN_EXPIRED"
				}
				server.RespondError(w, http.StatusUnauthorized, err.Error(), code, log, r)
				return
			}

			// Add user info to request context
			ctx := r.Context()
			ctx = contextWithUserID(ctx, claims.UserID)
			ctx = contextWithEmail(ctx, claims.Email)
			ctx = contextWithToken(ctx, token)
			ctx = contextWithTokenType(ctx, "refresh")

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoggingMiddleware creates a middleware that logs HTTP requests with structured logging.
func LoggingMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			requestID := middleware.GetReqID(r.Context())

			// Wrap response writer to capture status code
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			log.Info("request started",
				logger.RequestID(requestID),
				logger.Method(r.Method),
				logger.Path(r.URL.Path),
				logger.String("query", r.URL.RawQuery),
				logger.String("remote_addr", r.RemoteAddr),
				logger.String("user_agent", r.UserAgent()),
			)

			next.ServeHTTP(wrapped, r)

			log.Info("request completed",
				logger.RequestID(requestID),
				logger.Method(r.Method),
				logger.Path(r.URL.Path),
				logger.Int("status", wrapped.statusCode),
				logger.Dur("duration", time.Since(startTime)),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// RecoveryMiddleware creates middleware that recovers from panics.
func RecoveryMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("panic recovered",
						logger.Err(err.(error)),
						logger.Method(r.Method),
						logger.Path(r.URL.Path),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ContentTypeMiddleware enforces JSON content type for POST/PUT/PATCH requests.
func ContentTypeMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				contentType := r.Header.Get(api.HeaderContentType)
				if contentType != "" && contentType != api.ContentTypeJSON {
					server.RespondError(w, http.StatusUnsupportedMediaType,
						"unsupported content type, use application/json",
						"UNSUPPORTED_MEDIA_TYPE", log, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := middleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = r.Header.Get(api.HeaderXRequestID)
		}
		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set(api.HeaderXRequestID, requestID)

		ctx := context.WithValue(r.Context(), api.ContextKeyRequestID, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Context helper functions

func contextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func contextWithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey, email)
}

func contextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

func contextWithTokenType(ctx context.Context, tokenType string) context.Context {
	return context.WithValue(ctx, tokenTypeKey, tokenType)
}

// RequireUserID extracts and returns user ID from context, panicking if not present.
func RequireUserID(r *http.Request) string {
	userID, ok := r.Context().Value(userIDKey).(string)
	if !ok {
		panic("user ID not found in request context - Ensure AuthMiddleware is applied")
	}
	return userID
}

// UserID extracts user ID from context, returns empty string if not present.
func UserID(r *http.Request) string {
	userID, _ := r.Context().Value(userIDKey).(string)
	return userID
}

// UserEmail extracts user email from context.
func UserEmail(r *http.Request) string {
	email, _ := r.Context().Value(emailKey).(string)
	return email
}

// UserToken extracts JWT token from context.
func UserToken(r *http.Request) string {
	token, _ := r.Context().Value(tokenKey).(string)
	return token
}
