// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// ConfigCommandOptions holds options for the config command.
type ConfigCommandOptions struct {
	format     string
	configFile string
	editable   bool
	showAll    bool
	validate   bool
	outputFile string
}

// NewConfigCommand creates a new command to manage client configuration.
func NewConfigCommand() *cobra.Command {
	opts := &ConfigCommandOptions{}

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Управление конфигурацией клиента GophKeeper",
		Long: `Управляет настройками конфигурации клиента GophKeeper.

Поддерживает просмотр, редактирование и изменение значений конфигурации.
Конфигурация хранится в файле config.yaml в директории пользователя.

Доступные подкоманды:
  get     - Просмотр значений конфигурации
  set     - Установка значений конфигурации
  unset   - Удаление значений конфигурации
  list    - Вывод всех значений конфигурации
  reset   - Сброс значений к умолчанию
  edit    - Редактирование файла конфигурации в редакторе
  path    - Вывод пути к файлу конфигурации`,
		Example: `  # Просмотр всех значений конфигурации
  gophkeeper config list

  # Просмотр конкретного значения
  gophkeeper config get server

  # Установка значения конфигурации
  gophkeeper config set server https://api.gophkeeper.com

  # Удаление значения конфигурации
  gophkeeper config unset server

  # Редактирование файла конфигурации
  gophkeeper config edit

  # Вывод пути к файлу конфигурации
  gophkeeper config path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Если подкоманда не указана, показать справку
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("неизвестная подкоманда: %s", args[0])
		},
	}

	// Add subcommands
	cmd.AddCommand(
		NewConfigGetCommand(opts),
		NewConfigSetCommand(opts),
		NewConfigUnsetCommand(opts),
		NewConfigListCommand(opts),
		NewConfigResetCommand(opts),
		NewConfigEditCommand(opts),
		NewConfigPathCommand(opts),
	)

	return cmd
}

// NewConfigGetCommand creates a new command to get configuration values.
func NewConfigGetCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Получить значение конфигурации",
		Long:  `Выводит значение указанного ключа конфигурации.`,
		Example: `  gophkeeper config get server
  gophkeeper config get verbose
  gophkeeper config get --format json server`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate format
			validFormats := map[string]bool{
				"yaml":   true,
				"json":   true,
				"pretty": true,
			}
			if !validFormats[opts.format] {
				return fmt.Errorf("неверный формат вывода: %s. Допустимые значения: yaml, json, pretty", opts.format)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Get value from viper
			if !viper.IsSet(key) {
				return fmt.Errorf("ключ '%s' не найден в конфигурации", key)
			}

			value := viper.Get(key)

			// Display based on format
			switch opts.format {
			case "json":
				displayConfigJSON(key, value, cmd.OutOrStdout())
			case "yaml":
				displayConfigYAML(key, value, cmd.OutOrStdout())
			default: // pretty
				displayConfigPretty(key, value, cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.format, "format", "f", "pretty", "Формат вывода (yaml, json, pretty)")
	cmd.Flags().StringVarP(&opts.outputFile, "output", "o", "", "Файл для сохранения вывода")

	return cmd
}

// NewConfigSetCommand creates a new command to set configuration values.
func NewConfigSetCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Установить значение конфигурации",
		Long:  `Устанавливает значение для указанного ключа конфигурации.`,
		Example: `  gophkeeper config set server https://api.gophkeeper.com
  gophkeeper config set verbose true
  gophkeeper config set --validate server http://localhost:8080`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// Validate key
			if err := validateConfigKey(key); err != nil {
				return err
			}

			// Parse value (try to infer type)
			parsedValue, err := parseConfigValue(value)
			if err != nil {
				return fmt.Errorf("ошибка парсинга значения: %w", err)
			}

			// Validate value if requested
			if opts.validate {
				if err := validateConfigValue(key, parsedValue); err != nil {
					return fmt.Errorf("ошибка валидации: %w", err)
				}
			}

			// Set value in viper
			viper.Set(key, parsedValue)

			// Get config file path
			configFile := viper.ConfigFileUsed()
			if configFile == "" {
				configFile = opts.configFile
			}

			// Save to file
			if err := viper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
			}

			fmt.Printf("✓ Конфигурация успешно обновлена:\n")
			fmt.Printf("  %s = %v\n", key, parsedValue)
			fmt.Printf("  Файл: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.validate, "validate", false, "Проверить значение перед сохранением")
	cmd.Flags().StringVarP(&opts.configFile, "config-file", "c", "", "Путь к файлу конфигурации")

	return cmd
}

