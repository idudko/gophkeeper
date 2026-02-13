// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ListCommandOptions holds options for list command.
type ListCommandOptions struct {
	dataType string
	sortBy   string
	format   string
	verbose  bool
}

// NewListCommand creates a new command to list stored data.
func NewListCommand() *cobra.Command {
	opts := &ListCommandOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Отобразить список сохраненных данных",
		Long: `Отображает список всех сохраненных в хранилище GophKeeper данных.

Поддерживает фильтрацию по типу данных, сортировку и различные форматы вывода.`,
		Example: `  # Список всех данных
  gophkeeper list

  # Список только паролей
  gophkeeper list --type password

  # Список с сортировкой по дате обновления
  gophkeeper list --sort updated_at

  # Подробный список
  gophkeeper list --verbose

  # Формат вывода table или json
  gophkeeper list --format json`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate data type if specified
			if opts.dataType != "" {
				validTypes := map[string]bool{
					"password": true,
					"text":     true,
					"binary":   true,
					"card":     true,
				}
				if !validTypes[opts.dataType] {
					return fmt.Errorf("неверный тип данных: %s. Допустимые значения: password, text, binary, card", opts.dataType)
				}
			}

			// Validate sort field
			if opts.sortBy != "" {
				validSortFields := map[string]bool{
					"created_at": true,
					"updated_at": true,
					"type":       true,
					"meta":       true,
				}
				if !validSortFields[opts.sortBy] {
					return fmt.Errorf("неверное поле сортировки: %s. Допустимые значения: created_at, updated_at, type, meta", opts.sortBy)
				}
			}

			// Validate format
			validFormats := map[string]bool{
				"table": true,
				"json":  true,
				"wide":  true,
			}
			if !validFormats[opts.format] {
				return fmt.Errorf("неверный формат вывода: %s. Допустимые значения: table, json, wide", opts.format)
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

			// Fetch items from server
			items, err := fetchItems(ctx, serverAddr, token, opts.dataType)
			if err != nil {
				return err
			}

			// Sort items
			sortItems(items, opts.sortBy)

			// Display items based on format
			switch opts.format {
			case "json":
				displayItemsJSON(items, cmd.OutOrStdout())
			case "wide":
				displayItemsWide(items, opts.verbose, cmd.OutOrStdout())
			default: // table
				displayItemsTable(items, opts.verbose, cmd.OutOrStdout())
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&opts.dataType, "type", "t", "", "Фильтр по типу данных (password, text, binary, card)")
	cmd.Flags().StringVarP(&opts.sortBy, "sort", "", "updated_at", "Поле сортировки (created_at, updated_at, type, meta)")
	cmd.Flags().StringVarP(&opts.format, "format", "f", "table", "Формат вывода (table, json, wide)")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Подробный вывод")

	return cmd
}

// displayItemsTable displays items in table format.
func displayItemsTable(items []models.Item, verbose bool, output io.Writer) {
	if len(items) == 0 {
		fmt.Fprintln(output, "Нет сохраненных данных.")
		return
	}

	fmt.Fprintf(output, "Всего записей: %d\n\n", len(items))

	// Print header
	fmt.Fprintf(output, "%-36s %-10s %-20s %-20s\n", "ID", "ТИП", "ОПИСАНИЕ", "ОБНОВЛЕНО")
	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 90))

	// Print each item
	for _, item := range items {
		typeSymbol := getTypeSymbol(item.Type)
		meta := truncateString(item.Meta, 20)
		updatedAt := item.UpdatedAt.Format("2006-01-02 15:04")

		fmt.Fprintf(output, "%-36s %-2s %-7s %-20s %s\n",
			item.ID[:8]+"...",
			typeSymbol,
			string(item.Type),
			meta,
			updatedAt,
		)
	}

	fmt.Fprintf(output, "\nЛегенда типов:\n")
	fmt.Fprintf(output, "  🔑 - Пароль (login/password)\n")
	fmt.Fprintf(output, "  📝 - Текст\n")
	fmt.Fprintf(output, "  📦 - Бинарные данные (файлы)\n")
	fmt.Fprintf(output, "  💳 - Банковская карта\n")
}

// displayItemsWide displays items in wide format with more details.
func displayItemsWide(items []models.Item, verbose bool, output io.Writer) {
	if len(items) == 0 {
		fmt.Fprintln(output, "Нет сохраненных данных.")
		return
	}

	fmt.Fprintf(output, "Всего записей: %d\n\n", len(items))

	// Print header
	fmt.Fprintf(output, "%-36s %-10s %-30s %-20s %-20s\n", "ID", "ТИП", "ОПИСАНИЕ", "СОЗДАНО", "ОБНОВЛЕНО")
	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 140))

	// Print each item
	for _, item := range items {
		typeSymbol := getTypeSymbol(item.Type)
		meta := truncateString(item.Meta, 28)
		createdAt := item.CreatedAt.Format("2006-01-02 15:04")
		updatedAt := item.UpdatedAt.Format("2006-01-02 15:04")

		fmt.Fprintf(output, "%-36s %-2s %-7s %-30s %-20s %s\n",
			item.ID,
			typeSymbol,
			string(item.Type),
			meta,
			createdAt,
			updatedAt,
		)

		// Display additional details based on type
		if verbose {
			displayItemDetails(&item, "  ", output)
		}
	}
}

// displayItemsJSON displays items in JSON format.
func displayItemsJSON(items []models.Item, output io.Writer) {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	data := map[string]any{
		"success": true,
		"data":    items,
		"total":   len(items),
	}

	if err := encoder.Encode(data); err != nil {
		fmt.Fprintf(output, "Ошибка кодирования JSON: %v\n", err)
	}
}

// displayItemDetails displays detailed information about an item.
func displayItemDetails(item *models.Item, indent string, output io.Writer) {
	switch item.Type {
	case models.DataTypePassword:
		if item.Password != nil {
			fmt.Fprintf(output, "%sЛогин: %s\n", indent, item.Password.Login)
			fmt.Fprintf(output, "%sПароль: ********\n", indent)
		}
	case models.DataTypeText:
		if item.Text != nil {
			text := truncateString(item.Text.Data, 60)
			fmt.Fprintf(output, "%sТекст: %s\n", indent, text)
		}
	case models.DataTypeBinary:
		if item.Binary != nil {
			fmt.Fprintf(output, "%sФайл: %s\n", indent, item.Binary.Filename)
			fmt.Fprintf(output, "%sРазмер: %d байт\n", indent, len(item.Binary.Data))
		}
	case models.DataTypeCard:
		if item.Card != nil {
			cardNumber := maskCardNumber(item.Card.Number)
			fmt.Fprintf(output, "%sКарта: %s\n", indent, cardNumber)
			fmt.Fprintf(output, "%sВладелец: %s\n", indent, item.Card.HolderName)
			fmt.Fprintf(output, "%sСрок: %s\n", indent, item.Card.ExpiryDate)
		}
	}
	fmt.Fprintln(output)
}

// truncateString truncates a string to a maximum length.
