// Package commands provides HTTP utilities for GophKeeper client commands.
package commands

import (
	"context"
	"fmt"

	"github.com/idudko/gophkeeper/internal/client/http"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/rs/zerolog"
)

// getHTTPClient creates an HTTP client with retry support.
func getHTTPClient(serverAddr, token string) (*http.Client, error) {
	cfg := &http.Config{
		ServerAddr:  serverAddr,
		AccessToken: token,
		UserAgent:   "GophKeeperCLI/1.0.0",
		Logger:      zerolog.Nop(),
	}

	return http.NewClient(cfg)
}

// fetchItems fetches items from the server using the retryable HTTP client.
func fetchItems(ctx context.Context, serverAddr, token, dataType string) ([]models.Item, error) {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	path := api.PathItems
	if dataType != "" {
		path += "?type=" + dataType
	}

	var listResp struct {
		Success bool          `json:"success"`
		Data    []models.Item `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodGET, path, nil, &listResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if listResp.Error != nil {
		return nil, fmt.Errorf("ошибка сервера: %s (%s)", listResp.Error.Message, listResp.Error.Code)
	}

	return listResp.Data, nil
}

// sendCreateItemRequest sends a create item request to the server using the retryable HTTP client.
func sendCreateItemRequest(ctx context.Context, serverAddr, token string, item *models.Item) (*models.Item, error) {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	var createResp struct {
		Success bool         `json:"success"`
		Data    *models.Item `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodPOST, api.PathItems, item, &createResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if createResp.Error != nil {
		return nil, fmt.Errorf("ошибка сервера: %s (%s)", createResp.Error.Message, createResp.Error.Code)
	}

	if !createResp.Success || createResp.Data == nil {
		return nil, fmt.Errorf("не удалось создать элемент данных")
	}

	return createResp.Data, nil
}

// sendUpdateItemRequestInternal sends an update item request to the server using the retryable HTTP client.
func sendUpdateItemRequestInternal(ctx context.Context, serverAddr, token, itemID string, item *models.Item, skipResponse bool) (*models.Item, error) {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	path := api.PathItems + "/" + itemID

	if skipResponse {
		resp, err := client.Put(ctx, path, item)
		if err != nil {
			return nil, fmt.Errorf("ошибка запроса: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
		}

		return nil, nil
	}

	var updateResp struct {
		Success bool         `json:"success"`
		Data    *models.Item `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodPUT, path, item, &updateResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if updateResp.Error != nil {
		return nil, fmt.Errorf("ошибка сервера: %s (%s)", updateResp.Error.Message, updateResp.Error.Code)
	}

	return updateResp.Data, nil
}

// fetchItem fetches a single item from the server using the retryable HTTP client.
func fetchItem(ctx context.Context, serverAddr, token, itemID string) (*models.Item, error) {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	path := api.PathItems + "/" + itemID

	var getResp struct {
		Success bool         `json:"success"`
		Data    *models.Item `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodGET, path, nil, &getResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	if getResp.Error != nil {
		return nil, fmt.Errorf("ошибка сервера: %s (%s)", getResp.Error.Message, getResp.Error.Code)
	}

	if !getResp.Success || getResp.Data == nil {
		return nil, fmt.Errorf("не удалось получить элемент данных")
	}

	return getResp.Data, nil
}

// deleteItem sends a soft delete request to the server using the retryable HTTP client.
func deleteItem(ctx context.Context, serverAddr, token, itemID string) error {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	path := api.PathItems + "/" + itemID

	var deleteResp struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodDELETE, path, nil, &deleteResp); err != nil {
		return fmt.Errorf("ошибка запроса: %w", err)
	}

	if deleteResp.Error != nil {
		return fmt.Errorf("ошибка сервера: %s (%s)", deleteResp.Error.Message, deleteResp.Error.Code)
	}

	if !deleteResp.Success {
		return fmt.Errorf("не удалось удалить элемент данных")
	}

	return nil
}

// purgeItem sends a hard delete request to permanently remove an item using the retryable HTTP client.
func purgeItem(ctx context.Context, serverAddr, token, itemID string) error {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	path := api.PathItems + "/" + itemID + "/purge"

	var purgeResp struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := client.ParseJSONResponse(ctx, api.MethodDELETE, path, nil, &purgeResp); err != nil {
		return fmt.Errorf("ошибка запроса: %w", err)
	}

	if purgeResp.Error != nil {
		return fmt.Errorf("ошибка сервера: %s (%s)", purgeResp.Error.Message, purgeResp.Error.Code)
	}

	return nil
}
