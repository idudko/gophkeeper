// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd represents the base command when called without any subcommands
type RootCmd struct {
	cmd        *cobra.Command
	configPath string
	serverAddr string
	verbose    bool
	version    string
	gitCommit  string
	buildDate  string
}

// NewRootCommand creates a new root command for the CLI application.
func NewRootCommand(version, gitCommit, buildDate string) *cobra.Command {
	root := &RootCmd{
		version:   version,
		gitCommit: gitCommit,
		buildDate: buildDate,
	}

	cmd := &cobra.Command{
		Use:   "gophkeeper",
		Short: "GophKeeper - безопасный менеджер паролей",
		Long: `GophKeeper представляет собой клиент-серверную систему, позволяющую пользователю
надёжно и безопасно хранить логины, пароли, бинарные данные и прочую приватную информацию.

Клиент поддерживает работу с различными типами данных:
  - пары логин/пароль
  - произвольные текстовые данные
  - произвольные бинарные данные
  - данные банковских карт

Для всех типов данных поддерживается хранение произвольной текстовой метаинформации.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (git: %s, built: %s)", version, gitCommit, buildDate),
	}

	// Persistent flags (available to all subcommands)
	persistentFlags := cmd.PersistentFlags()
	persistentFlags.StringVarP(&root.configPath, "config", "c", "", "Путь к файлу конфигурации")
	persistentFlags.StringVarP(&root.serverAddr, "server", "s", "http://localhost:8080", "Адрес сервера GophKeeper")
	persistentFlags.BoolVarP(&root.verbose, "verbose", "v", false, "Подробный вывод (режим отладки)")

	// Bind flags to viper
	if err := viper.BindPFlag("config", persistentFlags.Lookup("config")); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка привязки флага config: %v\n", err)
		os.Exit(1)
	}
	if err := viper.BindPFlag("server", persistentFlags.Lookup("server")); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка привязки флага server: %v\n", err)
		os.Exit(1)
	}
	if err := viper.BindPFlag("verbose", persistentFlags.Lookup("verbose")); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка привязки флага verbose: %v\n", err)
		os.Exit(1)
	}

	// Add subcommands
	cmd.AddCommand(
		NewVersionCommand(root),
		NewUnlockCommand(),
		NewChangeMasterPasswordCommand(),
		NewRegisterCommand(),
		NewLoginCommand(),
		NewLogoutCommand(),
		NewAddCommand(),
		NewListCommand(),
		NewGetCommand(),
		NewUpdateCommand(),
		NewDeleteCommand(),
		NewSyncCommand(),
		NewConfigCommand(),
	)

	root.cmd = cmd
	return cmd
}

// Execute runs the root command
func (r *RootCmd) Execute() error {
	return r.cmd.Execute()
}

// PreRunE performs initialization before command execution
func (r *RootCmd) PreRunE(cmd *cobra.Command, args []string) error {
	// Load configuration if specified
	if r.configPath != "" {
		viper.SetConfigFile(r.configPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("не удалось загрузить конфигурацию из %s: %w", r.configPath, err)
		}
	}

	// Set default values from environment variables
	viper.SetEnvPrefix("GOPHKEEPER")
	viper.AutomaticEnv()

	return nil
}

// GetServerAddr returns the configured server address
func (r *RootCmd) GetServerAddr() string {
	return viper.GetString("server")
}

// IsVerbose returns whether verbose mode is enabled
func (r *RootCmd) IsVerbose() bool {
	return viper.GetBool("verbose")
}

// GetVersion returns the client version
func (r *RootCmd) GetVersion() string {
	return r.version
}

// GetGitCommit returns the git commit hash
func (r *RootCmd) GetGitCommit() string {
	return r.gitCommit
}

// GetBuildDate returns the build date
func (r *RootCmd) GetBuildDate() string {
	return r.buildDate
}
