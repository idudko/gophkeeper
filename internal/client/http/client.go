// Package http provides HTTP client with retry support for GophKeeper API communication.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/rs/zerolog"
)

// Client represents an HTTP client with retry capabilities for GophKeeper API.
type Client struct {
	httpClient    *retryablehttp.Client
	serverAddr    string
	accessToken   string
	userAgent     string
	logger        zerolog.Logger
	requestLogger zerolog.Logger
}

// Config holds configuration for the HTTP client.
type Config struct {
	ServerAddr    string
	AccessToken   string
	UserAgent     string
	Logger        zerolog.Logger
	RequestLogger zerolog.Logger

	// Retry configuration
	RetryMax       int
	RetryWaitMin   time.Duration
	RetryWaitMax   time.Duration
	RequestTimeout time.Duration

	// CheckRetry is called to determine whether a request should be retried.
	// If nil, the default CheckRetry function is used.
	CheckRetry retryablehttp.CheckRetry

	// Backoff specifies the policy for how long to wait between retries.
	// If nil, retryablehttp.DefaultBackoff is used.
	Backoff retryablehttp.Backoff
}

// NewClient creates a new HTTP client with retry support.
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Set defaults
	if cfg.UserAgent == "" {
		cfg.UserAgent = "GophKeeperClient/1.0.0"
	}
	if cfg.RetryMax == 0 {
		cfg.RetryMax = 3
	}
	if cfg.RetryWaitMin == 0 {
		cfg.RetryWaitMin = 500 * time.Millisecond
	}
	if cfg.RetryWaitMax == 0 {
		cfg.RetryWaitMax = 5 * time.Second
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}

	// Create retryable HTTP client
	retryableClient := retryablehttp.NewClient()
	retryableClient.RetryMax = cfg.RetryMax
	retryableClient.RetryWaitMin = cfg.RetryWaitMin
	retryableClient.RetryWaitMax = cfg.RetryWaitMax
	retryableClient.HTTPClient.Timeout = cfg.RequestTimeout

	// Set check retry function if provided
	if cfg.CheckRetry != nil {
		retryableClient.CheckRetry = cfg.CheckRetry
	} else {
		retryableClient.CheckRetry = DefaultCheckRetry
	}

	// Set backoff function if provided
	if cfg.Backoff != nil {
		retryableClient.Backoff = cfg.Backoff
	} else {
		retryableClient.Backoff = retryablehttp.DefaultBackoff
	}

	// Set error logger
	retryableClient.ErrorHandler = retryableErrorHandler(cfg.Logger)
	retryableClient.Logger = retryableLogger(cfg.Logger)

	client := &Client{
		httpClient:    retryableClient,
		serverAddr:    cfg.ServerAddr,
		accessToken:   cfg.AccessToken,
		userAgent:     cfg.UserAgent,
		logger:        cfg.Logger,
		requestLogger: cfg.RequestLogger,
	}

	return client, nil
}

// SetAccessToken updates the access token for authentication.
func (c *Client) SetAccessToken(token string) {
	c.accessToken = token
}

// GetServerAddr returns the configured server address.
func (c *Client) GetServerAddr() string {
	return c.serverAddr
}

// Do performs an HTTP request with retry support.
func (c *Client) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	// Build full URL
	fullURL, err := url.JoinPath(c.serverAddr, path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL path: %w", err)
	}

	// Create retryable request
	req, err := retryablehttp.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	c.setHeaders(req)

	// Log request if request logger is configured
	if c.requestLogger.GetLevel() <= zerolog.DebugLevel {
		c.logRequest(req, method, fullURL)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("method", method).
			Str("url", fullURL).
			Msg("HTTP request failed after retries")
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Log response if request logger is configured
	if c.requestLogger.GetLevel() <= zerolog.DebugLevel {
		c.logResponse(resp, method, fullURL)
	}

	return resp, nil
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, api.MethodGET, path, nil)
}

// Post performs a POST request with JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.DoJSON(ctx, api.MethodPOST, path, body)
}

// Put performs a PUT request with JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.DoJSON(ctx, api.MethodPUT, path, body)
}

// Patch performs a PATCH request with JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.DoJSON(ctx, api.MethodPATCH, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	return c.Do(ctx, api.MethodDELETE, path, nil)
}

// DoJSON performs an HTTP request with JSON body and retry support.
func (c *Client) DoJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	resp, err := c.Do(ctx, method, path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set Content-Type header explicitly for JSON requests
	if resp.Request != nil && resp.Request.Header.Get(api.HeaderContentType) == "" {
		resp.Request.Header.Set(api.HeaderContentType, api.ContentTypeJSON)
	}

	return resp, nil
}

// ParseJSONResponse performs a request and parses the JSON response.
func (c *Client) ParseJSONResponse(ctx context.Context, method, path string, body any, response any) error {
	var resp *http.Response
	var err error

	if body != nil {
		resp, err = c.DoJSON(ctx, method, path, body)
	} else {
		resp, err = c.Do(ctx, method, path, nil)
	}

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return ParseResponse(resp, response)
}

// ParseResponse parses an HTTP response into the provided interface.
func ParseResponse(resp *http.Response, v any) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("failed to unmarshal JSON response: %w", err)
		}
	}

	return nil
}

