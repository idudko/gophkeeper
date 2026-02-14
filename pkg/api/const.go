// Package api provides API constants, routes, and endpoints for GophKeeper system.
package api

const (
	// API version
	APIVersion = "v1"
	BasePath   = "/api/" + APIVersion

	// Endpoint paths
	PathRegister = BasePath + "/register"
	PathLogin    = BasePath + "/login"
	PathItems    = BasePath + "/items"
	PathSync     = BasePath + "/sync"
)

const (
	// Content types
	ContentTypeJSON = "application/json"
)

const (
	// Validation constraints
	MinPasswordLength = 8
	MaxPasswordLength = 128
	MaxBinarySize     = 10 * 1024 * 1024 // 10MB
)

const (
	// Pagination
	DefaultLimit  = 100
	DefaultOffset = 0
	MaxLimit      = 1000
)

const (
	// HTTP methods
	MethodGET     = "GET"
	MethodPOST    = "POST"
	MethodPUT     = "PUT"
	MethodPATCH   = "PATCH"
	MethodDELETE  = "DELETE"
	MethodOPTIONS = "OPTIONS"
)

const (
	// Header names
	HeaderAuthorization = "Authorization"
	HeaderBearer        = "Bearer"
	HeaderContentType   = "Content-Type"
	HeaderAccept        = "Accept"
	HeaderUserAgent     = "User-Agent"
	HeaderXRequestID  = "X-Request-ID"
)

const (
	// Context keys
	ContextKeyRequestID = "requestID"
)