// NewConfigUnsetCommand creates a new command to unset configuration values.
func NewConfigUnsetCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Удалить значение конфигурации",
		Long:  `Удаляет значение указанного ключа конфигурации, возвращая его к значению по умолчанию.`,
		Example: `  gophkeeper config unset server
  gophkeeper config unset verbose`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Check if key exists
			if !viper.IsSet(key) {
				return fmt.Errorf("ключ '%s' не найден в конфигурации", key)
			}

			// Confirm deletion
			fmt.Printf("Вы уверены, что хотите удалить ключ '%s'? (да/yes/y): ", key)
			var confirmation string
			if _, err := fmt.Scanln(&confirmation); err != nil {
				return fmt.Errorf("ошибка ввода подтверждения: %w", err)
			}

			confirmation = strings.ToLower(strings.TrimSpace(confirmation))
			if confirmation != "да" && confirmation != "yes" && confirmation != "y" {
				fmt.Println("Удаление отменено.")
				return nil
			}

			// Remove key
			value := viper.Get(key)
			viper.Set(key, nil)

			// Get config file path
			configFile := viper.ConfigFileUsed()
			if configFile == "" {
				configFile = opts.configFile
			}

			// Save to file
			if err := viper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
			}

			fmt.Printf("✓ Ключ конфигурации успешно удален:\n")
			fmt.Printf("  %s (было: %v)\n", key, value)
			fmt.Printf("  Файл: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.configFile, "config-file", "c", "", "Путь к файлу конфигурации")

	return cmd
}

// NewConfigListCommand creates a new command to list all configuration values.
func NewConfigListCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Вывести все значения конфигурации",
		Long:  `Выводит все текущие значения конфигурации.`,
		Example: `  gophkeeper config list
  gophkeeper config list --format json
  gophkeeper config list --show-all`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate format
			validFormats := map[string]bool{
				"yaml":   true,
				"json":   true,
				"pretty": true,
			}
			if !validFormats[opts.format] {
				return fmt.Errorf("неверный формат вывода: %s. Допустимые значения: yaml, json, pretty", opts.format)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get all settings
			settings := viper.AllSettings()

			// Filter default values if not showing all
			if !opts.showAll {
				settings = filterDefaultValues(settings)
			}

			// Display based on format
			switch opts.format {
			case "json":
				displayAllConfigJSON(settings, cmd.OutOrStdout())
			case "yaml":
				displayAllConfigYAML(settings, cmd.OutOrStdout())
			default: // pretty
				displayAllConfigPretty(settings, cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.format, "format", "f", "pretty", "Формат вывода (yaml, json, pretty)")
	cmd.Flags().BoolVarP(&opts.showAll, "show-all", "a", false, "Показать все значения, включая значения по умолчанию")

	return cmd
}

// NewConfigResetCommand creates a new command to reset configuration.
func NewConfigResetCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset [key]",
		Short: "Сбросить конфигурацию к значениям по умолчанию",
		Long: `Сбрасывает конфигурацию к значениям по умолчанию.

Если указан ключ, сбрасывается только этот ключ.
Если ключ не указан, сбрасывается вся конфигурация.`,
		Example: `  gophkeeper config reset
  gophkeeper config reset server
  gophkeeper config reset --force`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var key string
			if len(args) > 0 {
				key = args[0]
			}

			// Confirm reset unless --force flag is set
			if key == "" {
				fmt.Println("⚠️  ВНИМАНИЕ: Это действие сбросит ВСЮ конфигурацию к значениям по умолчанию!")
			} else {
				fmt.Printf("Сбросить ключ '%s' к значению по умолчанию?\n", key)
			}
			fmt.Print("Подтвердите (да/yes/y): ")

			var confirmation string
			if _, err := fmt.Scanln(&confirmation); err != nil {
				return fmt.Errorf("ошибка ввода подтверждения: %w", err)
			}

			confirmation = strings.ToLower(strings.TrimSpace(confirmation))
			if confirmation != "да" && confirmation != "yes" && confirmation != "y" {
				fmt.Println("Сброс отменен.")
				return nil
			}

			// Perform reset
			if key != "" {
				// Reset specific key
				viper.Set(key, nil)
				fmt.Printf("✓ Ключ '%s' сброшен к значению по умолчанию\n", key)
			} else {
				// Reset all configuration
				viper.Reset()
				fmt.Println("✓ Конфигурация сброшена к значениям по умолчанию")
			}

			// Get config file path
			configFile := viper.ConfigFileUsed()
			if configFile == "" {
				configFile = opts.configFile
			}

			// Save to file
			if err := viper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("ошибка сохранения конфигурации: %w", err)
			}

			fmt.Printf("  Файл: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.configFile, "config-file", "c", "", "Путь к файлу конфигурации")

	return cmd
}

