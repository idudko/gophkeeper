// Package config provides application configuration management for the GophKeeper system.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration settings for the application.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Address         string        `mapstructure:"address"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes  int           `mapstructure:"max_header_bytes"`
	EnableCORS      bool          `mapstructure:"enable_cors"`
	EnableMetrics   bool          `mapstructure:"enable_metrics"`
	EnableProfiling bool          `mapstructure:"enable_profiling"`
	TrustedProxies  []string      `mapstructure:"trusted_proxies"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	EnableSSL       bool          `mapstructure:"enable_ssl"`
	SSLCertPath     string        `mapstructure:"ssl_cert_path"`
	SSLKeyPath      string        `mapstructure:"ssl_key_path"`
	SSLMode         string        `mapstructure:"ssl_mode"`
}

// AuthConfig holds authentication and authorization settings.
type AuthConfig struct {
	JWTSecret            string        `mapstructure:"jwt_secret"`
	JWTIssuer            string        `mapstructure:"jwt_issuer"`
	JWTAudience          string        `mapstructure:"jwt_audience"`
	AccessTokenDuration  time.Duration `mapstructure:"access_token_duration"`
	RefreshTokenDuration time.Duration `mapstructure:"refresh_token_duration"`
	EnableRefresh        bool          `mapstructure:"enable_refresh"`
	PasswordCost         int           `mapstructure:"password_cost"`
	TokenHeader          string        `mapstructure:"token_header"`
	TokenCookie          bool          `mapstructure:"token_cookie"`
	CookieSecure         bool          `mapstructure:"cookie_secure"`
	CookieHTTPOnly       bool          `mapstructure:"cookie_http_only"`
	CookieSameSite       string        `mapstructure:"cookie_same_site"`
}

// EncryptionConfig holds encryption settings.
type EncryptionConfig struct {
	Algorithm     string `mapstructure:"algorithm"`
	KeyDerivation string `mapstructure:"key_derivation"`
	KeyIterations int    `mapstructure:"key_iterations"`
	SaltLength    int    `mapstructure:"salt_length"`
	NonceLength   int    `mapstructure:"nonce_length"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level            string `mapstructure:"level"`
	Format           string `mapstructure:"format"`
	Output           string `mapstructure:"output"`
	EnableCaller     bool   `mapstructure:"enable_caller"`
	EnableStacktrace bool   `mapstructure:"enable_stacktrace"`
	MaxSize          int    `mapstructure:"max_size"`
	MaxBackups       int    `mapstructure:"max_backups"`
	MaxAge           int    `mapstructure:"max_age"`
	Compress         bool   `mapstructure:"compress"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:         ":8080",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    15 * time.Second,
			IdleTimeout:     60 * time.Second,
			ShutdownTimeout: 30 * time.Second,
			MaxHeaderBytes:  1 << 20, // 1MB
			EnableCORS:      true,
			EnableMetrics:   true,
			EnableProfiling: false,
			TrustedProxies:  []string{"127.0.0.1", "::1"},
		},
		Database: DatabaseConfig{
			Driver:          "postgres",
			DSN:             "postgres://gophkeeper:gophkeeper@localhost:5432/gophkeeper?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			EnableSSL:       false,
			SSLMode:         "disable",
		},
		Auth: AuthConfig{
			JWTSecret:            "change-this-secret-in-production",
			JWTIssuer:            "gophkeeper",
			JWTAudience:          "gophkeeper-users",
			AccessTokenDuration:  24 * time.Hour,
			RefreshTokenDuration: 30 * 24 * time.Hour,
			EnableRefresh:        true,
			PasswordCost:         12,
			TokenHeader:          "Authorization",
			TokenCookie:          false,
			CookieSecure:         true,
			CookieHTTPOnly:       true,
			CookieSameSite:       "Strict",
		},
		Encryption: EncryptionConfig{
			Algorithm:     "chacha20-poly1305",
			KeyDerivation: "argon2id",
			KeyIterations: 3,
			SaltLength:    16,
			NonceLength:   12,
		},
		Logging: LoggingConfig{
			Level:            "info",
			Format:           "json",
			Output:           "stdout",
			EnableCaller:     true,
			EnableStacktrace: false,
			MaxSize:          100, // MB
			MaxBackups:       3,
			MaxAge:           7, // days
			Compress:         true,
		},
	}
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/gophkeeper")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Set default values
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal config
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values for configuration options.
func setDefaults() {
	viper.SetDefault("server.address", ":8080")
	viper.SetDefault("server.read_timeout", "15s")
	viper.SetDefault("server.write_timeout", "15s")
	viper.SetDefault("server.idle_timeout", "60s")
	viper.SetDefault("server.shutdown_timeout", "30s")
	viper.SetDefault("server.max_header_bytes", 1048576)
	viper.SetDefault("server.enable_cors", true)
	viper.SetDefault("server.enable_metrics", true)
	viper.SetDefault("server.enable_profiling", false)

	viper.SetDefault("database.driver", "postgres")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "5m")
	viper.SetDefault("database.conn_max_idle_time", "5m")
	viper.SetDefault("database.enable_ssl", false)
	viper.SetDefault("database.ssl_mode", "disable")

	viper.SetDefault("auth.jwt_issuer", "gophkeeper")
	viper.SetDefault("auth.jwt_audience", "gophkeeper-users")
	viper.SetDefault("auth.access_token_duration", "24h")
	viper.SetDefault("auth.refresh_token_duration", "720h")
	viper.SetDefault("auth.enable_refresh", true)
	viper.SetDefault("auth.password_cost", 12)
	viper.SetDefault("auth.token_header", "Authorization")
	viper.SetDefault("auth.token_cookie", false)
	viper.SetDefault("auth.cookie_secure", true)
	viper.SetDefault("auth.cookie_http_only", true)
	viper.SetDefault("auth.cookie_same_site", "Strict")

	viper.SetDefault("encryption.algorithm", "chacha20-poly1305")
	viper.SetDefault("encryption.key_derivation", "argon2id")
	viper.SetDefault("encryption.key_iterations", 3)
	viper.SetDefault("encryption.salt_length", 16)
	viper.SetDefault("encryption.nonce_length", 12)

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.enable_caller", true)
	viper.SetDefault("logging.enable_stacktrace", false)
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 7)
	viper.SetDefault("logging.compress", true)

	// Environment variables
	viper.SetEnvPrefix("GOPHKEEPER")
	viper.AutomaticEnv()
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	// Validate database config
	if c.Database.Driver == "" {
		return fmt.Errorf("database driver cannot be empty")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("database DSN cannot be empty")
	}
	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("database max_open_conns must be positive")
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database max_idle_conns cannot be negative")
	}

	// Validate auth config
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-this-secret-in-production" {
		// Warning but not error in dev mode
	}
	if c.Auth.JWTIssuer == "" {
		return fmt.Errorf("auth JWT issuer cannot be empty")
	}
	if c.Auth.AccessTokenDuration <= 0 {
		return fmt.Errorf("auth access token duration must be positive")
	}
	if c.Auth.PasswordCost < 4 || c.Auth.PasswordCost > 31 {
		return fmt.Errorf("auth password cost must be between 4 and 31")
	}

	// Validate encryption config
	validAlgorithms := map[string]bool{
		"chacha20-poly1305": true,
		"aes-256-gcm":       true,
	}
	if !validAlgorithms[c.Encryption.Algorithm] {
		return fmt.Errorf("invalid encryption algorithm: %s", c.Encryption.Algorithm)
	}

	validKeyDerivation := map[string]bool{
		"argon2id": true,
		"scrypt":   true,
		"pbkdf2":   true,
	}
	if !validKeyDerivation[c.Encryption.KeyDerivation] {
		return fmt.Errorf("invalid key derivation method: %s", c.Encryption.KeyDerivation)
	}
	if c.Encryption.KeyIterations <= 0 {
		return fmt.Errorf("encryption key iterations must be positive")
	}
	if c.Encryption.SaltLength <= 0 {
		return fmt.Errorf("encryption salt length must be positive")
	}
	if c.Encryption.NonceLength <= 0 {
		return fmt.Errorf("encryption nonce length must be positive")
	}

	// Validate logging config
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid logging format: %s", c.Logging.Format)
	}

	return nil
}

// GetDSN returns the database connection string.
func (c *Config) GetDSN() string {
	return c.Database.DSN
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Logging.Level == "info" && c.Auth.CookieSecure && c.Database.EnableSSL
}
