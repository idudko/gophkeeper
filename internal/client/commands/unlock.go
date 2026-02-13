// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"

	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/idudko/gophkeeper/pkg/logger"
)

// UnlockCommandOptions holds options for the unlock command.
type UnlockCommandOptions struct {
	masterPassword string
}

// NewUnlockCommand creates a new command to unlock encrypted credentials.
func NewUnlockCommand() *cobra.Command {
	var opts UnlockCommandOptions

	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Разблокировать учетные данные с помощью мастер-пароля",
		Long: `Разблокирует зашифрованные учетные данные (мастер-пароль и salt шифрования)
требуемые для работы с зашифрованными данными на клиенте.

Эта команда нужна, когда:
- Вы впервые запускаете gophkeeper после установки
- Вы хотите разблокировать данные после выхода из системы
- Вам нужно ввести мастер-пароль для доступа к зашифрованным данным`,
		Example: `  gophkeeper unlock
  gophkeeper unlock --master-password "MyMasterPass123!"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.GetGlobal().With(
				logger.String("command", "unlock"),
			)

			// Get server address
			serverAddr := viper.GetString("server")

			// If master password not provided as flag, prompt for it
			if opts.masterPassword == "" {
				fmt.Print("Введите мастер-пароль для разблокировки: ")
				masterPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("ошибка чтения мастер-пароля: %w", err)
				}
				opts.masterPassword = string(masterPasswordBytes)
				fmt.Println() // New line after password input
			}

			// Validate master password strength
			if err := validateMasterPassword(opts.masterPassword); err != nil {
				fmt.Printf("❌ Неверный мастер-пароль: %v\n", err)
				fmt.Println()
				fmt.Println("Требования к мастер-паролю:")
				fmt.Println("  - Минимум 12 символов")
				fmt.Println("  - Минимум 1 заглавная буква")
				fmt.Println("  - Минимум 1 строчная буква")
				fmt.Println("  - Минимум 1 цифра")
				fmt.Println("  - Минимум 1 специальный символ (!@#$%^&*)")
				return err
			}

			log.Info("attempting to unlock credentials",
				logger.String("server", serverAddr),
			)

			// Load encryption salt from local storage
			encryptionSalt, err := loadEncryptionSalt()
			if err != nil {
				fmt.Printf("❌ Ошибка загрузки соли шифрования: %v\n", err)
				fmt.Println()
				fmt.Println("Если вы еще не входили в систему, выполните:")
				fmt.Println("  gophkeeper login")
				return err
			}

			// Verify master password by trying to derive encryption key
			// If password is wrong, encryption operations will fail
			testPlaintext := []byte("test-validation-123")
			keyManager := GetKeyManagerWithPassword(opts.masterPassword, encryptionSalt)

			_, err = keyManager.Encrypt(testPlaintext)
			if err != nil {
				fmt.Printf("❌ Неверный мастер-пароль: %v\n", err)
				fmt.Println()
				fmt.Println("Пожалуйста, проверьте:")
				fmt.Println("  - Правильность ввода мастер-пароля")
				fmt.Println("  - Раскладку клавиатуры")
				fmt.Println("  - То, что используете мастер-пароль от этого аккаунта")
				return fmt.Errorf("неверный мастер-пароль: %w", err)
			}
			fmt.Println("✅ Учетные данные успешно разблокированы!")
			fmt.Println()
			fmt.Println("Теперь вы можете использовать следующие команды:")
			fmt.Println("  gophkeeper list    - просмотреть список данных")
			fmt.Println("  gophkeeper get     - получить конкретную запись")
			fmt.Println("  gophkeeper add     - добавить новые данные")
			fmt.Println("  gophkeeper sync    - синхронизировать данные с сервером")

			log.Info("credentials unlocked successfully")
			return nil
		},
	}

	// Flags
	cmd.Flags().StringVar(&opts.masterPassword, "master-password", "", "Мастер-пароль для разблокировки (если не указан, будет предложено ввести)")

	return cmd
}
