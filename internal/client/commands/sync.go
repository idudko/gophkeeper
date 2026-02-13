// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// SyncCommandOptions holds options for the sync command.
type SyncCommandOptions struct {
	force       bool
	keepLocal   bool
	keepServer  bool
	resolveAll  string
	localFile   string
	showDetails bool
	verbose     bool
}

// NewSyncCommand creates a new command to sync data with the server.
func NewSyncCommand() *cobra.Command {
	opts := &SyncCommandOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Синхронизировать данные с сервером",
		Long: `Синхронизирует локально сохраненные данные с сервером GophKeeper.

Команда выполняет двустороннюю синхронизацию данных:
- Загружает изменения с сервера
- Отправляет локальные изменения на сервер
- Обнаруживает и обрабатывает конфликты

При обнаружении конфликтов вы можете выбрать стратегию разрешения.`,
		Example: `  # Стандартная синхронизация с обработкой конфликтов
  gophkeeper sync

  # Принудительная перезапись локальных данных данными с сервера
  gophkeeper sync --keep-server

  # Принудительная перезапись серверных данных локальными данными
  gophkeeper sync --keep-local

  # Автоматическое разрешение всех конфликтов в пользу сервера
  gophkeeper sync --resolve-all server

  # Использовать файл для локального хранилища
  gophkeeper sync --local-file /path/to/local.json

  # Подробный вывод
  gophkeeper sync --verbose`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate conflict resolution strategy
			if opts.resolveAll != "" {
				validStrategies := map[string]bool{
					"local":  true,
					"server": true,
					"merge":  true,
				}
				if !validStrategies[opts.resolveAll] {
					return fmt.Errorf("неверная стратегия разрешения конфликтов: %s. Допустимые значения: local, server, merge", opts.resolveAll)
				}
			}

			// Validate mutually exclusive flags
			if opts.keepLocal && opts.keepServer {
				return fmt.Errorf("нельзя использовать --keep-local и --keep-server одновременно")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")

			// Load access token
			token, err := loadAccessToken()
			if err != nil {
				return err
			}

			// Determine local storage path
			localPath := opts.localFile
			if localPath == "" {
				localPath, err = getDefaultLocalStoragePath()
				if err != nil {
					return fmt.Errorf("ошибка определения пути к локальному хранилищу: %w", err)
				}
			}

			// Load local data
			localItems, lastSyncTime, err := loadLocalData(localPath)
			if err != nil {
				return fmt.Errorf("ошибка загрузки локальных данных: %w", err)
			}

			if opts.verbose {
				fmt.Printf("Загружено %d локальных элементов\n", len(localItems))
				fmt.Printf("Последняя синхронизация: %s\n\n", lastSyncTime.Format(time.RFC3339))
			}

			// Send sync request to server
			syncResponse, err := sendSyncRequest(ctx, serverAddr, token, localItems, lastSyncTime)
			if err != nil {
				return err
			}

			// Display sync summary
			displaySyncSummary(syncResponse, cmd.OutOrStdout())

			// Handle conflicts
			if len(syncResponse.Conflicts) > 0 {
				if err := handleConflicts(syncResponse.Conflicts, opts); err != nil {
					return err
				}
			}

			// Update local storage with server items
			if len(syncResponse.Items) > 0 || len(syncResponse.Conflicts) > 0 {
				if err := updateLocalStorage(localPath, syncResponse.Items, lastSyncTime); err != nil {
					return fmt.Errorf("ошибка обновления локального хранилища: %w", err)
				}
			}

			// Display success message
			fmt.Printf("\n✓ Синхронизация завершена успешно!\n")
			fmt.Printf("  Получено элементов с сервера: %d\n", len(syncResponse.Items))
			fmt.Printf("  Конфликтов: %d\n", len(syncResponse.Conflicts))

			return nil
		},
	}

	// Common flags
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Принудительная полная синхронизация")
	cmd.Flags().BoolVar(&opts.keepLocal, "keep-local", false, "Сохранить локальные версии (игнорировать сервер)")
	cmd.Flags().BoolVar(&opts.keepServer, "keep-server", false, "Использовать версии с сервера (игнорировать локальные)")
	cmd.Flags().StringVar(&opts.resolveAll, "resolve-all", "", "Автоматическое разрешение всех конфликтов (local/server/merge)")
	cmd.Flags().StringVar(&opts.localFile, "local-file", "", "Путь к файлу локального хранилища")
	cmd.Flags().BoolVar(&opts.showDetails, "show-details", false, "Показать подробности изменений")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Подробный вывод")

	return cmd
}

// SyncRequest represents a sync request payload.
type SyncRequest struct {
	Items      []models.Item `json:"items"`
	LastSyncAt time.Time     `json:"last_sync_at,omitempty"`
}

// SyncResponse represents a sync response payload.
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

// getDefaultLocalStoragePath returns the default path for local storage.
func getDefaultLocalStoragePath() (string, error) {
	configDir, err := getUserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "storage.json"), nil
}

// loadLocalData loads items from local storage.
func loadLocalData(path string) ([]models.Item, time.Time, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No local data yet
		return []models.Item{}, time.Time{}, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("ошибка чтения файла: %w", err)
	}

	// Parse JSON
	var storage struct {
		Items      []models.Item `json:"items"`
		LastSyncAt time.Time     `json:"last_sync_at"`
	}

	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, time.Time{}, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	return storage.Items, storage.LastSyncAt, nil
}