// setHeaders sets common headers for the request.
func (c *Client) setHeaders(req *retryablehttp.Request) {
	// User agent
	req.Header.Set(api.HeaderUserAgent, c.userAgent)

	// Authorization
	if c.accessToken != "" {
		req.Header.Set(api.HeaderAuthorization, fmt.Sprintf("%s %s", api.HeaderBearer, c.accessToken))
	}

	// Accept
	req.Header.Set(api.HeaderAccept, api.ContentTypeJSON)

	// Content-Type
	if req.Body != nil {
		req.Header.Set(api.HeaderContentType, api.ContentTypeJSON)
	}
}

// logRequest logs the request details.
func (c *Client) logRequest(req *retryablehttp.Request, method, url string) {
	c.requestLogger.Debug().
		Str("method", method).
		Str("url", url).
		Str("user_agent", req.Header.Get(api.HeaderUserAgent)).
		Str("authorization", maskToken(req.Header.Get(api.HeaderAuthorization))).
		Msg("Sending HTTP request")
}

// logResponse logs the response details.
func (c *Client) logResponse(resp *http.Response, method, url string) {
	c.requestLogger.Debug().
		Str("method", method).
		Str("url", url).
		Int("status", resp.StatusCode).
		Str("status_text", resp.Status).
		Int64("content_length", resp.ContentLength).
		Msg("Received HTTP response")
}

// DefaultCheckRetry provides a default retry policy.
func DefaultCheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Do not retry on context cancellation
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// Retry on network errors
	if err != nil {
		// Check if error is retryable
		if IsRetryableError(err) {
			return true, nil
		}
		return false, err
	}

	// Do not retry on success
	if resp == nil {
		return false, nil
	}

	// Retry on 5xx errors
	if resp.StatusCode >= 500 {
		return true, nil
	}

	// Retry on 429 (Too Many Requests)
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	// Retry on 408 (Request Timeout)
	if resp.StatusCode == http.StatusRequestTimeout {
		return true, nil
	}

	// Retry on connection errors
	if resp.StatusCode == 0 {
		return true, nil
	}

	// Do not retry on other status codes
	return false, nil
}

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common retryable error patterns
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no such host",
		"connection timed out",
		"i/o timeout",
		"EOF",
		"use of closed network connection",
		"client connection lost",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	return s == substr ||
		s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr
}

// maskToken masks a bearer token for logging.
func maskToken(authHeader string) string {
	if len(authHeader) <= len("Bearer ") {
		return "***"
	}

	prefix := authHeader[:7] // "Bearer "
	if prefix != "Bearer " {
		return "***"
	}

	token := authHeader[7:]
	if len(token) <= 8 {
		return prefix + "***"
	}

	return prefix + token[:4] + "****" + token[len(token)-4:]
}

// retryableErrorHandler handles errors from the retryablehttp client.
func retryableErrorHandler(logger zerolog.Logger) retryablehttp.ErrorHandler {
	return func(resp *http.Response, err error, numTries int) (*http.Response, error) {
		if err != nil {
			logger.Warn().
				Err(err).
				Int("attempt", numTries).
				Msg("Request failed, will retry")
		} else if resp.StatusCode >= 500 {
			logger.Warn().
				Int("status", resp.StatusCode).
				Str("status_text", resp.Status).
				Int("attempt", numTries).
				Msg("Received server error, will retry")
		}
		return resp, err
	}
}

// retryableLogger adapts zerolog.Logger to retryablehttp.Logger interface.
func retryableLogger(logger zerolog.Logger) retryablehttp.LeveledLogger {
	return &retryableLoggerAdapter{
		logger: logger,
	}
}

// retryableLoggerAdapter adapts zerolog to retryablehttp.LeveledLogger interface.
type retryableLoggerAdapter struct {
	logger zerolog.Logger
}

// Error logs at error level.
func (a *retryableLoggerAdapter) Error(msg string, keysAndValues ...any) {
	a.logger.Error().Fields(keysAndValuesToMap(keysAndValues)).Msg(msg)
}

// Info logs at info level.
func (a *retryableLoggerAdapter) Info(msg string, keysAndValues ...any) {
	a.logger.Info().Fields(keysAndValuesToMap(keysAndValues)).Msg(msg)
}

// Debug logs at debug level.
func (a *retryableLoggerAdapter) Debug(msg string, keysAndValues ...any) {
	a.logger.Debug().Fields(keysAndValuesToMap(keysAndValues)).Msg(msg)
}

// Warn logs at warn level.
func (a *retryableLoggerAdapter) Warn(msg string, keysAndValues ...any) {
	a.logger.Warn().Fields(keysAndValuesToMap(keysAndValues)).Msg(msg)
}

// keysAndValuesToMap converts alternating keys and values to a map.
func keysAndValuesToMap(kv []any) map[string]any {
	m := make(map[string]any)
	for i := 0; i < len(kv); i += 2 {
		if i+1 < len(kv) {
			if key, ok := kv[i].(string); ok {
				m[key] = kv[i+1]
			}
		}
	}
	return m
}

// ErrorResponse represents a standard error response from the server.
type ErrorResponse struct {
	Success bool `json:"success"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
	Path      string `json:"path,omitempty"`
}

// ParseErrorResponse parses an error response from the server.
func ParseErrorResponse(resp *http.Response) (*ErrorResponse, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read error response: %w", err)
	}

	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err != nil {
		// If we can't parse the error response, return a simple error
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return &errorResp, fmt.Errorf("%s: %s", errorResp.Error.Code, errorResp.Error.Message)
}
