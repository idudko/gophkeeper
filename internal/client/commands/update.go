// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	clientcrypto "github.com/idudko/gophkeeper/internal/client/crypto"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// UpdateCommandOptions holds options for the update command.
type UpdateCommandOptions struct {
	dataType   string
	meta       string
	login      string
	password   string
	text       string
	filePath   string
	cardNumber string
	cardHolder string
	cardExpiry string
	cardCVV    string
	encrypt    bool
	masterKey  string // Used if --master-key is provided
	force      bool
}

// NewUpdateCommand creates a new command to update existing data.
func NewUpdateCommand() *cobra.Command {
	opts := &UpdateCommandOptions{}

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Обновить существующие данные в хранилище",
		Long: `Обновляет существующие данные в защищенном хранилище GophKeeper по их ID.

Поддерживает обновление всех типов данных:
  - password: пары логин/пароль
  - text: произвольные текстовые данные
  - binary: произвольные бинарные данные (файлы)
  - card: данные банковских карт

Данные могут быть зашифрованы на клиентской стороне перед отправкой на сервер.`,
		Example: `  # Обновить пароль
  gophkeeper update 550e8400-e29b-41d4-a716-446655440000 --login newuser --password newpass

  # Обновить описание
  gophkeeper update 550e8400-e29b-41d4-a716-446655440000 --meta "Новое описание"

  # Обновить карту
  gophkeeper update 550e8400-e29b-41d4-a716-446655440000 --expiry 12/26

  # Принудительное обновление (игнорировать конфликты версий)
  gophkeeper update 550e8400-e29b-41d4-a716-446655440000 --force`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// If encryption is enabled, master key is required
			if opts.encrypt && opts.masterKey == "" {
				return fmt.Errorf("для шифрования требуется мастер-ключ. Используйте --master-key")
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

			// Load master password and encryption salt for client-side encryption
			masterPassword, encryptionSalt, err := loadEncryptionCredentials(opts.masterKey)
			if err != nil {
				return fmt.Errorf("ошибка загрузки учётных данных шифрования: %w", err)
			}

			// Fetch existing item from server using retryable HTTP client
			existingItem, err := fetchItem(ctx, serverAddr, token, itemID)
			if err != nil {
				return err
			}

			// Create updated item from options
			updatedItem, err := createUpdatedItem(existingItem, opts)
			if err != nil {
				return fmt.Errorf("ошибка создания обновленного элемента: %w", err)
			}

			// Encrypt data on client-side using master password and server salt
			keyManager := clientcrypto.NewKeyManager(masterPassword, encryptionSalt)

			// Convert models.Item to clientcrypto.Item for encryption
			cryptoItem := convertToCryptoItem(updatedItem)

			if err := keyManager.EncryptItem(cryptoItem); err != nil {
				return fmt.Errorf("ошибка шифрования данных на клиенте: %w", err)
			}

			// Convert back to models.Item for sending to server
			convertFromCryptoItem(cryptoItem, updatedItem)

			// Send update request to server using retryable HTTP client
			resultItem, err := sendUpdateItemRequestInternal(ctx, serverAddr, token, itemID, updatedItem, false)
			if err != nil {
				return err
			}

			// Display success message
			fmt.Printf("✓ Данные успешно обновлены!\n")
			fmt.Printf("  ID: %s\n", resultItem.ID)
			fmt.Printf("  Тип: %s\n", resultItem.Type)
			fmt.Printf("  Версия: %d\n", resultItem.Version)
			if opts.meta != "" && resultItem.Meta != existingItem.Meta {
				fmt.Printf("  Новое описание: %s\n", resultItem.Meta)
			}
			if opts.encrypt {
				fmt.Printf("  Шифрование: клиентское (ChaCha20-Poly1305)\n")
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&opts.dataType, "type", "t", "", "Тип данных (password, text, binary, card)")
	cmd.Flags().StringVar(&opts.meta, "meta", "", "Метаинформация (описание)")

	// Encryption flags
	cmd.Flags().BoolVar(&opts.encrypt, "encrypt", false, "Зашифровать данные на клиентской стороне")
	cmd.Flags().StringVar(&opts.masterKey, "master-key", "", "Мастер-ключ для шифрования")

	// Password type flags
	cmd.Flags().StringVar(&opts.login, "login", "", "Новый логин (для типа password)")
	cmd.Flags().StringVar(&opts.password, "password", "", "Новый пароль (для типа password)")

	// Text type flags
	cmd.Flags().StringVar(&opts.text, "text", "", "Новые текстовые данные (для типа text)")

	// Binary type flags
	cmd.Flags().StringVar(&opts.filePath, "file", "", "Путь к новому файлу (для типа binary)")

	// Card type flags
	cmd.Flags().StringVar(&opts.cardNumber, "number", "", "Новый номер карты (для типа card)")
	cmd.Flags().StringVar(&opts.cardHolder, "holder", "", "Новое имя владельца карты (для типа card)")
	cmd.Flags().StringVar(&opts.cardExpiry, "expiry", "", "Новый срок действия карты MM/YY (для типа card)")
	cmd.Flags().StringVar(&opts.cardCVV, "cvv", "", "Новый CVV код (для типа card)")

	// Force flag
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Принудительное обновление (игнорировать конфликты)")

	return cmd
}

// createUpdatedItem creates an updated item from existing item and options.
func createUpdatedItem(existingItem *models.Item, opts *UpdateCommandOptions) (*models.Item, error) {
	// Create a copy of the existing item
	updatedItem := *existingItem

	// Update meta if provided
	if opts.meta != "" {
		updatedItem.Meta = opts.meta
	}

	// Increment version for optimistic locking
	updatedItem.Version++
	updatedItem.UpdatedAt = time.Now()

	// Update type-specific data
	switch existingItem.Type {
	case models.DataTypePassword:
		// Preserve existing password data structure
		if existingItem.Password == nil {
			updatedItem.Password = &models.PasswordData{}
		} else {
			updatedItem.Password = &models.PasswordData{
				Meta:     existingItem.Password.Meta,
				Login:    existingItem.Password.Login,
				Password: existingItem.Password.Password,
			}
		}

		// Update fields if provided
		if opts.login != "" {
			updatedItem.Password.Login = opts.login
		}
		if opts.password != "" {
			updatedItem.Password.Password = opts.password
		}
		if opts.meta != "" {
			updatedItem.Password.Meta = opts.meta
		}

	case models.DataTypeText:
		// Preserve existing text data structure
		if existingItem.Text == nil {
			updatedItem.Text = &models.TextData{}
		} else {
			updatedItem.Text = &models.TextData{
				Meta: existingItem.Text.Meta,
				Data: existingItem.Text.Data,
			}
		}

		// Update fields if provided
		if opts.text != "" {
			updatedItem.Text.Data = opts.text
		}
		if opts.meta != "" {
			updatedItem.Text.Meta = opts.meta
		}

	case models.DataTypeBinary:
		// Preserve existing binary data structure
		if existingItem.Binary == nil {
			updatedItem.Binary = &models.BinaryData{}
		} else {
			updatedItem.Binary = &models.BinaryData{
				Meta:     existingItem.Binary.Meta,
				Filename: existingItem.Binary.Filename,
				Data:     existingItem.Binary.Data,
			}
		}

		// Update file if provided
		if opts.filePath != "" {
			data, err := os.ReadFile(opts.filePath)
			if err != nil {
				return nil, fmt.Errorf("ошибка чтения файла: %w", err)
			}
			updatedItem.Binary.Data = data
			updatedItem.Binary.Filename = filepath.Base(opts.filePath)
		}
		if opts.meta != "" {
			updatedItem.Binary.Meta = opts.meta
		}

	case models.DataTypeCard:
		// Preserve existing card data structure
		if existingItem.Card == nil {
			updatedItem.Card = &models.CardData{}
		} else {
			updatedItem.Card = &models.CardData{
				Meta:       existingItem.Card.Meta,
				Number:     existingItem.Card.Number,
				HolderName: existingItem.Card.HolderName,
				ExpiryDate: existingItem.Card.ExpiryDate,
				CVV:        existingItem.Card.CVV,
			}
		}

		// Update fields if provided
		if opts.cardNumber != "" {
			updatedItem.Card.Number = opts.cardNumber
		}
		if opts.cardHolder != "" {
			updatedItem.Card.HolderName = opts.cardHolder
		}
		if opts.cardExpiry != "" {
			updatedItem.Card.ExpiryDate = opts.cardExpiry
		}
		if opts.cardCVV != "" {
			updatedItem.Card.CVV = opts.cardCVV
		}
		if opts.meta != "" {
			updatedItem.Card.Meta = opts.meta
		}
	}

	return &updatedItem, nil
}
