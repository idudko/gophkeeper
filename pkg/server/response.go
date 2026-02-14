// Package server provides common HTTP response utilities for GophKeeper server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
)

// RespondJSON sends a JSON response with proper headers.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(api.HeaderContentType, api.ContentTypeJSON)
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(data); err != nil {
		// Log the internal error for debugging
		logger.Error("failed to encode JSON response",
			logger.Err(err),
		)
		// Send generic error message to client (no internal details exposed)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RespondError sends a JSON error response with logging.
func RespondError(w http.ResponseWriter, status int, message, code string, log logger.Logger, r *http.Request) {
	log.Error("request error",
		logger.String("code", code),
		logger.String("message", message),
		logger.Method(r.Method),
		logger.Path(r.URL.Path),
	)

	w.Header().Set(api.HeaderContentType, api.ContentTypeJSON)
	w.WriteHeader(status)

	response := map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"path":      r.URL.Path,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error("failed to encode error response",
			logger.Err(err),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RespondSuccess sends a success response with optional data.
func RespondSuccess(w http.ResponseWriter, message string, data any) {
	response := map[string]any{
		"success": true,
		"data": map[string]any{
			"message": message,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if data != nil {
		response["data"].(map[string]any)["data"] = data
	}

	RespondJSON(w, http.StatusOK, response)
}

// GetRequestID extracts request ID from headers or context.
func GetRequestID(r *http.Request) string {
	if rid := r.Header.Get(api.HeaderXRequestID); rid != "" {
		return rid
	}
	if rid := r.Context().Value(api.ContextKeyRequestID); rid != nil {
		if str, ok := rid.(string); ok {
			return str
		}
	}
	return ""
}

// GetUserIDFromContext extracts user ID from request context.
func GetUserIDFromContext(ctx context.Context) string {
	if uid := ctx.Value("userID"); uid != nil {
		if str, ok := uid.(string); ok {
			return str
		}
	}
	return ""
}

// GetUserEmailFromContext extracts user email from request context.
func GetUserEmailFromContext(ctx context.Context) string {
	if email := ctx.Value("email"); email != nil {
		if str, ok := email.(string); ok {
			return str
		}
	}
	return ""
}

// GetTokenFromContext extracts JWT token from request context.
func GetTokenFromContext(ctx context.Context) string {
	if token := ctx.Value("token"); token != nil {
		if str, ok := token.(string); ok {
			return str
		}
	}
	return ""
}

// ParseIntQuery parses an integer query parameter with validation.
func ParseIntQuery(s string, defaultValue, min, max int) int {
	if s == "" {
		return defaultValue
	}

	var value int
	n, err := fmt.Sscanf(s, "%d", &value)
	if n == 0 || err != nil {
		return defaultValue
	}

	if min > 0 && value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

// ParseBoolQuery parses a boolean query parameter.
func ParseBoolQuery(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	return s == "true" || s == "1" || s == "yes"
}

// writeJSONResponse writes a JSON response with proper headers and error handling.
func writeJSONResponse(w http.ResponseWriter, status int, data any) error {
	w.Header().Set(api.HeaderContentType, api.ContentTypeJSON)
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes an error response with proper headers.
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) error {
	response := map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return writeJSONResponse(w, status, response)
}

// ResponseBuilder provides a fluent interface for building responses.
type ResponseBuilder struct {
	writer  http.ResponseWriter
	logger  logger.Logger
	request *http.Request
	status  int
	success bool
	data    any
	error   *ErrorResponseData
	meta    *ResponseMeta
}

// ErrorResponseData contains error response data.
type ErrorResponseData struct {
	Code    string
	Message string
	Details string
	Field   string
}

// ResponseMeta contains response metadata.
type ResponseMeta struct {
	RequestID string
	Version   string
	Limit     int
	Offset    int
	Total     int
	HasMore   bool
	Timestamp string
}

// NewResponseBuilder creates a new response builder.
func NewResponseBuilder(w http.ResponseWriter, log logger.Logger, r *http.Request) *ResponseBuilder {
	return &ResponseBuilder{
		writer:  w,
		logger:  log,
		request: r,
		status:  http.StatusOK,
		success: true,
		meta: &ResponseMeta{
			RequestID: GetRequestID(r),
			Version:   api.APIVersion,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// SetStatus sets the HTTP status code.
func (rb *ResponseBuilder) SetStatus(status int) *ResponseBuilder {
	rb.status = status
	return rb
}

// SetSuccess sets whether to response indicates success.
func (rb *ResponseBuilder) SetSuccess(success bool) *ResponseBuilder {
	rb.success = success
	return rb
}

// SetData sets the response data.
func (rb *ResponseBuilder) SetData(data any) *ResponseBuilder {
	rb.data = data
	return rb
}

// SetError sets error information.
func (rb *ResponseBuilder) SetError(code, message string) *ResponseBuilder {
	return rb.SetErrorWithDetails(code, message, "")
}

// SetErrorWithDetails sets error information with details.
func (rb *ResponseBuilder) SetErrorWithDetails(code, message, details string) *ResponseBuilder {
	rb.success = false
	rb.error = &ErrorResponseData{
		Code:    code,
		Message: message,
		Details: details,
	}
	return rb
}

// SetErrorField sets error information with field name.
func (rb *ResponseBuilder) SetErrorField(code, message, field string) *ResponseBuilder {
	rb.success = false
	rb.error = &ErrorResponseData{
		Code:    code,
		Message: message,
		Field:   field,
	}
	return rb
}

// SetMeta sets the response metadata.
func (rb *ResponseBuilder) SetMeta(meta *ResponseMeta) *ResponseBuilder {
	rb.meta = meta
	return rb
}

// SetPagination sets pagination metadata.
func (rb *ResponseBuilder) SetPagination(limit, offset, total int) *ResponseBuilder {
	rb.meta.Limit = limit
	rb.meta.Offset = offset
	rb.meta.Total = total
	rb.meta.HasMore = offset+limit < total
	return rb
}

// Build builds and sends the response.
func (rb *ResponseBuilder) Build() {
	response := map[string]any{
		"success": rb.success,
	}

	if rb.success {
		if rb.data != nil {
			response["data"] = rb.data
		}
	} else if rb.error != nil {
		response["error"] = map[string]any{
			"code":    rb.error.Code,
			"message": rb.error.Message,
		}
		if rb.error.Details != "" {
			response["error"].(map[string]any)["details"] = rb.error.Details
		}
		if rb.error.Field != "" {
			response["error"].(map[string]any)["field"] = rb.error.Field
		}
	}

	if rb.meta != nil {
		response["meta"] = rb.meta
	} else {
		response["timestamp"] = time.Now().UTC().Format(time.RFC3339)
		if rb.request != nil {
			response["path"] = rb.request.URL.Path
		}
	}

	rb.writer.Header().Set(api.HeaderContentType, api.ContentTypeJSON)
	rb.writer.WriteHeader(rb.status)

	if err := json.NewEncoder(rb.writer).Encode(response); err != nil {
		rb.logger.Error("failed to encode response",
			logger.Err(err),
			logger.String("status", fmt.Sprintf("%d", rb.status)),
		)
		http.Error(rb.writer, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Log the response
	if rb.success {
		rb.logger.Info("response sent",
			logger.Int("status", rb.status),
			logger.Path(rb.request.URL.Path),
		)
	} else {
		rb.logger.Error("error response sent",
			logger.String("code", rb.error.Code),
			logger.String("message", rb.error.Message),
			logger.Int("status", rb.status),
			logger.Path(rb.request.URL.Path),
		)
	}
}
