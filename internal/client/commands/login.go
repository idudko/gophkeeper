// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/idudko/gophkeeper/internal/client/http"
	"github.com/idudko/gophkeeper/internal/models"
	"github.com/idudko/gophkeeper/pkg/api"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// LoginRequest represents a login request payload.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents a login response payload.
type LoginResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Token          string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		UserID         string `json:"user_id"`
		Email          string `json:"email"`
		EncryptionSalt string `json:"encryption_salt"` // Salt for client-side key derivation
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewLoginCommand creates a new command to login user.
func NewLoginCommand() *cobra.Command {
	var (
		email     string
		password  string
		saveToken bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Авторизоваться в системе GophKeeper",
		Long: `Авторизует пользователя в системе GophKeeper.

После успешной авторизации токен будет сохранён локально для использования в последующих командах.
Мастер-пароль также будет сохранён локально для клиентского шифрования данных.`,
		Example: `  # Вход с параметрами
  gophkeeper login --email user@example.com --password mypassword

  # Вход без сохранения токенов
  gophkeeper login --no-save-token`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			serverAddr := viper.GetString("server")

			// Validate inputs
			if email == "" {
				return models.ErrEmailRequiredForLogin
			}
			if password == "" {
				return models.ErrPasswordRequiredForLogin
			}

			// Send login request
			resp, err := performLogin(ctx, serverAddr, email, password)
			if err != nil {
				return err
			}

			// Display success message
			fmt.Printf("✓ Авторизация успешно выполнена!\n")
			fmt.Printf("  ID пользователя: %s\n", resp.Data.UserID)
			fmt.Printf("  Email: %s\n", resp.Data.Email)

			// Save master password and encryption salt locally
			if err := saveCredentials(password, resp.Data.EncryptionSalt); err != nil {
				fmt.Fprintf(os.Stderr, "\n⚠️  Предупреждение: не удалось сохранить мастер-пароль: %v\n", err)
			} else {
				fmt.Printf("✓ Мастер-пароль сохранён локально.\n")
			}

			// Save tokens
			if saveToken {
				if err := saveTokens(resp.Data.Token, resp.Data.RefreshToken); err != nil {
					fmt.Fprintf(os.Stderr, "\n⚠️  Предупреждение: не удалось сохранить токены: %v\n", err)
					fmt.Fprintf(os.Stderr, "Токены:\n")
					fmt.Fprintf(os.Stderr, "  Access Token:  %s\n", resp.Data.Token)
					fmt.Fprintf(os.Stderr, "  Refresh Token: %s\n", resp.Data.RefreshToken)
				} else {
					fmt.Printf("✓ Токены сохранены локально.\n")
				}
			} else {
				fmt.Printf("\nТокены:\n")
				fmt.Printf("  Access Token:  %s\n", resp.Data.Token)
				fmt.Printf("  Refresh Token: %s\n", resp.Data.RefreshToken)
			}

			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email адрес")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Пароль")
	cmd.Flags().BoolVar(&saveToken, "save-token", true, "Сохранить токены локально")

	return cmd
}

// performLogin sends login request to the server using the retryable HTTP client.
func performLogin(ctx context.Context, serverAddr, email, password string) (*LoginResponse, error) {
	// Create HTTP client with retry support
	cfg := &http.Config{
		ServerAddr: serverAddr,
		UserAgent:  "GophKeeperCLI/1.0.0",
		Logger:     zerolog.Nop(),
	}

	client, err := http.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP клиента: %w", err)
	}

	// Prepare request body
	reqBody := LoginRequest{
		Email:    email,
		Password: password,
	}

	// Send request
	var loginResp LoginResponse
	if err := client.ParseJSONResponse(ctx, api.MethodPOST, api.PathLogin, reqBody, &loginResp); err != nil {
		return nil, fmt.Errorf("ошибка запроса: %w", err)
	}

	// Check for errors in response
	if loginResp.Error != nil {
		return nil, fmt.Errorf("ошибка авторизации: %s (%s)", loginResp.Error.Message, loginResp.Error.Code)
	}

	if !loginResp.Success {
		return nil, fmt.Errorf("авторизация не удалась")
	}

	return &loginResp, nil
}
