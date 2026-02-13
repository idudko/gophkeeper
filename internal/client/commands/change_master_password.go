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

// ChangeMasterPasswordOptions holds options for change-master-password command.
type ChangeMasterPasswordOptions struct {
	force      bool
	quiet      bool
	skipBackup bool
}

// NewChangeMasterPasswordCommand creates a new command to change master password.
func NewChangeMasterPasswordCommand() *cobra.Command {
	var opts ChangeMasterPasswordOptions

	cmd := &cobra.Command{
		Use:   "change-master-password",
		Short: "Сменить мастер-пароль шифрования",
		Long: `Сменяет мастер-пароль, который используется для шифрования всех данных
на клиенте. Эта операция перешифрует все ваши данные новым мастер-паролем.

ВАЖНО:
- Старый мастер-пароль будет необходим для валидации
- Все ваши данные будут перешифрованы
- При ошибке изменения пароля данные могут остаться в несогласованном состоянии
- Рекомендуется создать резервную копию перед изменением

Процесс смены мастер-пароля:
1. Проверка старого мастер-пароля
2. Загрузка всех данных с сервера
3. Расшифровка всех данных
4. Ввод и валидация нового мастер-пароля
5. Перешифрование всех данных новым паролем
6. Синхронизация с сервером`,
		Example: `  gophkeeper change-master-password
  gophkeeper change-master-password --force --skip-backup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := logger.GetGlobal().With(
				logger.String("command", "change-master-password"),
			)

			serverAddr := viper.GetString("server")

			// Step 1: Get access token
			token, err := loadAccessToken()
			if err != nil {
				fmt.Printf("❌ Ошибка: %v\n", err)
				fmt.Println()
				fmt.Println("Пожалуйста, сначала выполните:")
				fmt.Println("  gophkeeper login")
				fmt.Println()
				return err
			}

			// Step 2: Load old master password
			oldMasterPassword, encryptionSalt, err := loadEncryptionCredentials("")
			if err != nil {
				fmt.Printf("❌ Ошибка загрузки учетных данных: %v\n", err)
				fmt.Println()
				fmt.Println("Если вы еще не разблокировали данные, выполните:")
				fmt.Println("  gophkeeper unlock")
				return err
			}

			// Verify old master password
			log.Info("verifying old master password")
			keyManager := GetKeyManagerWithPassword(oldMasterPassword, encryptionSalt)

			// Test old password by encrypting sample data
			testData := []byte("test-validation-for-master-password-change")
			_, err = keyManager.Encrypt(testData)
			if err != nil {
				fmt.Printf("❌ Неверный мастер-пароль или поврежденные учетные данные\n")
				fmt.Println()
				fmt.Println("Если вы забыли старый мастер-пароль, все данные будут потеряны.")
				fmt.Println("Это невозможно исправить без резервной копии.")
				return fmt.Errorf("неверный мастер-пароль: %w", err)
			}

			fmt.Println("✅ Старый мастер-пароль проверен")

			// Step 3: Prompt for new master password
			fmt.Println()
			fmt.Println("=== Ввод нового мастер-пароля ===")
			fmt.Println("Требования к мастер-паролю:")
			fmt.Println("  - Минимум 12 символов")
			fmt.Println("  - Минимум 1 заглавная буква")
			fmt.Println("  - Минимум 1 строчная буква")
			fmt.Println("  - Минимум 1 цифра")
			fmt.Println("  - Минимум 1 специальный символ (!@#$%^&*)")
			fmt.Println()

			var newMasterPassword string
			for {
				fmt.Print("Введите новый мастер-пароль: ")
				newPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("ошибка чтения пароля: %w", err)
				}
				newMasterPassword = string(newPasswordBytes)
				fmt.Println()

				// Validate new password
				if err := validateMasterPassword(newMasterPassword); err != nil {
					fmt.Printf("❌ Пароль не соответствует требованиям: %v\n", err)
					fmt.Println()
					continue
				}

				// Confirm new password
				fmt.Print("Подтвердите новый мастер-пароль: ")
				confirmPasswordBytes, err := term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return fmt.Errorf("ошибка чтения пароля: %w", err)
				}
				confirmPassword := string(confirmPasswordBytes)
				fmt.Println()

				if newMasterPassword != confirmPassword {
					fmt.Println("❌ Пароли не совпадают. Попробуйте еще раз.")
					fmt.Println()
					continue
				}

				if newMasterPassword == oldMasterPassword {
					fmt.Println("❌ Новый пароль должен отличаться от старого.")
					fmt.Println()
					continue
				}

				// Password is valid
				break
			}

			// Step 4: Load all items from server
			fmt.Println()
			fmt.Println("=== Загрузка данных с сервера ===")
			ctx := cmd.Context()

			items, err := fetchItems(ctx, serverAddr, token, "")
			if err != nil {
				fmt.Printf("❌ Ошибка загрузки данных: %v\n", err)
				return fmt.Errorf("ошибка загрузки данных: %w", err)
			}

			if len(items) == 0 {
				fmt.Println("ℹ️  У вас нет данных на сервере.")
				fmt.Println("Мастер-пароль будет изменен, но данные перешифровывать не требуется.")
			} else {
				fmt.Printf("✅ Загружено %d записей\n", len(items))
			}

			// Step 5: Decrypt all items with old master password
			if len(items) > 0 {
				fmt.Println()
				fmt.Println("=== Расшифровка данных старым паролем ===")

				for i := range items {
					if err := decryptItemData(&items[i], oldMasterPassword, encryptionSalt); err != nil {
						fmt.Printf("❌ Ошибка расшифровки записи %s: %v\n", items[i].ID, err)
						return fmt.Errorf("ошибка расшифровки данных: %w", err)
					}

					if !opts.quiet {
						if (i+1)%10 == 0 || i == len(items)-1 {
							fmt.Printf("  Расшифровано: %d/%d\n", i+1, len(items))
						}
					}
				}

				fmt.Println("✅ Все данные расшифрованы")
			}

			// Step 6: Encrypt all items with new master password
			if len(items) > 0 {
				fmt.Println()
				fmt.Println("=== Шифрование данных новым паролем ===")

				for i := range items {
					if err := encryptItemData(&items[i], newMasterPassword, encryptionSalt); err != nil {
						fmt.Printf("❌ Ошибка шифрования записи %s: %v\n", items[i].ID, err)
						return fmt.Errorf("ошибка шифрования данных: %w", err)
					}

					if !opts.quiet {
						if (i+1)%10 == 0 || i == len(items)-1 {
							fmt.Printf("  Зашифровано: %d/%d\n", i+1, len(items))
						}
					}
				}

				fmt.Println("✅ Все данные зашифрованы")
			}

			// Step 7: Sync all items to server
			if len(items) > 0 {
				fmt.Println()
				fmt.Println("=== Синхронизация данных с сервером ===")

				for i := range items {
					itemID := items[i].ID
					updatedItem, err := sendUpdateItemRequestInternal(ctx, serverAddr, token, itemID, &items[i], true)
					if err != nil {
						fmt.Printf("❌ Ошибка обновления записи %s: %v\n", itemID, err)
						if !opts.force {
							return fmt.Errorf("ошибка обновления данных: %w", err)
						}
						fmt.Printf("  ⚠️  Пропуск записи из-за ошибки (используйте --force для продолжения)\n")
						continue
					}

					// Update item version from response
					if updatedItem != nil && updatedItem.Version > 0 {
						items[i].Version = updatedItem.Version
					}

					if !opts.quiet {
						if (i+1)%10 == 0 || i == len(items)-1 {
							fmt.Printf("  Обновлено: %d/%d\n", i+1, len(items))
						}
					}
				}

				fmt.Println("✅ Все данные синхронизированы")
			}

			// Step 8: Save new master password
			fmt.Println()
			fmt.Println("=== Сохранение нового мастер-пароля ===")

			if err := saveCredentials(newMasterPassword, encryptionSalt); err != nil {
				fmt.Printf("❌ Ошибка сохранения нового мастер-пароля: %v\n", err)
				return fmt.Errorf("ошибка сохранения мастер-пароля: %w", err)
			}

			fmt.Println("✅ Новый мастер-пароль сохранен")
			fmt.Println()
			fmt.Println("✅ Мастер-пароль успешно изменен!")
			fmt.Println()
			fmt.Println("Важно: Старый мастер-пароль больше не действителен.")
			fmt.Println("Все данные на сервере теперь зашифрованы новым паролем.")
			fmt.Println()
			fmt.Println("Рекомендации:")
			fmt.Println("  - Убедитесь, что новый мастер-пароль надежный и вы его запомнили")
			fmt.Println("  - Сохраните новую информацию в надежном менеджере паролей")
			fmt.Println("  - Регулярно создавайте резервные копии данных")

			log.Info("master password changed successfully",
				logger.String("item_count", fmt.Sprintf("%d", len(items))),
			)

			return nil
		},
	}

	// Flags
	cmd.Flags().BoolVar(&opts.force, "force", false, "Принудительно продолжать при ошибках обновления данных")
	cmd.Flags().BoolVarP(&opts.quiet, "quiet", "q", false, "Тихий режим (меньше вывода)")
	cmd.Flags().BoolVar(&opts.skipBackup, "skip-backup", false, "Пропустить создание резервной копии")

	return cmd
}