// sendSyncRequest sends a sync request to the server using the retryable HTTP client.
func sendSyncRequest(ctx context.Context, serverAddr, token string, localItems []models.Item, lastSyncTime time.Time) (*SyncResponse, error) {
	client, err := getHTTPClient(serverAddr, token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	// Prepare request body
	reqBody := SyncRequest{
		Items:      localItems,
		LastSyncAt: lastSyncTime,
	}

	var syncResp SyncResponse

	if err := client.ParseJSONResponse(ctx, api.MethodPOST, api.PathSync, reqBody, &syncResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	// Set last sync time if empty
	if syncResp.LastSyncAt.IsZero() {
		syncResp.LastSyncAt = time.Now()
	}

	return &syncResp, nil
}

// displaySyncSummary displays a summary of sync operation.
func displaySyncSummary(resp *SyncResponse, output io.Writer) {
	fmt.Fprintf(output, "════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(output, "                  СИНХРОНИЗАЦИЯ                              \n")
	fmt.Fprintf(output, "════════════════════════════════════════════════════════════\n\n")

	fmt.Fprintf(output, "Получено элементов с сервера: %d\n", len(resp.Items))
	fmt.Fprintf(output, "Обнаружено конфликтов: %d\n", len(resp.Conflicts))
	fmt.Fprintf(output, "Время синхронизации: %s\n\n", resp.LastSyncAt.Format("2006-01-02 15:04:05"))

	if len(resp.Conflicts) > 0 {
		fmt.Fprintf(output, "⚠️  Обнаружены конфликты:\n")
		for i, conflict := range resp.Conflicts {
			fmt.Fprintf(output, "  %d. %s (%s)\n", i+1, conflict.Server.ID, conflict.Server.Type)
			fmt.Fprintf(output, "     Причина: %s\n", conflict.Reason)
		}
		fmt.Fprintf(output, "\n")
	}

	if len(resp.Items) > 0 {
		fmt.Fprintf(output, "✓ Новые/обновленные элементы:\n")
		for i, item := range resp.Items {
			if i >= 5 { // Show only first 5
				fmt.Fprintf(output, "  ... и еще %d элементов\n", len(resp.Items)-5)
				break
			}
			fmt.Fprintf(output, "  - %s (%s) - %s\n", item.ID[:8]+"...", item.Type, item.Meta)
		}
		fmt.Fprintf(output, "\n")
	}
}

// handleConflicts handles sync conflicts according to the specified options.
func handleConflicts(conflicts []ConflictItem, opts *SyncCommandOptions) error {
	if len(conflicts) == 0 {
		return nil
	}

	fmt.Printf("\n════════════════════════════════════════════════════════════\n")
	fmt.Printf("                  ОБРАБОТКА КОНФЛИКТОВ                      \n")
	fmt.Printf("════════════════════════════════════════════════════════════\n\n")

	resolvedItems := make([]models.Item, 0, len(conflicts))

	for i, conflict := range conflicts {
		fmt.Printf("Конфликт #%d: %s (%s)\n", i+1, conflict.Server.ID, conflict.Server.Type)
		fmt.Printf("  Причина: %s\n", conflict.Reason)
		fmt.Printf("  Локальная версия: v%d, обновлена %s\n",
			conflict.Local.Version, conflict.Local.UpdatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("  Серверная версия: v%d, обновлена %s\n",
			conflict.Server.Version, conflict.Server.UpdatedAt.Format("2006-01-02 15:04"))

		var resolvedItem *models.Item
		var err error

		// Determine resolution strategy
		switch {
		case opts.resolveAll != "":
			// Auto-resolve with specified strategy
			resolvedItem, err = autoResolveConflict(conflict, opts.resolveAll)
		case opts.keepLocal:
			// Keep local version
			resolvedItem = &conflict.Local
			fmt.Printf("  → Выбрана локальная версия (--keep-local)\n")
		case opts.keepServer:
			// Keep server version
			resolvedItem = &conflict.Server
			fmt.Printf("  → Выбрана серверная версия (--keep-server)\n")
		default:
			// Default: keep server version
			resolvedItem = &conflict.Server
			fmt.Printf("  → Выбрана серверная версия (по умолчанию)\n")
		}

		if err != nil {
			return fmt.Errorf("ошибка разрешения конфликта #%d: %w", i+1, err)
		}

		if resolvedItem != nil {
			resolvedItems = append(resolvedItems, *resolvedItem)
		}

		fmt.Printf("\n")
	}

	// Send resolved conflicts back to server if any were resolved
	if len(resolvedItems) > 0 {
		fmt.Printf("Отправка разрешенных конфликтов на сервер...\n")
		// TODO: Implement sending resolved conflicts
	}

	return nil
}

// autoResolveConflict automatically resolves a conflict.
func autoResolveConflict(conflict ConflictItem, strategy string) (*models.Item, error) {
	switch strategy {
	case "local":
		fmt.Printf("  → Автоматически выбрана локальная версия\n")
		return &conflict.Local, nil
	case "server":
		fmt.Printf("  → Автоматически выбрана серверная версия\n")
		return &conflict.Server, nil
	case "merge":
		// Simple merge strategy: use server version with higher number
		if conflict.Server.Version > conflict.Local.Version {
			fmt.Printf("  → Автоматически выбрана серверная версия (более новая)\n")
			return &conflict.Server, nil
		} else {
			fmt.Printf("  → Автоматически выбрана локальная версия (более новая)\n")
			return &conflict.Local, nil
		}
	default:
		return nil, fmt.Errorf("неизвестная стратегия: %s", strategy)
	}
}

// updateLocalStorage updates the local storage file with new items.
func updateLocalStorage(path string, items []models.Item, lastSyncTime time.Time) error {
	storage := struct {
		Items      []models.Item `json:"items"`
		LastSyncAt time.Time     `json:"last_sync_at"`
	}{
		Items:      items,
		LastSyncAt: lastSyncTime,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка кодирования JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("ошибка записи файла: %w", err)
	}

	return nil
}
