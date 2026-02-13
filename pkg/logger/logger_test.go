// Package logger provides tests for logger package.
package logger

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.Equal(t, "stdout", config.Output)
	assert.True(t, config.EnableCaller)
	assert.False(t, config.EnableStacktrace)
}

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)
	assert.NotNil(t, lgr)

	// Test logging
	lgr.Info("test message", String("key", "value"))
	assert.NotEmpty(t, buf.String())
}

func TestLogger_Levels(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "info",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	// Debug message should not be logged
	lgr.Debug("debug message")
	buf.Reset()

	// Info message should be logged
	lgr.Info("info message")
	assert.Contains(t, buf.String(), "info message")

	buf.Reset()

	// Error message should be logged
	lgr.Error("error message")
	assert.Contains(t, buf.String(), "error message")
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Info("test",
		String("string_key", "string_value"),
		Int("int_key", 42),
		Bool("bool_key", true),
	)

	output := buf.String()
	assert.Contains(t, output, "string_key")
	assert.Contains(t, output, "string_value")
	assert.Contains(t, output, "int_key")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "bool_key")
	assert.Contains(t, output, "true")
}

func TestLogger_ErrorField(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	testErr := assert.AnError
	lgr.Error("error occurred", Err(testErr))

	output := buf.String()
	assert.Contains(t, output, "error")
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr = lgr.With(
		String("static_field", "static_value"),
		RequestID("req-123"),
	)

	lgr.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "static_field")
	assert.Contains(t, output, "static_value")
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "req-123")
}

func TestLogger_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	requestID := "test-request-id"
	lgr = lgr.WithRequestID(requestID)

	lgr.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, requestID)
}

func TestLogger_FormattedMessages(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Debugf("debug formatted: %s", "debug")
	assert.Contains(t, buf.String(), "debug formatted: debug")

	buf.Reset()

	lgr.Infof("info formatted: %s", "info")
	assert.Contains(t, buf.String(), "info formatted: info")

	buf.Reset()

	lgr.Warnf("warn formatted: %d", 42)
	assert.Contains(t, buf.String(), "warn formatted: 42")

	buf.Reset()

	lgr.Errorf("error formatted: %f", 3.14)
	assert.Contains(t, buf.String(), "error formatted: 3.14")
}

func TestLogger_CommonFields(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Info("test",
		RequestID("req-456"),
		UserID("user-789"),
		Method("GET"),
		Path("/api/test"),
		Status(200),
		Latency(100*time.Millisecond),
	)

	output := buf.String()
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "req-456")
	assert.Contains(t, output, "user_id")
	assert.Contains(t, output, "user-789")
	assert.Contains(t, output, "method")
	assert.Contains(t, output, "GET")
	assert.Contains(t, output, "path")
	assert.Contains(t, output, "/api/test")
	assert.Contains(t, output, "status")
	assert.Contains(t, output, "200")
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "info",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Info("json test", String("key", "value"))

	// Verify it's valid JSON
	var logEntry map[string]any
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "json test", logEntry["message"])
	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "value", logEntry["key"])
}

func TestLogger_ConsoleFormat(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "info",
		Format:           "console",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Info("console test")

	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "console test")
	assert.Contains(t, output, "INF")
}

func TestLogger_WithCaller(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     true,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	lgr.Info("caller test")

	output := buf.String()
	assert.Contains(t, output, "caller")
}

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "-")
	assert.Contains(t, id2, "-")
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "info",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	SetGlobal(lgr)

	// Test global logger functions
	assert.NotPanics(t, func() {
		Info("global info")
		Warn("global warn")
		Error("global error")
		Debug("global debug")
	})

	global := GetGlobal()
	assert.NotNil(t, global)
}

func TestLogger_Sync(t *testing.T) {
	config := &Config{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := New(config)
	require.NoError(t, err)

	err = lgr.Sync()
	assert.NoError(t, err)
}

func TestLogger_ZeroLogger(t *testing.T) {
	config := &Config{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := New(config)
	require.NoError(t, err)

	zl := lgr.ZeroLogger()
	assert.NotNil(t, zl)
}

func TestLogger_AllLevels(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		logFunc func(Logger)
	}{
		{
			name:  "debug level",
			level: "debug",
			logFunc: func(lgr Logger) {
				lgr.Debug("debug message")
			},
		},
		{
			name:  "info level",
			level: "info",
			logFunc: func(lgr Logger) {
				lgr.Info("info message")
			},
		},
		{
			name:  "warn level",
			level: "warn",
			logFunc: func(lgr Logger) {
				lgr.Warn("warn message")
			},
		},
		{
			name:  "error level",
			level: "error",
			logFunc: func(lgr Logger) {
				lgr.Error("error message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &Config{
				Level:            tt.level,
				Format:           "json",
				EnableCaller:     false,
				EnableStacktrace: false,
			}

			lgr, err := NewWithWriter(&buf, config)
			require.NoError(t, err)

			tt.logFunc(lgr)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestNewProduction(t *testing.T) {
	lgr, err := NewProduction()
	require.NoError(t, err)
	assert.NotNil(t, lgr)
}

func TestNewDevelopment(t *testing.T) {
	lgr, err := NewDevelopment()
	require.NoError(t, err)
	assert.NotNil(t, lgr)
}

func TestInit(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	// Set as global
	SetGlobal(lgr)

	// Test that global logger works
	logger := GetGlobal()
	assert.NotNil(t, logger)
	logger.Info("test message", String("key", "value"))

	// Verify log was written
	assert.NotEmpty(t, buf.String())
}

func TestSetGlobal(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)

	SetGlobal(lgr)

	// Verify global logger is set
	global := GetGlobal()
	assert.NotNil(t, global)
}

func TestGetGlobal(t *testing.T) {
	// Clear global logger
	SetGlobal(nil)

	// GetGlobal should create default logger if none exists
	logger := GetGlobal()
	assert.NotNil(t, logger)

	// Set a custom logger
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)
	SetGlobal(lgr)

	// GetGlobal should return the custom logger
	logger = GetGlobal()
	assert.NotNil(t, logger)
}

func TestUseLog(t *testing.T) {
	// Set global logger using UseLog
	UseLog()

	// Verify global logger is set
	logger := GetGlobal()
	assert.NotNil(t, logger)

	// Verify it has zerolog's logger
	assert.NotNil(t, logger.ZeroLogger())
}

func TestGlobalConvenienceFunctions(t *testing.T) {
	var buf bytes.Buffer
	config := &Config{
		Level:            "debug",
		Format:           "json",
		EnableCaller:     false,
		EnableStacktrace: false,
	}

	lgr, err := NewWithWriter(&buf, config)
	require.NoError(t, err)
	SetGlobal(lgr)

	// Test all convenience functions
	Debug("debug message", String("key1", "value1"))
	Info("info message", String("key2", "value2"))
	Warn("warn message", String("key3", "value3"))
	Error("error message", String("key4", "value4"))

	Debugf("formatted %s", "debug")
	Infof("formatted %s", "info")
	Warnf("formatted %s", "warn")
	Errorf("formatted %s", "error")

	// Test With and WithRequestID
	logger := With(String("context", "test"))
	assert.NotNil(t, logger)

	logger = WithRequestID("test-req-id")
	assert.NotNil(t, logger)

	// Test Sync
	err = Sync()
	require.NoError(t, err)
}
