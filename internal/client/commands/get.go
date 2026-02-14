// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// GetCommandOptions holds options for get command.
type GetCommandOptions struct {
	outputFile string
	masterKey  string
	format     string
	decrypt    bool
	showSecret bool
}

// NewGetCommand creates a new command to get stored data.
func NewGetCommand() *cobra.Command {
	opts := &GetCommandOptions{}

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Получить и отобразить сохраненные данные",
		Long: `Получает данные из защищенного хранилища GophKeeper по их ID.

Поддерживает дешифрование данных на клиентской стороне, если они были зашифрованы при сохранении.
Для бинарных данных можно указать путь для сохранения файла.`,
		Example: `  # Получить данные по ID
  gophkeeper get 550e8400-e29b-41d4-a716-446655440000

  # Получить и расшифровать данные
  gophkeeper get 550e8400-e29b-41d4-a716-446655440000 --decrypt

  # Получить бинарные данные и сохранить в файл
  gophkeeper get 550e8400-e29b-41d4-a716-446655440000 --output /path/to/save/file.bin

  # Получить данные в формате JSON
  gophkeeper get 550e8400-e29b-41d4-a716-446655440000 --format json

  # Показать чувствительные данные (пароли, CVV)
  gophkeeper get 550e8400-e29b-41d4-a716-446655440000 --show-secret`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate format
			validFormats := map[string]bool{
				"pretty": true,
				"json":   true,
			}
			if !validFormats[opts.format] {
				return fmt.Errorf("неверный формат вывода: %s. Допустимые значения: pretty, json", opts.format)
			}

			// If decryption is enabled, master key is required
			if opts.decrypt && opts.masterKey == "" {
				return fmt.Errorf("для дешифрования требуется мастер-ключ. Используйте --master-key")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")
			itemID := args[0]

			// Load access token
			token, err := loadAccessToken()
			if err != nil {
				return err
			}

			// Fetch item from server using retryable HTTP client
			item, err := fetchItem(ctx, serverAddr, token, itemID)
			if err != nil {
				return err
			}

			// Decrypt data if requested
			if opts.decrypt {
				// Load master password and encryption salt
				masterPassword, encryptionSalt, err := loadEncryptionCredentials(opts.masterKey)
				if err != nil {
					return fmt.Errorf("ошибка загрузки учётных данных шифрования: %w", err)
				}

				// Decrypt item data using KeyManager
				if err := decryptItemData(item, masterPassword, encryptionSalt); err != nil {
					return fmt.Errorf("ошибка дешифрования данных: %w", err)
				}
			}

			// Handle binary data output
			if item.Type == models.DataTypeBinary && opts.outputFile != "" {
				return saveBinaryData(item, opts.outputFile)
			}

			// Display item based on format
			switch opts.format {
			case "json":
				displayItemJSON(item, cmd.OutOrStdout())
			default: // pretty
				displayItemPretty(item, opts.showSecret, cmd.OutOrStdout())
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&opts.outputFile, "output", "o", "", "Путь для сохранения бинарных данных")
	cmd.Flags().StringVar(&opts.masterKey, "master-key", "", "Мастер-ключ для дешифрования")
	cmd.Flags().StringVar(&opts.format, "format", "pretty", "Формат вывода (pretty, json)")
	cmd.Flags().BoolVar(&opts.decrypt, "decrypt", true, "Расшифровать данные на клиентской стороне")
	cmd.Flags().BoolVar(&opts.showSecret, "show-secret", false, "Показать чувствительные данные (пароли, CVV)")

	return cmd
}

// saveBinaryData saves binary data to a file.
func saveBinaryData(item *models.Item, outputPath string) error {
	if item.Binary == nil || len(item.Binary.Data) == 0 {
		return fmt.Errorf("нет бинарных данных для сохранения")
	}

	// If output path is a directory, use original filename
	if info, err := os.Stat(outputPath); err == nil && info.IsDir() {
		outputPath = outputPath + string(os.PathSeparator) + item.Binary.Filename
	}

	// Write file
	if err := os.WriteFile(outputPath, item.Binary.Data, 0644); err != nil {
		return fmt.Errorf("ошибка сохранения файла: %w", err)
	}

	fmt.Printf("✓ Бинарные данные сохранены в: %s\n", outputPath)
	fmt.Printf("  Размер: %d байт\n", len(item.Binary.Data))

	return nil
}

// displayItemPretty displays item in a pretty human-readable format.
func displayItemPretty(item *models.Item, showSecret bool, output io.Writer) {
	fmt.Fprintf(output, "╔════════════════════════════════════════════════════════╗\n")
	fmt.Fprintf(output, "║                    GophKeeper Data                           ║\n")
	fmt.Fprintf(output, "╚══════════════════════════════════════════════════════════╝\n\n")

	fmt.Fprintf(output, "ID:           %s\n", item.ID)
	fmt.Fprintf(output, "Тип:          %s %s\n", getTypeSymbol(item.Type), item.Type)
	fmt.Fprintf(output, "Описание:     %s\n", item.Meta)
	fmt.Fprintf(output, "Версия:       %d\n", item.Version)
	fmt.Fprintf(output, "Создано:      %s\n", item.CreatedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(output, "Обновлено:    %s\n", item.UpdatedAt.Format("2006-01-02 15:04:05 MST"))

	switch item.Type {
	case models.DataTypePassword:
		if item.Password != nil {
			fmt.Fprintf(output, "\n🔑 Данные для входа:\n")
			fmt.Fprintf(output, "  Логин:       %s\n", item.Password.Login)
			if showSecret {
				fmt.Fprintf(output, "  Пароль:       %s\n", item.Password.Password)
			} else {
				fmt.Fprintf(output, "  Пароль:       *** (используйте --show-secret)\n")
			}
			if item.Password.Meta != "" {
				fmt.Fprintf(output, "  Описание:     %s\n", item.Password.Meta)
			}
		}
	case models.DataTypeText:
		if item.Text != nil {
			fmt.Fprintf(output, "\n📝 Текстовые данные:\n")
			if item.Text.Meta != "" {
				fmt.Fprintf(output, "  Описание:     %s\n", item.Text.Meta)
			}
			fmt.Fprintf(output, "  Текст:\n")
			fmt.Fprintf(output, "    %s\n", item.Text.Data)
		}
	case models.DataTypeBinary:
		if item.Binary != nil {
			fmt.Fprintf(output, "\n📦 Бинарные данные:\n")
			if item.Binary.Meta != "" {
				fmt.Fprintf(output, "  Описание:     %s\n", item.Binary.Meta)
			}
			fmt.Fprintf(output, "  Имя файла:    %s\n", item.Binary.Filename)
			fmt.Fprintf(output, "  Размер:        %d байт\n", len(item.Binary.Data))
		}
	case models.DataTypeCard:
		if item.Card != nil {
			fmt.Fprintf(output, "\n💳 Данные карты:\n")
			if item.Card.Meta != "" {
				fmt.Fprintf(output, "  Описание:     %s\n", item.Card.Meta)
			}
			fmt.Fprintf(output, "  Номер карты: %s\n", maskCardNumber(item.Card.Number))
			fmt.Fprintf(output, "  Владелец:     %s\n", item.Card.HolderName)
			fmt.Fprintf(output, "  Срок действия: %s\n", item.Card.ExpiryDate)
			if showSecret {
				fmt.Fprintf(output, "  CVV:          %s\n", item.Card.CVV)
			} else {
				fmt.Fprintf(output, "  CVV:          *** (используйте --show-secret)\n")
			}
		}
	}

	fmt.Fprintf(output, "\n")
}

// displayItemJSON displays item in JSON format.
func displayItemJSON(item *models.Item, output io.Writer) {
	fmt.Fprintf(output, "{\n")
	fmt.Fprintf(output, "  \"id\": \"%s\",\n", item.ID)
	fmt.Fprintf(output, "  \"type\": \"%s\",\n", item.Type)
	fmt.Fprintf(output, "  \"version\": %d,\n", item.Version)
	fmt.Fprintf(output, "  \"meta\": %q,\n", item.Meta)
	fmt.Fprintf(output, "  \"created_at\": \"%s\",\n", item.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(output, "  \"updated_at\": \"%s\",\n", item.UpdatedAt.Format(time.RFC3339))

	switch item.Type {
	case models.DataTypePassword:
		fmt.Fprintf(output, "  \"password\": {\n")
		fmt.Fprintf(output, "    \"login\": %q,\n", item.Password.Login)
		fmt.Fprintf(output, "    \"password\": %q,\n", item.Password.Password)
		if item.Password.Meta != "" {
			fmt.Fprintf(output, "    \"meta\": %q\n", item.Password.Meta)
		}
		fmt.Fprintf(output, "  }\n")
	case models.DataTypeText:
		fmt.Fprintf(output, "  \"text\": {\n")
		fmt.Fprintf(output, "    \"data\": %q,\n", item.Text.Data)
		if item.Text.Meta != "" {
			fmt.Fprintf(output, "    \"meta\": %q\n", item.Text.Meta)
		}
		fmt.Fprintf(output, "  }\n")
	case models.DataTypeBinary:
		fmt.Fprintf(output, "  \"binary\": {\n")
		fmt.Fprintf(output, "    \"filename\": %q,\n", item.Binary.Filename)
		fmt.Fprintf(output, "    \"size\": %d\n", len(item.Binary.Data))
		if item.Binary.Meta != "" {
			fmt.Fprintf(output, "    \"meta\": %q\n", item.Binary.Meta)
		}
		fmt.Fprintf(output, "  }\n")
	case models.DataTypeCard:
		fmt.Fprintf(output, "  \"card\": {\n")
		fmt.Fprintf(output, "    \"number\": %q,\n", item.Card.Number)
		fmt.Fprintf(output, "    \"holder_name\": %q,\n", item.Card.HolderName)
		fmt.Fprintf(output, "    \"expiry_date\": %q,\n", item.Card.ExpiryDate)
		fmt.Fprintf(output, "    \"cvv\": %q\n", item.Card.CVV)
		if item.Card.Meta != "" {
			fmt.Fprintf(output, "    \"meta\": %q\n", item.Card.Meta)
		}
		fmt.Fprintf(output, "  }\n")
	}

	fmt.Fprintf(output, "}\n")
}
