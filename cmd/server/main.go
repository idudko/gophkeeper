// Package main provides GophKeeper server application.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/idudko/gophkeeper/internal/auth"
	"github.com/idudko/gophkeeper/internal/config"
	"github.com/idudko/gophkeeper/internal/migrations"
	"github.com/idudko/gophkeeper/internal/server/handlers"
	servermiddleware "github.com/idudko/gophkeeper/internal/server/middleware"
	postgresstorage "github.com/idudko/gophkeeper/internal/server/storage"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/rs/zerolog"
)

const (
	// Version is server version
	Version = "1.0.0"

	// Default configuration file path
	defaultConfigPath = "./config/config.yaml"
)

// Build information (set by linker flags)
var (
	gitCommit string
	buildDate string
	goVersion string
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", defaultConfigPath, "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	lgr, err := logger.New(&logger.Config{
		Level:            cfg.Logging.Level,
		Format:           cfg.Logging.Format,
		Output:           cfg.Logging.Output,
		EnableCaller:     cfg.Logging.EnableCaller,
		EnableStacktrace: cfg.Logging.EnableStacktrace,
		TimeFormat:       time.RFC3339,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	logger.SetGlobal(lgr)

	lgr.Info("Starting GophKeeper server",
		logger.String("version", Version),
		logger.String("git_commit", gitCommit),
		logger.String("build_date", buildDate),
		logger.String("config", *configPath),
	)

	// Set build date if not provided
	if buildDate == "" {
		buildDate = time.Now().Format(time.RFC3339)
	}

	// Initialize database storage
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbConfig := &postgresstorage.Config{
		DSN:               cfg.Database.DSN,
		MaxOpenConns:      int32(cfg.Database.MaxOpenConns),
		MaxIdleConns:      int32(cfg.Database.MaxIdleConns),
		MinConns:          int32(2),
		ConnMaxLifetime:   cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime:   cfg.Database.ConnMaxIdleTime,
		HealthCheckPeriod: 1 * time.Minute,
	}

	dbStorage, err := postgresstorage.NewPostgresStorage(ctx, dbConfig)
	if err != nil {
		lgr.Fatal("Failed to initialize database",
			logger.Err(err),
			logger.String("dsn", cfg.Database.DSN),
		)
	}
	defer dbStorage.Close()

	lgr.Info("Database connection established",
		logger.String("max_open_conns", fmt.Sprintf("%d", cfg.Database.MaxOpenConns)),
		logger.String("max_idle_conns", fmt.Sprintf("%d", cfg.Database.MaxIdleConns)),
	)

	// Run database migrations
	zlog := zerolog.New(os.Stderr).With().Timestamp().Logger()
	migrator, err := migrations.NewMigrator(cfg.Database.DSN, zlog)
	if err != nil {
		lgr.Fatal("Failed to create migrator",
			logger.Err(err),
		)
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil {
		lgr.Fatal("Failed to run migrations",
			logger.Err(err),
		)
	}

	lgr.Info("Database migrations completed")

	// Initialize authenticator
	authenticator := auth.NewAuthenticator(
		cfg.Auth.JWTSecret,
		cfg.Auth.JWTIssuer,
		cfg.Auth.JWTIssuer,
		cfg.Auth.AccessTokenDuration,
		cfg.Auth.RefreshTokenDuration,
		cfg.Auth.PasswordCost,
	)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(dbStorage, authenticator, lgr)
	itemsHandler := handlers.NewItemsHandler(dbStorage, lgr)
	syncHandler := handlers.NewSyncHandler(dbStorage, lgr)
	systemHandler := handlers.NewSystemHandler(
		dbStorage,
		lgr,
		Version,
		parseBuildDate(buildDate),
		gitCommit,
	)

	// Setup router
	router := chi.NewRouter()

	// Apply global middleware
	router.Use(
		servermiddleware.RequestIDMiddleware,
		servermiddleware.RecoveryMiddleware(lgr),
		servermiddleware.LoggingMiddleware(lgr),
		middleware.Heartbeat("/ping"),
	)

	// Apply CORS if enabled
	if cfg.Server.EnableCORS {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
			MaxAge:           300,
		})
		router.Use(corsHandler.Handler)
	}

	// Setup routes
	setupRoutes(router, authHandler, itemsHandler, syncHandler, systemHandler, authenticator, lgr)

	// Create HTTP server
	srv := &http.Server{
		Addr:           cfg.Server.Address,
		Handler:        router,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// Create HTTP client with retry for external calls
	_ = createRetryableHTTPClient(lgr)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		lgr.Info("Server starting",
			logger.String("address", cfg.Server.Address),
			logger.String("read_timeout", cfg.Server.ReadTimeout.String()),
			logger.String("write_timeout", cfg.Server.WriteTimeout.String()),
		)
		errChan <- srv.ListenAndServe()
	}()

	// Create context that is cancelled on interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Wait for either server error or interrupt signal
	select {
	case err := <-errChan:
		if err != nil && err != http.ErrServerClosed {
			lgr.Error("Server failed to start",
				logger.Err(err),
			)
		}
	case <-ctx.Done():
		lgr.Info("Received shutdown signal")
	}

	// Graceful shutdown
	lgr.Info("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		lgr.Error("Server shutdown error",
			logger.Err(err),
		)
	} else {
		lgr.Info("Server shutdown completed successfully")
	}

	lgr.Info("GophKeeper server stopped")
}

