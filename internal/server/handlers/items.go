// Package handlers provides HTTP request handlers for items management.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/idudko/gophkeeper/pkg/server"
	"github.com/idudko/gophkeeper/pkg/storage"
)

// ItemsHandler handles items-related HTTP requests.
type ItemsHandler struct {
	storage storage.Storage
	log     logger.Logger
}

// NewItemsHandler creates a new items handler.
func NewItemsHandler(storage storage.Storage, log logger.Logger) *ItemsHandler {
	return &ItemsHandler{
		storage: storage,
		log:     log,
	}
}

// CreateItemRequest represents a request to create a new item.
type CreateItemRequest struct {
	Type models.DataType       `json:"type" validate:"required,oneof=password text binary card"`
	Meta string                `json:"meta" validate:"max=256"`
	Data CreateItemDataRequest `json:"data" validate:"required"`
}

// CreateItemDataRequest represents type-specific item data for creation.
type CreateItemDataRequest struct {
	Password *models.PasswordData `json:"password,omitempty"`
	Text     *models.TextData     `json:"text,omitempty"`
	Binary   *models.BinaryData   `json:"binary,omitempty"`
	Card     *models.CardData     `json:"card,omitempty"`
}

// UpdateItemRequest represents a request to update an existing item.
type UpdateItemRequest struct {
	Version int                    `json:"version" validate:"required,min=1"`
	Meta    *string                `json:"meta,omitempty" validate:"omitempty,max=256"`
	Data    *CreateItemDataRequest `json:"data,omitempty"`
}

