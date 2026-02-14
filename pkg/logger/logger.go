// Package logger provides structured logging functionality for the GophKeeper system using zerolog.
package logger

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is the main logger interface.
type Logger interface {
	// Core logging methods
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)

	// Logging with formatted messages
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)

	// Context-aware logging
	With(fields ...Field) Logger
	WithRequestID(requestID string) Logger

	// Synchronous logging (flushes buffers before returning)
	Sync() error

	// Create request-scoped logger
	ForRequest(requestID string) Logger

	// Get underlying zerolog.Logger
	ZeroLogger() zerolog.Logger
}

// zerologLogger implements the Logger interface using zerolog.
type zerologLogger struct {
	logger zerolog.Logger
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// Config holds logger configuration.
type Config struct {
	Level            string // "debug", "info", "warn", "error", "fatal"
	Format           string // "json" or "console"
	Output           string // "stdout", "stderr", or file path
	EnableCaller     bool
	EnableStacktrace bool
	TimeFormat       string
}

// DefaultConfig returns the default logger configuration.
func DefaultConfig() *Config {
	return &Config{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		EnableCaller:     true,
		EnableStacktrace: false,
		TimeFormat:       zerolog.TimeFormatUnix,
	}
}

// New creates a new logger with the given configuration.
func New(config *Config) (Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set global log level
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	if config.TimeFormat != "" {
		zerolog.TimeFieldFormat = config.TimeFormat
	}

	// Create output writer
	output, err := createOutput(config.Output)
	if err != nil {
		return nil, err
	}

	return newLoggerWithWriter(output, config)
}

// NewWithWriter creates a new logger with the given writer and configuration.
// This is primarily useful for testing where you want to capture log output.
func NewWithWriter(writer io.Writer, config *Config) (Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set global log level
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	if config.TimeFormat != "" {
		zerolog.TimeFieldFormat = config.TimeFormat
	}

	return newLoggerWithWriter(writer, config)
}

// newLoggerWithWriter creates a logger with a specific writer using the given config.
func newLoggerWithWriter(writer io.Writer, config *Config) (Logger, error) {
	// Create context with output and level
	ctx := zerolog.New(writer).With().Timestamp()
	if config.EnableCaller {
		ctx = ctx.Caller()
	}
	if config.EnableStacktrace {
		ctx = ctx.Stack()
	}

	// Create logger
	zlogger := ctx.Logger()

	// Set format
	if config.Format == "console" {
		zlogger = zlogger.Output(zerolog.ConsoleWriter{Out: writer, TimeFormat: time.RFC3339})
	}

	return &zerologLogger{
		logger: zlogger,
	}, nil
}

// NewDevelopment creates a development-friendly logger.
func NewDevelopment() (Logger, error) {
	return New(&Config{
		Level:            "debug",
		Format:           "console",
		Output:           "stdout",
		EnableCaller:     true,
		EnableStacktrace: true,
		TimeFormat:       time.RFC3339,
	})
}

// NewProduction creates a production-ready logger.
func NewProduction() (Logger, error) {
	return New(&Config{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		EnableCaller:     true,
		EnableStacktrace: false,
		TimeFormat:       zerolog.TimeFormatUnix,
	})
}

// parseLevel parses a string level name to a zerolog.Level.
func parseLevel(level string) (zerolog.Level, error) {
	switch level {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "fatal":
		return zerolog.FatalLevel, nil
	default:
		return zerolog.InfoLevel, nil
	}
}

// createOutput creates an io.Writer based on config.
func createOutput(output string) (io.Writer, error) {
	switch output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}

// Debug logs a message at DebugLevel.
func (l *zerologLogger) Debug(msg string, fields ...Field) {
	event := l.logger.Debug()
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Info logs a message at InfoLevel.
func (l *zerologLogger) Info(msg string, fields ...Field) {
	event := l.logger.Info()
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Warn logs a message at WarnLevel.
func (l *zerologLogger) Warn(msg string, fields ...Field) {
	event := l.logger.Warn()
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Error logs a message at ErrorLevel.
func (l *zerologLogger) Error(msg string, fields ...Field) {
	event := l.logger.Error()
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Fatal logs a message at FatalLevel and exits the process.
func (l *zerologLogger) Fatal(msg string, fields ...Field) {
	event := l.logger.Fatal()
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// Debugf logs a formatted message at DebugLevel.
func (l *zerologLogger) Debugf(format string, args ...any) {
	l.logger.Debug().Msgf(format, args...)
}

// Infof logs a formatted message at InfoLevel.
func (l *zerologLogger) Infof(format string, args ...any) {
	l.logger.Info().Msgf(format, args...)
}

// Warnf logs a formatted message at WarnLevel.
func (l *zerologLogger) Warnf(format string, args ...any) {
	l.logger.Warn().Msgf(format, args...)
}

// Errorf logs a formatted message at ErrorLevel.
func (l *zerologLogger) Errorf(format string, args ...any) {
	l.logger.Error().Msgf(format, args...)
}

// Fatalf logs a formatted message at FatalLevel and exits the process.
func (l *zerologLogger) Fatalf(format string, args ...any) {
	l.logger.Fatal().Msgf(format, args...)
}

// With returns a logger with additional fields added to each log entry.
func (l *zerologLogger) With(fields ...Field) Logger {
	ctx := l.logger.With()
	for _, f := range fields {
		ctx = ctx.Interface(f.Key, f.Value)
	}
	return &zerologLogger{
		logger: ctx.Logger(),
	}
}

// WithRequestID returns a logger with a request ID field.
func (l *zerologLogger) WithRequestID(requestID string) Logger {
	return l.With(RequestID(requestID))
}

// Sync flushes any buffered log entries.
func (l *zerologLogger) Sync() error {
	// zerolog doesn't buffer, this is a no-op
	return nil
}

// ForRequest creates a new logger instance for a specific request.
func (l *zerologLogger) ForRequest(requestID string) Logger {
	return l.With(
		RequestID(requestID),
		Field{Key: "component", Value: "request"},
	)
}

// ZeroLogger returns the underlying zerolog.Logger.
func (l *zerologLogger) ZeroLogger() zerolog.Logger {
	return l.logger
}

// Global logger instance
var global Logger

// SetGlobal sets the global logger instance.
func SetGlobal(logger Logger) {
	global = logger
}

// GetGlobal returns the global logger instance.
func GetGlobal() Logger {
	if global == nil {
		logger, err := New(nil)
		if err != nil {
			panic(err)
		}
		global = logger
	}
	return global
}

// Convenience functions using the global logger.

func Debug(msg string, fields ...Field) {
	GetGlobal().Debug(msg, fields...)
}

func Info(msg string, fields ...Field) {
	GetGlobal().Info(msg, fields...)
}

func Warn(msg string, fields ...Field) {
	GetGlobal().Warn(msg, fields...)
}

func Error(msg string, fields ...Field) {
	GetGlobal().Error(msg, fields...)
}

func Fatal(msg string, fields ...Field) {
	GetGlobal().Fatal(msg, fields...)
}

func Debugf(format string, args ...any) {
	GetGlobal().Debugf(format, args...)
}

func Infof(format string, args ...any) {
	GetGlobal().Infof(format, args...)
}

func Warnf(format string, args ...any) {
	GetGlobal().Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	GetGlobal().Errorf(format, args...)
}

func Fatalf(format string, args ...any) {
	GetGlobal().Fatalf(format, args...)
}

func With(fields ...Field) Logger {
	return GetGlobal().With(fields...)
}

func WithRequestID(requestID string) Logger {
	return GetGlobal().WithRequestID(requestID)
}

func Sync() error {
	return GetGlobal().Sync()
}

// Common field constructors
var (
	RequestID = func(id string) Field { return Field{Key: "request_id", Value: id} }
	UserID    = func(id string) Field { return Field{Key: "user_id", Value: id} }
	Method    = func(method string) Field { return Field{Key: "method", Value: method} }
	Path      = func(path string) Field { return Field{Key: "path", Value: path} }
	Status    = func(status int) Field { return Field{Key: "status", Value: status} }
	Latency   = func(d any) Field { return Field{Key: "latency", Value: d} }
	Err       = func(err error) Field { return Field{Key: "error", Value: err} }

	// Basic types
	String  = func(key, value string) Field { return Field{Key: key, Value: value} }
	Int     = func(key string, value int) Field { return Field{Key: key, Value: value} }
	Int64   = func(key string, value int64) Field { return Field{Key: key, Value: value} }
	Float64 = func(key string, value float64) Field { return Field{Key: key, Value: value} }
	Bool    = func(key string, value bool) Field { return Field{Key: key, Value: value} }
	Dur     = func(key string, value time.Duration) Field { return Field{Key: key, Value: value} }
	Time    = func(key string, value time.Time) Field { return Field{Key: key, Value: value} }
	Any     = func(key string, value any) Field { return Field{Key: key, Value: value} }
)

// GenerateRequestID generates a unique request ID for tracking.
// Uses a simple format: timestamp + random string for uniqueness.
func GenerateRequestID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))
}

// randomString generates a random string of specified length.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Level names
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

// Init initializes the global logger with the given configuration.
// This is a convenience function for quick setup.
func Init(config *Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}
	SetGlobal(logger)
	return nil
}

// UseLog sets the global logger to use the standard zerolog log package.
// This is useful when you want to use the default zerolog logger.
func UseLog() {
	global = &zerologLogger{logger: log.Logger}
}
