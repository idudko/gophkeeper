// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"os"

	"github.com/idudko/gophkeeper/internal/client/http"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/idudko/gophkeeper/pkg/logger"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RegisterRequest represents the registration request payload.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterResponse represents the registration response payload.
type RegisterResponse struct {
	Success bool `json:"success"`
	Data    struct {
		UserID         string `json:"user_id"`
		Email          string `json:"email"`
		CreatedAt      string `json:"created_at"`
		EncryptionSalt string `json:"encryption_salt"` // Salt for client-side key derivation
		AccessToken    string `json:"access_token,omitempty"`
		RefreshToken   string `json:"refresh_token,omitempty"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewRegisterCommand creates a new command to register a new user.
func NewRegisterCommand() *cobra.Command {
	var (
		email    string
		password string
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Зарегистрировать нового пользователя",
		Long: `Регистрирует нового пользователя в системе GophKeeper.

После регистрации вы получите доступ к функциям хранения и синхронизации данных.
Пароль должен содержать минимум 8 символов.`,
		Example: `  gophkeeper register --email user@example.com --password mypassword`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")

			// Validate inputs
			if email == "" {
				return models.ErrEmailRequiredForRegister
			}
			if password == "" {
				return models.ErrPasswordRequiredForRegister
			}

			log := logger.GetGlobal().With(
				logger.String("request_id", getCmdRequestID(cmd)),
				logger.String("command", "register"),
			)

			log.Info("user registration attempt",
				logger.String("email", email),
			)

			// Prepare registration request
			reqBody := RegisterRequest{
				Email:    email,
				Password: password,
			}

			// Create HTTP client with retry support
			cfg := &http.Config{
				ServerAddr: serverAddr,
				UserAgent:  "GophKeeperCLI/1.0.0",
				Logger:     zerolog.Nop(),
			}

			client, err := http.NewClient(cfg)
			if err != nil {
				return fmt.Errorf("ошибка создания HTTP клиента: %w", err)
			}

			// Send request
			var registerResp RegisterResponse
			if err := client.ParseJSONResponse(ctx, api.MethodPOST, api.PathRegister, reqBody, &registerResp); err != nil {
				return fmt.Errorf("ошибка запроса: %w", err)
			}

			// Check for errors in response
			if registerResp.Error != nil {
				return fmt.Errorf("ошибка сервера: %s (%s)", registerResp.Error.Message, registerResp.Error.Code)
			}

			if !registerResp.Success {
				return fmt.Errorf("регистрация не удалась")
			}

			log.Info("user registered successfully",
				logger.String("user_id", registerResp.Data.UserID),
				logger.String("email", registerResp.Data.Email),
			)

			// Save master password and encryption salt locally
			// Note: We save't encrypt the password here - it's saved raw for use with KeyManager
			if err := saveCredentials(password, registerResp.Data.EncryptionSalt); err != nil {
				log.Error("failed to save credentials",
					logger.Err(err),
				)
			}

			// Display success message
			fmt.Printf("✓ Регистрация успешно выполнена!\n")
			fmt.Printf("  ID пользователя: %s\n", registerResp.Data.UserID)
			fmt.Printf("  Email: %s\n", registerResp.Data.Email)
			fmt.Printf("  Salt шифрования: %s\n", registerResp.Data.EncryptionSalt)
			fmt.Printf("  Дата регистрации: %s\n", registerResp.Data.CreatedAt)

			// If tokens were returned, save them
			if registerResp.Data.AccessToken != "" && registerResp.Data.RefreshToken != "" {
				if err := saveTokens(registerResp.Data.AccessToken, registerResp.Data.RefreshToken); err != nil {
					fmt.Fprintf(os.Stderr, "\n⚠️  Предупреждение: не удалось сохранить токены: %v\n", err)
					fmt.Fprintf(os.Stderr, "Токены:\n")
					fmt.Fprintf(os.Stderr, "  Access Token:  %s\n", registerResp.Data.AccessToken)
					fmt.Fprintf(os.Stderr, "  Refresh Token: %s\n", registerResp.Data.RefreshToken)
				} else {
					fmt.Printf("✓ Токены сохранены локально. Вы автоматически авторизованы.\n")
				}
			} else {
				fmt.Printf("\nТеперь вы можете войти с помощью команды 'login'.\n")
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email адрес для регистрации")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Пароль (минимум 8 символов)")

	return cmd
}