// CreateItem handles creation of a new item.
func (h *ItemsHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Decode request
	var req CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate request
	if req.Type == "" {
		server.RespondError(w, http.StatusBadRequest, "item type is required", "TYPE_REQUIRED", log, r)
		return
	}

	// Validate data type
	validTypes := map[models.DataType]bool{
		models.DataTypePassword: true,
		models.DataTypeText:     true,
		models.DataTypeBinary:   true,
		models.DataTypeCard:     true,
	}
	if !validTypes[req.Type] {
		server.RespondError(w, http.StatusBadRequest, "invalid item type", "INVALID_TYPE", log, r)
		return
	}

	// Validate type-specific data
	dataCount := 0
	if req.Data.Password != nil {
		dataCount++
	}
	if req.Data.Text != nil {
		dataCount++
	}
	if req.Data.Binary != nil {
		dataCount++
	}
	if req.Data.Card != nil {
		dataCount++
	}

	if dataCount == 0 {
		server.RespondError(w, http.StatusBadRequest, "item data is required", "DATA_REQUIRED", log, r)
		return
	}
	if dataCount > 1 {
		server.RespondError(w, http.StatusBadRequest, "only one data type can be specified", "MULTIPLE_DATA_TYPES", log, r)
		return
	}

	// Validate specific data fields
	switch req.Type {
	case models.DataTypePassword:
		if req.Data.Password == nil || req.Data.Password.Login == "" || req.Data.Password.Password == "" {
			server.RespondError(w, http.StatusBadRequest, "login and password are required", "MISSING_FIELDS", log, r)
			return
		}
	case models.DataTypeText:
		if req.Data.Text == nil || req.Data.Text.Data == "" {
			server.RespondError(w, http.StatusBadRequest, "text data is required", "MISSING_FIELDS", log, r)
			return
		}
	case models.DataTypeBinary:
		if req.Data.Binary == nil || req.Data.Binary.Filename == "" || len(req.Data.Binary.Data) == 0 {
			server.RespondError(w, http.StatusBadRequest, "filename and data are required", "MISSING_FIELDS", log, r)
			return
		}
	case models.DataTypeCard:
		if req.Data.Card == nil || req.Data.Card.Number == "" || req.Data.Card.CVV == "" {
			server.RespondError(w, http.StatusBadRequest, "card number and CVV are required", "MISSING_FIELDS", log, r)
			return
		}
	}

	log.Info("creating new item",
		logger.String("user_id", userID),
		logger.String("type", string(req.Type)),
	)

	// Create item model
	item := &models.Item{
		ID:        uuid.New().String(),
		UserID:    userID,
		Type:      req.Type,
		Version:   1,
		Meta:      req.Meta,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set type-specific data
	item.Password = req.Data.Password
	item.Text = req.Data.Text
	item.Binary = req.Data.Binary
	item.Card = req.Data.Card

	// Store item with retry logic
	// Note: Data encryption is handled on the client-side using the user's master password
	if err := h.createItemWithRetry(ctx, item, 3); err != nil {
		log.Error("failed to create item",
			logger.Err(err),
			logger.String("item_id", item.ID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to create item", "CREATE_ERROR", log, r)
		return
	}

	log.Info("item created successfully",
		logger.String("item_id", item.ID),
		logger.String("user_id", userID),
	)

	// Respond with created item
	server.RespondJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"data": map[string]any{
			"id":         item.ID,
			"type":       string(item.Type),
			"version":    item.Version,
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		},
	})
}

// GetItems handles retrieval of items list.
func (h *ItemsHandler) GetItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Parse query parameters
	dataType := models.DataType(r.URL.Query().Get("type"))
	limit := server.ParseIntQuery(r.URL.Query().Get("limit"), api.DefaultLimit, 1, api.MaxLimit)
	offset := server.ParseIntQuery(r.URL.Query().Get("offset"), api.DefaultOffset, 0, -1)

	log.Info("retrieving items",
		logger.String("user_id", userID),
		logger.String("type", string(dataType)),
		logger.Int("limit", limit),
		logger.Int("offset", offset),
	)

	// Get items with retry logic
	items, err := h.getItemsWithRetry(ctx, userID, dataType, limit, offset, 3)
	if err != nil {
		log.Error("failed to get items",
			logger.Err(err),
			logger.String("user_id", userID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to retrieve items", "GET_ITEMS_ERROR", log, r)
		return
	}

	// Convert items to response format
	itemResponses := make([]map[string]any, len(items))
	for i, item := range items {
		itemResponses[i] = map[string]any{
			"id":         item.ID,
			"type":       string(item.Type),
			"version":    item.Version,
			"meta":       item.Meta,
			"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
			"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}

	log.Info("items retrieved successfully",
		logger.String("user_id", userID),
		logger.Int("count", len(items)),
	)

	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"items":  itemResponses,
			"total":  len(items),
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetItem handles retrieval of a specific item.
func (h *ItemsHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Get item ID from URL
	itemID := r.URL.Query().Get("id")
	if itemID == "" {
		itemID = chi.URLParam(r, "id")
	}
	if itemID == "" {
		server.RespondError(w, http.StatusBadRequest, "item ID is required", "ITEM_ID_REQUIRED", log, r)
		return
	}

	// Validate UUID format
	if _, err := uuid.Parse(itemID); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid item ID format", "INVALID_ITEM_ID", log, r)
		return
	}

	log.Info("retrieving item",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
	)

	// Get item with retry logic
	item, err := h.getItemWithRetry(ctx, userID, itemID, 3)
	if err != nil {
		if err == models.ErrItemNotFound {
			server.RespondError(w, http.StatusNotFound, "item not found", "NOT_FOUND", log, r)
			return
		}
		log.Error("failed to get item",
			logger.Err(err),
			logger.String("item_id", itemID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to retrieve item", "GET_ITEM_ERROR", log, r)
		return
	}

	// Respond with item data
	// Note: Data decryption is handled on the client-side using the user's master password
	// Build response
	response := map[string]any{
		"id":         item.ID,
		"type":       string(item.Type),
		"version":    item.Version,
		"meta":       item.Meta,
		"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
	}

	// Add type-specific data
	switch item.Type {
	case models.DataTypePassword:
		response["data"] = item.Password
	case models.DataTypeText:
		response["data"] = item.Text
	case models.DataTypeBinary:
		response["data"] = item.Binary
	case models.DataTypeCard:
		response["data"] = item.Card
	}

	log.Info("item retrieved successfully",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
	)

	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    response,
	})
}

// UpdateItem handles update of an existing item.
func (h *ItemsHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Get item ID from URL
	itemID := r.URL.Query().Get("id")
	if itemID == "" {
		itemID = chi.URLParam(r, "id")
	}
	if itemID == "" {
		server.RespondError(w, http.StatusBadRequest, "item ID is required", "ITEM_ID_REQUIRED", log, r)
		return
	}

	// Decode request
	var req UpdateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, http.StatusBadRequest, "invalid request body", "INVALID_JSON", log, r)
		return
	}
	defer r.Body.Close()

	// Validate version (optimistic locking)
	if req.Version <= 0 {
		server.RespondError(w, http.StatusBadRequest, "version must be positive", "INVALID_VERSION", log, r)
		return
	}

	log.Info("updating item",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
		logger.Int("version", req.Version),
	)

	// Get current item with retry logic
	item, err := h.getItemWithRetry(ctx, userID, itemID, 3)
	if err != nil {
		if err == models.ErrItemNotFound {
			server.RespondError(w, http.StatusNotFound, "item not found", "NOT_FOUND", log, r)
			return
		}
		log.Error("failed to get item",
			logger.Err(err),
			logger.String("item_id", itemID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to retrieve item", "GET_ITEM_ERROR", log, r)
		return
	}

	// Check version for optimistic locking
	if item.Version != req.Version {
		server.RespondError(w, http.StatusConflict, "item has been modified by another process", "VERSION_CONFLICT", log, r)
		return
	}

	// Update item fields
	if req.Meta != nil {
		item.Meta = *req.Meta
	}
	if req.Data != nil {
		dataCount := 0
		if req.Data.Password != nil {
			dataCount++
			item.Password = req.Data.Password
		}
		if req.Data.Text != nil {
			dataCount++
			item.Text = req.Data.Text
		}
		if req.Data.Binary != nil {
			dataCount++
			item.Binary = req.Data.Binary
		}
		if req.Data.Card != nil {
			dataCount++
			item.Card = req.Data.Card
		}

		if dataCount > 1 {
			server.RespondError(w, http.StatusBadRequest, "only one data type can be specified", "MULTIPLE_DATA_TYPES", log, r)
			return
		}
	}

	item.UpdatedAt = time.Now()

	// Update item with retry logic
	if err := h.updateItemWithRetry(ctx, item, 3); err != nil {
		if err == models.ErrVersionConflict {
			server.RespondError(w, http.StatusConflict, "item has been modified by another process", "VERSION_CONFLICT", log, r)
			return
		}
		log.Error("failed to update item",
			logger.Err(err),
			logger.String("item_id", itemID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to update item", "UPDATE_ERROR", log, r)
		return
	}

	log.Info("item updated successfully",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
		logger.Int("version", item.Version),
	)

	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id":         item.ID,
			"type":       string(item.Type),
			"version":    item.Version,
			"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
		},
	})
}

// DeleteItem handles deletion of an item.
func (h *ItemsHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.With(logger.RequestID(server.GetRequestID(r)))

	// Get user ID from context
	userID := server.GetUserIDFromContext(ctx)
	if userID == "" {
		server.RespondError(w, http.StatusUnauthorized, "user not authenticated", "UNAUTHORIZED", log, r)
		return
	}

	// Get item ID from URL
	itemID := r.URL.Query().Get("id")
	if itemID == "" {
		itemID = chi.URLParam(r, "id")
	}
	if itemID == "" {
		server.RespondError(w, http.StatusBadRequest, "item ID is required", "ITEM_ID_REQUIRED", log, r)
		return
	}

	log.Info("deleting item",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
	)

	// Delete item with retry logic
	if err := h.deleteItemWithRetry(ctx, userID, itemID, 3); err != nil {
		if err == models.ErrItemNotFound {
			server.RespondError(w, http.StatusNotFound, "item not found", "NOT_FOUND", log, r)
			return
		}
		log.Error("failed to delete item",
			logger.Err(err),
			logger.String("item_id", itemID),
		)
		server.RespondError(w, http.StatusInternalServerError, "failed to delete item", "DELETE_ERROR", log, r)
		return
	}

	log.Info("item deleted successfully",
		logger.String("user_id", userID),
		logger.String("item_id", itemID),
	)

	server.RespondJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id":         itemID,
			"deleted_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// Encryption is now handled client-side using the user's master password.
// The server stores encrypted data as-is from the client without decrypting it.
// This implements Zero-Knowledge architecture where the server cannot access user data.

// Retry helper methods

func (h *ItemsHandler) createItemWithRetry(ctx context.Context, item *models.Item, maxRetries int) error {
	var err error
	for i := range maxRetries {
		err = h.storage.CreateItem(ctx, item)
		if err == nil {
			return nil
		}
		if err == models.ErrUserAlreadyExists {
			return err
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return err
}

func (h *ItemsHandler) getItemsWithRetry(ctx context.Context, userID string, dataType models.DataType, limit, offset int, maxRetries int) ([]*models.Item, error) {
	var items []*models.Item
	var err error
	for i := range maxRetries {
		items, err = h.storage.GetItems(ctx, userID, dataType, limit, offset)
		if err == nil {
			return items, nil
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return nil, err
}

func (h *ItemsHandler) getItemWithRetry(ctx context.Context, userID, itemID string, maxRetries int) (*models.Item, error) {
	var item *models.Item
	var err error
	for i := range maxRetries {
		item, err = h.storage.GetItem(ctx, userID, itemID)
		if err == nil {
			return item, nil
		}
		if err == models.ErrItemNotFound {
			return nil, err
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return nil, err
}

func (h *ItemsHandler) updateItemWithRetry(ctx context.Context, item *models.Item, maxRetries int) error {
	var err error
	for i := range maxRetries {
		err = h.storage.UpdateItem(ctx, item)
		if err == nil {
			return nil
		}
		if err == models.ErrVersionConflict || err == models.ErrItemNotFound {
			return err
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return err
}

func (h *ItemsHandler) deleteItemWithRetry(ctx context.Context, userID, itemID string, maxRetries int) error {
	var err error
	for i := range maxRetries {
		err = h.storage.DeleteItem(ctx, userID, itemID)
		if err == nil {
			return nil
		}
		if err == models.ErrItemNotFound {
			return err
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}
	return err
}
