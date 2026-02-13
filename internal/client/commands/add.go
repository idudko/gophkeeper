// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	clientcrypto "github.com/idudko/gophkeeper/internal/client/crypto"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddCommandOptions holds options for the add command.
type AddCommandOptions struct {
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
}

// NewAddCommand creates a new command to add data to the storage.
func NewAddCommand() *cobra.Command {
	opts := &AddCommandOptions{}

	cmd := &cobra.Command{
		Use:   "add [type]",
		Short: "Добавить новые данные в хранилище",
		Long: `Добавляет новые данные в защищенное хранилище GophKeeper.

Поддерживаются следующие типы данных:
  - password: пары логин/пароль
  - text: произвольные текстовые данные
  - binary: произвольные бинарные данные (файлы)
  - card: данные банковских карт

Для всех типов данных можно добавить метаинформацию (описание, теги и т.д.).

Данные шифруются на клиентской стороне перед отправкой на сервер.`,
		Example: `  # Добавить пароль
  gophkeeper add password --meta "GitHub" --login myuser --password mypass

  # Добавить текстовые данные
  gophkeeper add text --meta "Заметка" --text "Важная информация"

  # Добавить файл
  gophkeeper add binary --meta "Фото" --file /path/to/photo.jpg

  # Добавить карту
  gophkeeper add card --meta "Личная карта" --number 1234567890123456 --holder "Ivan Ivanov" --expiry 12/25 --cvv 123`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.dataType = args[0]

			// Validate data type
			validTypes := map[string]bool{
				"password": true,
				"text":     true,
				"binary":   true,
				"card":     true,
			}

			if !validTypes[opts.dataType] {
				return fmt.Errorf("неверный тип данных: %s. Допустимые значения: password, text, binary, card", opts.dataType)
			}

			// Default to encrypt with client-side encryption
			opts.encrypt = true

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")

			// Load access token
			token, err := loadAccessToken()
			if err != nil {
				return fmt.Errorf("ошибка загрузки токена: %w", err)
			}

			// Load master password and encryption salt for client-side encryption
			masterPassword, encryptionSalt, err := loadEncryptionCredentials(opts.masterKey)
			if err != nil {
				return fmt.Errorf("ошибка загрузки учётных данных шифрования: %w", err)
			}

			// Validate data based on type
			if err := validateItemData(opts); err != nil {
				return err
			}

			// Create item based on type
			item, err := createItemFromOptions(opts)
			if err != nil {
				return fmt.Errorf("ошибка создания элемента: %w", err)
			}

			// Encrypt data on client-side using master password and server salt
			keyManager := clientcrypto.NewKeyManager(masterPassword, encryptionSalt)

			// Convert models.Item to clientcrypto.Item for encryption
			cryptoItem := convertToCryptoItem(item)

			if err := keyManager.EncryptItem(cryptoItem); err != nil {
				return fmt.Errorf("ошибка шифрования данных на клиенте: %w", err)
			}

			// Convert back to models.Item for sending to server
			convertFromCryptoItem(cryptoItem, item)

			// Send request to server
			createdItem, err := sendCreateItemRequest(ctx, serverAddr, token, item)
			if err != nil {
				return err
			}

			// Display success message
			fmt.Printf("✓ Данные успешно добавлены!\n")
			fmt.Printf("  ID: %s\n", createdItem.ID)
			fmt.Printf("  Тип: %s\n", createdItem.Type)
			if opts.meta != "" {
				fmt.Printf("  Описание: %s\n", opts.meta)
			}
			fmt.Printf("  Шифрование: клиентское (ChaCha20-Poly1305, Argon2id)\n")

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVar(&opts.meta, "meta", "", "Метаинформация (описание, теги)")
	cmd.Flags().StringVar(&opts.masterKey, "master-key", "", "Мастер-пароль (по умолчанию загружается из локального хранилища)")

	// Password type flags
	cmd.Flags().StringVar(&opts.login, "login", "", "Логин (для типа password)")
	cmd.Flags().StringVar(&opts.password, "password", "", "Пароль (для типа password)")

	// Text type flags
	cmd.Flags().StringVar(&opts.text, "text", "", "Текстовые данные (для типа text)")

	// Binary type flags
	cmd.Flags().StringVar(&opts.filePath, "file", "", "Путь к файлу (для типа binary)")

	// Card type flags
	cmd.Flags().StringVar(&opts.cardNumber, "number", "", "Номер карты (для типа card)")
	cmd.Flags().StringVar(&opts.cardHolder, "holder", "", "Имя владельца карты (для типа card)")
	cmd.Flags().StringVar(&opts.cardExpiry, "expiry", "", "Срок действия карты MM/YY (для типа card)")
	cmd.Flags().StringVar(&opts.cardCVV, "cvv", "", "CVV код (для типа card)")

	return cmd
}

// createItemFromOptions creates an Item struct from command options.
func createItemFromOptions(opts *AddCommandOptions) (*models.Item, error) {
	item := &models.Item{
		ID:        uuid.New().String(),
		Type:      models.DataType(opts.dataType),
		Version:   1,
		Meta:      opts.meta,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	switch opts.dataType {
	case string(models.DataTypePassword):
		item.Password = &models.PasswordData{
			Meta:     opts.meta,
			Login:    opts.login,
			Password: opts.password,
		}
	case string(models.DataTypeText):
		item.Text = &models.TextData{
			Meta: opts.meta,
			Data: opts.text,
		}
	case string(models.DataTypeBinary):
		data, err := os.ReadFile(opts.filePath)
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения файла: %w", err)
		}
		item.Binary = &models.BinaryData{
			Meta:     opts.meta,
			Filename: filepath.Base(opts.filePath),
			Data:     data,
		}
	case string(models.DataTypeCard):
		item.Card = &models.CardData{
			Meta:       opts.meta,
			Number:     opts.cardNumber,
			HolderName: opts.cardHolder,
			ExpiryDate: opts.cardExpiry,
			CVV:        opts.cardCVV,
		}
	}

	return item, nil
}

// validateItemData validates the item data based on type.
func validateItemData(opts *AddCommandOptions) error {
	switch opts.dataType {
	case "password":
		if opts.login == "" {
			return fmt.Errorf("логин обязателен для типа password")
		}
		if opts.password == "" {
			return fmt.Errorf("пароль обязателен для типа password")
		}
	case "text":
		if opts.text == "" {
			return fmt.Errorf("текст обязателен для типа text")
		}
	case "binary":
		if opts.filePath == "" {
			return fmt.Errorf("путь к файлу обязателен для типа binary")
		}
		if _, err := os.Stat(opts.filePath); os.IsNotExist(err) {
			return fmt.Errorf("файл не найден: %s", opts.filePath)
		}
	case "card":
		if opts.cardNumber == "" {
			return fmt.Errorf("номер карты обязателен для типа card")
		}
		if opts.cardExpiry == "" {
			return fmt.Errorf("срок действия карты обязателен для типа card")
		}
	}
	return nil
}
