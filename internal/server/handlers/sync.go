// Package handlers provides HTTP request handlers for GophKeeper server.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/idudko/gophkeeper/internal/models"

	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/idudko/gophkeeper/pkg/server"
	"github.com/idudko/gophkeeper/pkg/storage"
)

// SyncHandler handles synchronization-related HTTP requests.

// NewSyncHandler creates a new synchronization handler.
func NewSyncHandler(storage storage.Storage, log logger.Logger) *SyncHandler {
	return &SyncHandler{
		storage: storage,
		log:     log,
	}
}

// SyncRequest represents a synchronization request from client.
type SyncRequest struct {
	LastSyncAt *time.Time `json:"last_sync_at"`
	Items      []SyncItem `json:"items"`
}

// SyncItem represents an item in a sync request.
type SyncItem struct {
	ID        string               `json:"id" validate:"required,uuid"`
	Type      models.DataType      `json:"type" validate:"required"`
	Version   int                  `json:"version"`
	Meta      string               `json:"meta"`
	Password  *models.PasswordData `json:"password,omitempty"`
	Text      *models.TextData     `json:"text,omitempty"`
	Binary    *models.BinaryData   `json:"binary,omitempty"`
	Card      *models.CardData     `json:"card,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// SyncResponse represents a synchronization response to client.
type SyncResponse struct {
	Items      []models.Item  `json:"items"`
	Conflicts  []ConflictItem `json:"conflicts"`
	LastSyncAt time.Time      `json:"last_sync_at"`
	Message    string         `json:"message,omitempty"`
}

// ConflictItem represents a sync conflict.
type ConflictItem struct {
	Local  models.Item `json:"local"`
	Server models.Item `json:"server"`
	Reason string      `json:"reason"`
}

// ResolveConflictRequest represents a conflict resolution request.
type ResolveConflictRequest struct {
	ItemID   string      `json:"item_id" validate:"required,uuid"`
	Strategy string      `json:"strategy" validate:"required,oneof=local server merge"`
	Data     models.Item `json:"data,omitempty"`
}

// SyncHandler handles synchronization-related HTTP requests.
type SyncHandler struct {
	storage storage.Storage
	log     logger.Logger
}

// Sync handles POST /api/v1/sync - synchronizes data with client.
func (h *SyncHandler) Sync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Parse request
	var syncReq SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&syncReq); err != nil {
		log.Error("failed to decode sync request", logger.Err(err))
		server.RespondError(w, http.StatusBadRequest, "Invalid request format", "INVALID_REQUEST", log, r)
		return
	}

	// Get client's last sync time
	var clientLastSync time.Time
	if syncReq.LastSyncAt != nil {
		clientLastSync = *syncReq.LastSyncAt
	}

	log.Info("sync request received",
		logger.String("user_id", userID),
		logger.Time("client_last_sync", clientLastSync),
	)

	// Get server's last sync time for user
	serverLastSync, err := h.storage.GetLastSync(ctx, userID)
	if err != nil {
		log.Error("failed to get last sync time",
			logger.Err(err),
			logger.String("user_id", userID),
		)
		server.RespondError(w, http.StatusInternalServerError, "Failed to sync data", "INTERNAL_ERROR", log, r)
		return
	}

	// Fetch items from server since server's last sync (or all if no sync yet)
	serverItems, err := h.storage.GetItemsSince(ctx, userID, serverLastSync, 100, 0)
	if err != nil {
		log.Error("failed to fetch items for sync",
			logger.Err(err),
			logger.String("user_id", userID),
		)
		server.RespondError(w, http.StatusInternalServerError, "Failed to sync data", "INTERNAL_ERROR", log, r)
		return
	}

	// Convert []*models.Item to []models.Item for response
	items := make([]models.Item, 0, len(serverItems))
	for _, item := range serverItems {
		if item != nil {
			items = append(items, *item)
		}
	}

	// Detect conflicts
	conflicts := h.detectConflicts(syncReq.Items, serverItems)

	// Apply non-conflicting client updates
	for _, clientItem := range syncReq.Items {
		if !h.isInConflicts(clientItem.ID, conflicts) {
			// Create or update item
			_, err := h.createOrUpdateItem(ctx, userID, clientItem)
			if err != nil {
				log.Error("failed to create/update item",
					logger.Err(err),
					logger.String("item_id", clientItem.ID),
					logger.String("user_id", userID),
				)
				// Continue with other items
				continue
			}
		}
	}

	// Update last sync timestamp
	if err := h.storage.UpdateLastSync(ctx, userID, time.Now()); err != nil {
		log.Error("failed to update last sync time",
			logger.Err(err),
			logger.String("user_id", userID),
		)
		// Continue anyway
	}

	// Respond with server items and conflicts
	response := SyncResponse{
		Items:      items,
		Conflicts:  conflicts,
		LastSyncAt: time.Now(),
	}

	server.RespondJSON(w, http.StatusOK, response)
}

// ResolveConflict handles POST /api/v1/sync/resolve-conflict - resolves sync conflicts.
func (h *SyncHandler) ResolveConflict(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Parse request
	var resolveReq ResolveConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&resolveReq); err != nil {
		log.Error("failed to decode resolve conflict request", logger.Err(err))
		server.RespondError(w, http.StatusBadRequest, "Invalid request format", "INVALID_REQUEST", log, r)
		return
	}

	// Fetch current server item
	serverItem, err := h.storage.GetItem(ctx, userID, resolveReq.ItemID)
	if err != nil {
		log.Error("failed to fetch item",
			logger.Err(err),
			logger.String("item_id", resolveReq.ItemID),
			logger.String("user_id", userID),
		)

		server.RespondError(w, http.StatusNotFound, "Item not found", "ITEM_NOT_FOUND", log, r)
		return
	}

	// Apply resolution strategy
	var finalItem *models.Item
	switch resolveReq.Strategy {
	case "local":
		// Use client's version
		finalItem = &resolveReq.Data
		finalItem.ID = resolveReq.ItemID
	case "server":
		// Use server's version
		finalItem = serverItem
	case "merge":
		// Merge both versions (simple implementation: use server with updated fields from client)
		finalItem = h.mergeItems(serverItem, &resolveReq.Data)
	default:
		server.RespondError(w, http.StatusBadRequest, "Invalid resolution strategy", "INVALID_STRATEGY", log, r)
		return
	}

	// Update item with resolved version
	if err := h.storage.UpdateItem(ctx, finalItem); err != nil {
		log.Error("failed to update resolved item",
			logger.Err(err),
			logger.String("item_id", resolveReq.ItemID),
			logger.String("user_id", userID),
		)
		server.RespondError(w, http.StatusInternalServerError, "Failed to resolve conflict", "INTERNAL_ERROR", log, r)
		return
	}

	server.RespondSuccess(w, "Conflict resolved successfully", nil)
}

// detectConflicts detects conflicts between client and server items.
func (h *SyncHandler) detectConflicts(clientItems []SyncItem, serverItems []*models.Item) []ConflictItem {
	conflicts := make([]ConflictItem, 0)

	serverItemMap := make(map[string]*models.Item)
	for _, item := range serverItems {
		serverItemMap[item.ID] = item
	}

	for _, clientItem := range clientItems {
		if serverItem, exists := serverItemMap[clientItem.ID]; exists {
			// Check for conflicts
			if clientItem.Version != serverItem.Version {
				conflicts = append(conflicts, ConflictItem{
					Local: models.Item{
						ID:        clientItem.ID,
						Type:      clientItem.Type,
						Version:   clientItem.Version,
						Meta:      clientItem.Meta,
						CreatedAt: clientItem.CreatedAt,
						UpdatedAt: clientItem.UpdatedAt,
						Password:  clientItem.Password,
						Text:      clientItem.Text,
						Binary:    clientItem.Binary,
						Card:      clientItem.Card,
					},
					Server: *serverItem,
					Reason: fmt.Sprintf("Version mismatch: client=%d, server=%d", clientItem.Version, serverItem.Version),
				})
			}
		}
	}

	return conflicts
}

// isInConflicts checks if an item ID is in the conflicts list.
func (h *SyncHandler) isInConflicts(itemID string, conflicts []ConflictItem) bool {
	for _, conflict := range conflicts {
		if conflict.Local.ID == itemID || conflict.Server.ID == itemID {
			return true
		}
	}
	return false
}

// createOrUpdateItem creates or updates an item based on its existence.
func (h *SyncHandler) createOrUpdateItem(ctx context.Context, userID string, syncItem SyncItem) (*models.Item, error) {
	item := &models.Item{
		ID:        syncItem.ID,
		UserID:    userID,
		Type:      syncItem.Type,
		Version:   syncItem.Version,
		Meta:      syncItem.Meta,
		CreatedAt: syncItem.CreatedAt,
		UpdatedAt: syncItem.UpdatedAt,
		Password:  syncItem.Password,
		Text:      syncItem.Text,
		Binary:    syncItem.Binary,
		Card:      syncItem.Card,
	}

	// Try to get existing item
	existingItem, err := h.storage.GetItem(ctx, userID, item.ID)
	if err != nil {
		// Item doesn't exist, create it
		if err := h.storage.CreateItem(ctx, item); err != nil {
			return nil, err
		}
		return item, nil
	}

	// Item exists, update it
	item.Version = existingItem.Version + 1
	if err := h.storage.UpdateItem(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

// mergeItems merges two item versions (simple implementation).
func (h *SyncHandler) mergeItems(serverItem *models.Item, clientData *models.Item) *models.Item {
	// Simple merge strategy: use server version as base, update fields from client
	merged := *serverItem
	merged.Version = serverItem.Version + 1
	merged.UpdatedAt = time.Now()

	// Update fields from client if provided
	if clientData.Meta != "" {
		merged.Meta = clientData.Meta
	}

	// Type-specific merging would go here based on item type
	// This is a simplified implementation

	return &merged
}
