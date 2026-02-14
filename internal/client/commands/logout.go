// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	ErrNoTokens = errors.New("no tokens found")
)

// NewLogoutCommand creates a new command to logout user.
func NewLogoutCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Выйти из системы GophKeeper",
		Long: `Выходит из системы GophKeeper, удаляя сохраненные токены аутентификации и мастер-пароль.

После выхода вам потребуется снова выполнить команду 'login' для авторизации.`,
		Example: `  gophkeeper logout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Remove saved tokens
			if err := clearTokens(); err != nil {
				// Check if it's just that tokens don't exist
				if errors.Is(err, ErrNoTokens) {
					fmt.Println("Токены авторизации не найдены.")
				} else {
					fmt.Printf("⚠️  Предупреждение: не удалось удалить токены: %v\n", err)
				}
			}

			// Remove saved credentials (master password and encryption salt)
			if err := clearCredentials(); err != nil {
				// Check if it's just that credentials don't exist
				if !errors.Is(err, os.ErrNotExist) {
					fmt.Printf("⚠️  Предупреждение: не удалось удалить мастер-пароль: %v\n", err)
				}
			} else {
				fmt.Println("✓ Мастер-пароль удалён из локального хранилища.")
			}

			fmt.Println("✓ Вы успешно вышли из системы.")
			return nil
		},
	}

	return cmd
}