// NewConfigEditCommand creates a new command to edit configuration file.
func NewConfigEditCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Открыть файл конфигурации в редакторе",
		Long: `Открывает файл конфигурации в текстовом редакторе для ручного редактирования.

Редактор определяется переменной окружения EDITOR или автоматически выбирается.`,
		Example: `  gophkeeper config edit
  EDITOR=nano gophkeeper config edit
  gophkeeper config edit --editor vim`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get config file path
			configFile := viper.ConfigFileUsed()
			if configFile == "" {
				configFile = opts.configFile
			}

			// If config file doesn't exist, create default one
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				// Ensure directory exists
				configDir := filepath.Dir(configFile)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					return fmt.Errorf("ошибка создания директории конфигурации: %w", err)
				}

				// Write default config
				if err := viper.WriteConfigAs(configFile); err != nil {
					return fmt.Errorf("ошибка создания файла конфигурации: %w", err)
				}

				fmt.Printf("✓ Создан файл конфигурации: %s\n", configFile)
			}

			// Determine editor to use
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}

			// Platform-specific defaults
			if editor == "" {
				switch runtime.GOOS {
				case "windows":
					editor = "notepad"
				case "darwin":
					editor = "open"
				default: // linux, bsd, etc.
					editor = "vi"
				}
			}

			fmt.Printf("Открытие файла конфигурации в редакторе: %s\n", configFile)
			fmt.Printf("Редактор: %s\n\n", editor)

			// Open editor
			editorCmd := exec.Command(editor, configFile)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("ошибка запуска редактора: %w", err)
			}

			fmt.Println("\n✓ Редактирование завершено")

			// Validate configuration if requested
			if opts.validate {
				if err := validateConfigFile(configFile); err != nil {
					return fmt.Errorf("ошибка валидации конфигурации: %w", err)
				}
				fmt.Println("✓ Конфигурация валидна")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.configFile, "config-file", "", "Путь к файлу конфигурации")
	cmd.Flags().BoolVar(&opts.validate, "validate", false, "Проверить конфигурацию после редактирования")

	return cmd
}

// NewConfigPathCommand creates a new command to show config file path.
func NewConfigPathCommand(opts *ConfigCommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Вывести путь к файлу конфигурации",
		Long:  `Выводит полный путь к файлу конфигурации клиента.`,
		Example: `  gophkeeper config path
  gophkeeper config path --dir`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get config file path
			configFile := viper.ConfigFileUsed()
			if configFile == "" {
				// Try to construct default path
				configDir, err := getUserConfigDir()
				if err != nil {
					return fmt.Errorf("не удалось определить путь к конфигурации: %w", err)
				}
				configFile = filepath.Join(configDir, "config.yaml")
			}

			// Display based on flags
			if opts.showAll {
				// Show directory only
				fmt.Println(filepath.Dir(configFile))
			} else {
				// Show full file path
				fmt.Println(configFile)
			}

			// Check if file exists
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "\n⚠️  Файл конфигурации не существует\n")
				fmt.Fprintf(os.Stderr, "   Используйте 'gophkeeper config edit' для создания файла конфигурации\n")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.showAll, "dir", "d", false, "Показать только директорию")

	return cmd
}

// validateConfigKey validates a configuration key.
func validateConfigKey(key string) error {
	validKeys := map[string]bool{
		"server":  true,
		"verbose": true,
		"config":  true,
		"output":  true,
	}

	// Check if key is valid
	if !validKeys[key] && !strings.HasPrefix(key, "custom.") {
		return fmt.Errorf("неверный ключ конфигурации: %s", key)
	}

	return nil
}

