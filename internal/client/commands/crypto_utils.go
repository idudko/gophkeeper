// Package commands provides crypto utilities for GophKeeper client commands.
package commands

import (
	"fmt"

	"github.com/google/uuid"
	clientcrypto "github.com/idudko/gophkeeper/internal/client/crypto"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
)

// GetKeyManager creates a KeyManager from stored credentials.
func GetKeyManager() (*clientcrypto.KeyManager, error) {
	masterPassword, encryptionSalt, err := loadEncryptionCredentials("")
	if err != nil {
		return nil, err
	}
	return clientcrypto.NewKeyManager(masterPassword, encryptionSalt), nil
}

// GetKeyManagerWithPassword creates a KeyManager with the provided master password.
func GetKeyManagerWithPassword(masterPassword, encryptionSalt string) *clientcrypto.KeyManager {
	return clientcrypto.NewKeyManager(masterPassword, encryptionSalt)
}

// convertToCryptoItem converts models.Item to clientcrypto.Item for encryption.
func convertToCryptoItem(item *models.Item) *clientcrypto.Item {
	cryptoItem := &clientcrypto.Item{
		ID:        item.ID,
		UserID:    item.UserID,
		Type:      clientcrypto.DataType(item.Type),
		Version:   item.Version,
		Meta:      item.Meta,
		CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if item.DeletedAt != nil {
		deletedAt := item.DeletedAt.Format("2006-01-02T15:04:05Z07:00")
		cryptoItem.DeletedAt = &deletedAt
	}

	switch item.Type {
	case models.DataTypePassword:
		if item.Password != nil {
			cryptoItem.Password = &clientcrypto.PasswordData{
				Meta:     item.Password.Meta,
				Login:    item.Password.Login,
				Password: item.Password.Password,
			}
		}
	case models.DataTypeText:
		if item.Text != nil {
			cryptoItem.Text = &clientcrypto.TextData{
				Meta: item.Text.Meta,
				Data: item.Text.Data,
			}
		}
	case models.DataTypeBinary:
		if item.Binary != nil {
			cryptoItem.Binary = &clientcrypto.BinaryData{
				Meta:     item.Binary.Meta,
				Filename: item.Binary.Filename,
				Data:     item.Binary.Data,
			}
		}
	case models.DataTypeCard:
		if item.Card != nil {
			cryptoItem.Card = &clientcrypto.CardData{
				Meta:       item.Card.Meta,
				Number:     item.Card.Number,
				HolderName: item.Card.HolderName,
				ExpiryDate: item.Card.ExpiryDate,
				CVV:        item.Card.CVV,
			}
		}
	}

	return cryptoItem
}

// convertFromCryptoItem converts clientcrypto.Item back to models.Item after decryption.
func convertFromCryptoItem(cryptoItem *clientcrypto.Item, item *models.Item) {
	item.Meta = cryptoItem.Meta

	switch item.Type {
	case models.DataTypePassword:
		if cryptoItem.Password != nil && item.Password != nil {
			item.Password.Login = cryptoItem.Password.Login
			item.Password.Password = cryptoItem.Password.Password
		}
	case models.DataTypeText:
		if cryptoItem.Text != nil && item.Text != nil {
			item.Text.Data = cryptoItem.Text.Data
		}
	case models.DataTypeBinary:
		if cryptoItem.Binary != nil && item.Binary != nil {
			item.Binary.Data = cryptoItem.Binary.Data
		}
	case models.DataTypeCard:
		if cryptoItem.Card != nil && item.Card != nil {
			item.Card.Number = cryptoItem.Card.Number
			item.Card.HolderName = cryptoItem.Card.HolderName
			item.Card.ExpiryDate = cryptoItem.Card.ExpiryDate
			item.Card.CVV = cryptoItem.Card.CVV
		}
	}
}

// decryptItemData decrypts item data using the provided credentials.
func decryptItemData(item *models.Item, masterPassword, encryptionSalt string) error {
	keyManager := clientcrypto.NewKeyManager(masterPassword, encryptionSalt)
	cryptoItem := convertToCryptoItem(item)

	if err := keyManager.DecryptItem(cryptoItem); err != nil {
		return fmt.Errorf("ошибка расшифровки: %w", err)
	}

	convertFromCryptoItem(cryptoItem, item)
	return nil
}

// encryptItemData encrypts item data using the provided credentials.
func encryptItemData(item *models.Item, masterPassword, encryptionSalt string) error {
	keyManager := clientcrypto.NewKeyManager(masterPassword, encryptionSalt)
	cryptoItem := convertToCryptoItem(item)

	if err := keyManager.EncryptItem(cryptoItem); err != nil {
		return fmt.Errorf("ошибка шифрования: %w", err)
	}

	convertFromCryptoItem(cryptoItem, item)
	return nil
}

// validateMasterPassword validates the master password strength.
func validateMasterPassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("пароль должен содержать минимум 12 символов")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '!' || c == '@' || c == '#' || c == '$' || c == '%' || c == '^' || c == '&' || c == '*':
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("пароль должен содержать минимум 1 заглавную букву")
	}
	if !hasLower {
		return fmt.Errorf("пароль должен содержать минимум 1 строчную букву")
	}
	if !hasDigit {
		return fmt.Errorf("пароль должен содержать минимум 1 цифру")
	}
	if !hasSpecial {
		return fmt.Errorf("пароль должен содержать минимум 1 специальный символ (!@#$%%^&*)")
	}

	return nil
}

// getCmdRequestID generates a unique request ID for logging.
func getCmdRequestID(cmd *cobra.Command) string {
	return "cmd_" + cmd.Name() + "_" + uuid.New().String()[:8]
}
