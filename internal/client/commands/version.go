// Package commands provides CLI commands for GophKeeper client application.
package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// NewVersionCommand creates a new command to display version information.
func NewVersionCommand(root *RootCmd) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Отображает информацию о версии и дате сборки",
		Long:  `Показывает версию клиента GophKeeper, дату сборки, git commit и другую информацию о системе.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Display version information
			fmt.Printf("GophKeeper CLI Client\n")
			fmt.Printf("======================\n\n")

			// Version
			fmt.Printf("Версия:    %s\n", root.version)

			// Git commit
			if root.gitCommit != "" {
				fmt.Printf("Git commit: %s\n", root.gitCommit)
			} else {
				fmt.Printf("Git commit: unknown\n")
			}

			// Build date
			if root.buildDate != "" {
				fmt.Printf("Дата сборки: %s\n", root.buildDate)
			} else {
				fmt.Printf("Дата сборки: unknown\n")
			}

			// Go version
			fmt.Printf("Go версия:  %s\n", runtime.Version())

			// OS/Arch
			fmt.Printf("ОС/Арх:    %s/%s\n", runtime.GOOS, runtime.GOARCH)

			return nil
		},
	}

	return cmd
}

// VersionInfo holds version-related information.
type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	GoVersion string `json:"go_version"`
	GOOS      string `json:"go_os"`
	GOARCH    string `json:"go_arch"`
}

// GetVersionInfo returns version information as a struct.
func GetVersionInfo(root *RootCmd) VersionInfo {
	return VersionInfo{
		Version:   root.version,
		GitCommit: root.gitCommit,
		BuildDate: root.buildDate,
		GoVersion: runtime.Version(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
	}
}