// setupRoutes configures all API routes.
func setupRoutes(
	router *chi.Mux,
	authHandler *handlers.AuthHandler,
	itemsHandler *handlers.ItemsHandler,
	syncHandler *handlers.SyncHandler,
	systemHandler *handlers.SystemHandler,
	authenticator *auth.Authenticator,
	lgr logger.Logger,
) {
	// API version v1 routes
	router.Route(api.BasePath, func(r chi.Router) {
		// Public routes (no authentication required)
		r.Group(func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Get("/version", systemHandler.GetVersion)
			r.Get("/health", systemHandler.GetHealth)
		})

		// Refresh token route (requires refresh token)
		r.Group(func(r chi.Router) {
			r.Use(servermiddleware.RefreshTokenMiddleware(authenticator, lgr))
			r.Post("/refresh", authHandler.RefreshToken)
		})

		// Authenticated routes (require access token)
		r.Group(func(r chi.Router) {
			r.Use(servermiddleware.AuthMiddleware(authenticator, lgr))
			r.Use(servermiddleware.ContentTypeMiddleware(lgr))

			// Auth routes
			r.Post("/password/change", authHandler.ChangePassword)

			// Items routes
			r.Route("/items", func(r chi.Router) {
				r.Get("/", itemsHandler.GetItems)
				r.Post("/", itemsHandler.CreateItem)
				r.Get("/{id}", itemsHandler.GetItem)
				r.Put("/{id}", itemsHandler.UpdateItem)
				r.Delete("/{id}", itemsHandler.DeleteItem)
			})

			// Sync routes
			r.Route("/sync", func(r chi.Router) {
				r.Post("/", syncHandler.Sync)
				r.Post("/resolve-conflict", syncHandler.ResolveConflict)
			})
		})
	})

	// Root endpoint
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(api.HeaderContentType, api.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{
  "success": true,
  "data": {
    "name": "GophKeeper API",
    "version": "%s",
    "description": "Secure password manager API",
    "endpoints": {
      "api": "/api/v1",
      "docs": "/api/v1/docs",
      "health": "/api/v1/health",
      "version": "/api/v1/version"
    }
  }
}`, Version)
	})
}

// createRetryableHTTPClient creates an HTTP client with retry support.
func createRetryableHTTPClient(lgr logger.Logger) *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 500 * time.Millisecond
	client.RetryWaitMax = 5 * time.Second
	client.Backoff = retryablehttp.DefaultBackoff
	client.Logger = nil

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

// parseBuildDate parses build date string to time.Time.
func parseBuildDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return time.Now()
	}
	return t
}
