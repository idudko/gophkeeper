// Package commands provides local storage utilities for GophKeeper client.
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// loadAccessToken loads the access token from local storage.
func loadAccessToken() (string, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать токен: %w (возможно, требуется войти в систему)", err)
	}

	return string(data), nil
}

// loadEncryptionCredentials loads the master password and encryption salt.
func loadEncryptionCredentials(masterKeyOverride string) (string, string, error) {
	// If master key is provided via flag, use it
	if masterKeyOverride != "" {
		salt, err := loadEncryptionSalt()
		if err != nil {
			return "", "", err
		}
		return masterKeyOverride, salt, nil
	}

	// Otherwise, load from local storage
	credPath, err := getCredentialsPath()
	if err != nil {
		return "", "", err
	}

	data, err := os.ReadFile(credPath)
	if err != nil {
		return "", "", fmt.Errorf("не удалось загрузить учётные данные: %w (возможно, требуется разблокировать хранилище)", err)
	}

	var creds struct {
		MasterPassword string `json:"master_password"`
		EncryptionSalt string `json:"encryption_salt"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return "", "", fmt.Errorf("ошибка парсинга учётных данных: %w", err)
	}

	return creds.MasterPassword, creds.EncryptionSalt, nil
}

// loadEncryptionSalt loads the encryption salt from local storage.
func loadEncryptionSalt() (string, error) {
	saltPath, err := getSaltPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(saltPath)
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать соль шифрования: %w", err)
	}

	return string(data), nil
}

// saveCredentials saves the master password and encryption salt to local storage.
func saveCredentials(masterPassword, encryptionSalt string) error {
	credPath, err := getCredentialsPath()
	if err != nil {
		return err
	}

	creds := struct {
		MasterPassword string `json:"master_password"`
		EncryptionSalt  string `json:"encryption_salt"`
	}{
		MasterPassword: masterPassword,
		EncryptionSalt:  encryptionSalt,
	}

	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("ошибка сериализации учётных данных: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(credPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("ошибка создания директории: %w", err)
	}

	if err := os.WriteFile(credPath, data, 0600); err != nil {
		return fmt.Errorf("ошибка записи учётных данных: %w", err)
	}

	return nil
}

// saveTokens saves access and refresh tokens to local storage.
func saveTokens(accessToken, refreshToken string) error {
	configDir, err := getUserConfigDir()
	if err != nil {
		return err
	}

	tokenPath := filepath.Join(configDir, "token")
	if err := os.WriteFile(tokenPath, []byte(accessToken), 0600); err != nil {
		return fmt.Errorf("ошибка сохранения токена: %w", err)
	}

	if refreshToken != "" {
		refreshTokenPath := filepath.Join(configDir, "refresh_token")
		if err := os.WriteFile(refreshTokenPath, []byte(refreshToken), 0600); err != nil {
			return fmt.Errorf("ошибка сохранения refresh токена: %w", err)
		}
	}

	return nil
}

// clearTokens removes stored tokens from local storage.
func clearTokens() error {
	configDir, err := getUserConfigDir()
	if err != nil {
		return err
	}

	tokenPath := filepath.Join(configDir, "token")
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ошибка удаления токена: %w", err)
	}

	refreshTokenPath := filepath.Join(configDir, "refresh_token")
	if err := os.Remove(refreshTokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ошибка удаления refresh токена: %w", err)
	}

	return nil
}

// clearCredentials removes stored credentials from local storage.
func clearCredentials() error {
	configDir, err := getUserConfigDir()
	if err != nil {
		return err
	}

	credPath := filepath.Join(configDir, "credentials.json")
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ошибка удаления учётных данных: %w", err)
	}

	return nil
}

// getTokenPath returns the path to the stored access token.
func getTokenPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("ошибка получения директории конфигурации: %w", err)
	}
	return filepath.Join(configDir, "gophkeeper", "token"), nil
}

// getCredentialsPath returns the path to the stored encryption credentials.
func getCredentialsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("ошибка получения директории конфигурации: %w", err)
	}
	return filepath.Join(configDir, "gophkeeper", "credentials.json"), nil
}

// getSaltPath returns the path to the stored encryption salt.
func getSaltPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("ошибка получения директории конфигурации: %w", err)
	}
	return filepath.Join(configDir, "gophkeeper", "salt"), nil
}

// getUserConfigDir returns the user config directory for GophKeeper.
func getUserConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("ошибка получения директории конфигурации: %w", err)
	}
	gophkeeperDir := filepath.Join(configDir, "gophkeeper")
	if err := os.MkdirAll(gophkeeperDir, 0700); err != nil {
		return "", fmt.Errorf("ошибка создания директории gophkeeper: %w", err)
	}
	return gophkeeperDir, nil
}