// parseConfigValue parses a string value and tries to infer its type.
func parseConfigValue(value string) (any, error) {
	// Try to parse as bool
	if strings.ToLower(value) == "true" {
		return true, nil
	}
	if strings.ToLower(value) == "false" {
		return false, nil
	}

	// Try to parse as integer
	var intVal int64
	if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
		// Check if the entire string was consumed
		if fmt.Sprintf("%d", intVal) == value {
			return intVal, nil
		}
	}

	// Try to parse as float
	var floatVal float64
	if _, err := fmt.Sscanf(value, "%f", &floatVal); err == nil {
		// Check if the entire string was consumed
		if fmt.Sprintf("%f", floatVal) == value ||
			fmt.Sprintf("%.2f", floatVal) == value ||
			fmt.Sprintf("%.6f", floatVal) == value {
			return floatVal, nil
		}
	}

	// Return as string
	return value, nil
}

// validateConfigValue validates a configuration value.
func validateConfigValue(key string, value any) error {
	switch key {
	case "server":
		if str, ok := value.(string); ok {
			if str == "" {
				return fmt.Errorf("адрес сервера не может быть пустым")
			}
			if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
				return fmt.Errorf("адрес сервера должен начинаться с http:// или https://")
			}
		} else {
			return fmt.Errorf("адрес сервера должен быть строкой")
		}
	case "verbose":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("verbose должен быть булевым значением (true/false)")
		}
	}

	return nil
}

// validateConfigFile validates the configuration file.
func validateConfigFile(configFile string) error {
	// Load file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла конфигурации: %w", err)
	}

	// Try to parse as YAML
	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("ошибка парсинга YAML: %w", err)
	}

	// Validate all keys
	for key, value := range config {
		if err := validateConfigValue(key, value); err != nil {
			return fmt.Errorf("ошибка в ключе '%s': %w", key, err)
		}
	}

	return nil
}

// filterDefaultValues filters out default values from settings.
func filterDefaultValues(settings map[string]any) map[string]any {
	filtered := make(map[string]any)

	// Define default values
	defaults := map[string]any{
		"server":  "http://localhost:8080",
		"verbose": false,
	}

	// Keep only values that differ from defaults
	for key, value := range settings {
		if defaultValue, exists := defaults[key]; exists {
			if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", defaultValue) {
				filtered[key] = value
			}
		} else {
			// Keep custom values
			filtered[key] = value
		}
	}

	return filtered
}

// displayConfigPretty displays a configuration value in pretty format.
func displayConfigPretty(key string, value any, output io.Writer) {
	fmt.Fprintf(output, "%s: %v\n", key, value)
}

// displayConfigJSON displays a configuration value in JSON format.
func displayConfigJSON(key string, value any, output io.Writer) {
	data := map[string]any{
		"success": true,
		"key":     key,
		"value":   value,
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

// displayConfigYAML displays a configuration value in YAML format.
func displayConfigYAML(key string, value any, output io.Writer) {
	data := map[string]any{
		key: value,
	}

	if yamlData, err := yaml.Marshal(data); err == nil {
		output.Write(yamlData)
	} else {
		fmt.Fprintf(output, "Ошибка кодирования YAML: %v\n", err)
	}
}

// displayAllConfigPretty displays all configuration values in pretty format.
func displayAllConfigPretty(settings map[string]any, output io.Writer) {
	if len(settings) == 0 {
		fmt.Fprintln(output, "Нет настроенных значений конфигурации.")
		return
	}

	fmt.Fprintf(output, "Конфигурация GophKeeper:\n")
	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 50))

	// Sort keys for consistent output
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}

	// Sort keys
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	// Display each key-value pair
	for _, key := range keys {
		fmt.Fprintf(output, "  %-15s: %v\n", key, settings[key])
	}

	fmt.Fprintf(output, "%s\n", strings.Repeat("-", 50))
}

// displayAllConfigJSON displays all configuration values in JSON format.
func displayAllConfigJSON(settings map[string]any, output io.Writer) {
	data := map[string]any{
		"success":  true,
		"settings": settings,
		"total":    len(settings),
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

// displayAllConfigYAML displays all configuration values in YAML format.
func displayAllConfigYAML(settings map[string]any, output io.Writer) {
	data := map[string]any{
		"settings": settings,
	}

	if yamlData, err := yaml.Marshal(data); err == nil {
		output.Write(yamlData)
	} else {
		fmt.Fprintf(output, "Ошибка кодирования YAML: %v\n", err)
	}
}
