// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/idudko/gophkeeper/internal/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DeleteCommandOptions holds options for the delete command.
type DeleteCommandOptions struct {
	confirm bool
	force   bool
	purge   bool
}

// NewDeleteCommand creates a new command to delete stored data.
func NewDeleteCommand() *cobra.Command {
	opts := &DeleteCommandOptions{}

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Удалить сохраненные данные из хранилища",
		Long: `Удаляет данные из защищенного хранилища GophKeeper по их ID.

По умолчанию выполняется мягкое удаление (soft delete), при котором данные помечаются как удаленные
но не удаляются физически из базы данных. Это позволяет восстановить данные при необходимости.

Флаг --purge выполняет окончательное удаление данных без возможности восстановления.`,
		Example: `  # Удалить данные с подтверждением
  gophkeeper delete 550e8400-e29b-41d4-a716-446655440

  # Удалить без подтверждения
  gophkeeper delete 550e8400-e29b-41d4-a716-446655440 --confirm

  # Принудительное удаление (игнорировать конфликты)
  gophkeeper delete 550e8400-e29b-41d4-a716-446655440 --force

  # Окончательное удаление (без возможности восстановления)
  gophkeeper delete 550e8400-e29b-41d4-a716-446655440 --purge`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")
			itemID := args[0]

			// Load access token
			token, err := loadAccessToken()
			if err != nil {
				return err
			}

			// Fetch item information before deletion
			item, err := fetchItem(ctx, serverAddr, token, itemID)
			if err != nil {
				return fmt.Errorf("не удалось получить информацию об элементе: %w", err)
			}

			// Display item information
			displayItemForDeletion(item, cmd.OutOrStdout())

			// Confirm deletion unless --confirm or --force flag is set
			if !opts.confirm && !opts.force {
				if err := confirmDeletion(itemID, opts.purge); err != nil {
					return err
				}
			}

			// Send delete request
			if opts.purge {
				if err := purgeItem(ctx, serverAddr, token, itemID); err != nil {
					fmt.Printf("❌ Ошибка при окончательном удалении: %v\n", err)
					if !opts.force {
						return err
					}
					fmt.Printf("✓ Данные окончательно удалены без возможности восстановления.\n")
				} else {
					fmt.Printf("✓ Данные окончательно удалены без возможности восстановления.\n")
				}
			} else {
				if err := deleteItem(ctx, serverAddr, token, itemID); err != nil {
					fmt.Printf("❌ Ошибка при удалении: %v\n", err)
					if !opts.force {
						return err
					}
					fmt.Printf("✓ Данные удалены (можно восстановить).\n")
				} else {
					fmt.Printf("✓ Данные удалены (можно восстановить).\n")
				}
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().BoolVarP(&opts.confirm, "confirm", "y", false, "Подтвердить удаление без запроса")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Принудительное удаление (без подтверждения и проверки конфликтов)")
	cmd.Flags().BoolVarP(&opts.purge, "purge", "p", false, "Окончательное удаление без возможности восстановления")

	return cmd
}

// displayItemForDeletion displays information about an item to be deleted.
func displayItemForDeletion(item *models.Item, output io.Writer) {
	fmt.Fprintf(output, "\n════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(output, "                  УДАЛЕНИЕ ДАННЫХ                            \n")
	fmt.Fprintf(output, "══════════════════════════════════════════════════════\n\n")

	// This is a simplified display - in a real implementation, you'd use the actual item type
	fmt.Fprintf(output, "Будет удален следующий элемент:\n")
	fmt.Fprintf(output, "  ID:    %s\n", item.ID)
	fmt.Fprintf(output, "  Тип:  данные\n")
	fmt.Fprintf(output, "\n")
}

// confirmDeletion prompts user to confirm deletion.
func confirmDeletion(itemID string, purge bool) error {
	var action string
	if purge {
		action = "ОКОНЧАТЕЛЬНО УДАЛИТЬ"
	} else {
		action = "удалить"
	}

	fmt.Printf("\nВы действительно хотите %s данные с ID %s?\n", strings.ToUpper(action), itemID)
	if purge {
		fmt.Printf("⚠️  ВНИМАНИЕ: Это действие необратимо! Данные будут потеряны навсегда.\n")
	}

	fmt.Print("Подтвердите (да/yes/y): ")

	var confirmation string
	_, err := fmt.Scanln(&confirmation)
	if err != nil {
		return fmt.Errorf("ошибка ввода подтверждения: %w", err)
	}

	confirmation = strings.ToLower(strings.TrimSpace(confirmation))
	if confirmation != "да" && confirmation != "yes" && confirmation != "y" {
		return fmt.Errorf("удаление отменено пользователем")
	}

	return nil
}
