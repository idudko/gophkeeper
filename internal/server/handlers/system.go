// Package handlers provides HTTP request handlers for GophKeeper server system endpoints.
package handlers

import (
	"context"

	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/idudko/gophkeeper/pkg/server"
	"github.com/idudko/gophkeeper/pkg/storage"
)

// SystemHandler handles system-related HTTP requests for version, health checks, etc.
type SystemHandler struct {
	storage   storage.Storage
	log       logger.Logger
	version   string
	buildDate time.Time
	gitCommit string
	goVersion string
}

// NewSystemHandler creates a new system handler.
func NewSystemHandler(
	storage storage.Storage,
	log logger.Logger,
	version string,
	buildDate time.Time,
	gitCommit string,
) *SystemHandler {
	return &SystemHandler{
		storage:   storage,
		log:       log,
		version:   version,
		buildDate: buildDate,
		gitCommit: gitCommit,
		goVersion: runtime.Version(),
	}
}

// GetVersion handles version information requests.
func (h *SystemHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	log.Info("version request received")

	// Get database status
	dbStatus := h.getSystemDatabaseStatus(r.Context(), log)

	// Build response
	response := map[string]any{
		"success": true,
		"data": map[string]any{
			"version":     h.version,
			"build_date":  h.buildDate.UTC().Format(time.RFC3339),
			"git_commit":  h.gitCommit,
			"go_version":  h.goVersion,
			"platform":    fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			"api_version": api.APIVersion,
			"features": []string{
				"encryption",
				"sync",
				"version_tracking",
				"conflict_resolution",
			},
		},
		"meta": map[string]any{
			"database": dbStatus,
		},
	}

	server.RespondJSON(w, http.StatusOK, response)
}

// GetHealth handles health check requests.
func (h *SystemHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get database status with retry logic
	dbStatus := h.checkSystemDatabaseHealth(ctx, 3, log)

	// Determine overall health status
	status := "healthy"
	httpStatus := http.StatusOK

	if dbStatus.Status == "unhealthy" {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	log.Info("health check completed",
		logger.String("status", status),
		logger.String("database", dbStatus.Status),
	)

	// Build response
	response := map[string]any{
		"success": status == "healthy",
		"data": map[string]any{
			"status":    status,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    h.getSystemUptime(),
			"services": map[string]string{
				"database": dbStatus.Status,
				"api":      "healthy",
			},
			"database": dbStatus,
		},
	}

	server.RespondJSON(w, httpStatus, response)
}

// SystemDatabaseStatus represents database health status for system endpoints.
type SystemDatabaseStatus struct {
	Status    string `json:"status"`
	Latency   string `json:"latency,omitempty"`
	Connected bool   `json:"connected"`
}

// getSystemDatabaseStatus checks database health for system endpoints.
func (h *SystemHandler) getSystemDatabaseStatus(ctx context.Context, log logger.Logger) SystemDatabaseStatus {
	start := time.Now()

	err := h.storage.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		log.Error("database health check failed",
			logger.Err(err),
			logger.String("latency", latency.String()),
		)
		return SystemDatabaseStatus{
			Status:    "unhealthy",
			Connected: false,
		}
	}

	return SystemDatabaseStatus{
		Status:    "healthy",
		Latency:   latency.String(),
		Connected: true,
	}
}

// checkSystemDatabaseHealth performs health check with retry logic for system endpoints.
func (h *SystemHandler) checkSystemDatabaseHealth(ctx context.Context, maxRetries int, log logger.Logger) SystemDatabaseStatus {
	var lastStatus SystemDatabaseStatus

	for i := range maxRetries {
		lastStatus = h.getSystemDatabaseStatus(ctx, log)

		if lastStatus.Connected {
			return lastStatus
		}

		// Exponential backoff
		waitTime := time.Duration(1<<uint(i)) * 100 * time.Millisecond
		if i < maxRetries-1 {
			log.Warn("database health check failed, retrying",
				logger.Int("attempt", i+1),
				logger.Int("max_retries", maxRetries),
				logger.String("wait_time", waitTime.String()),
			)
			time.Sleep(waitTime)
		}
	}

	return lastStatus
}

// getSystemUptime calculates server uptime for system endpoints.
func (h *SystemHandler) getSystemUptime() string {
	if h.buildDate.IsZero() {
		return "unknown"
	}
	uptime := time.Since(h.buildDate)

	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
