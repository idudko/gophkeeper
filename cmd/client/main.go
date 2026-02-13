// Package main provides GophKeeper CLI client application.
package main

import (
	"os"

	"github.com/idudko/gophkeeper/internal/client/commands"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// Version is client version
	Version = "1.0.0"
)

// Build information (set by linker flags)
var (
	gitCommit string
	buildDate string
	goVersion string
)

func init() {
	// Setup zerolog for structured logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Set log level based on environment
	if os.Getenv("GOPHKEEPER_DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Add version info to context
	log.Logger = log.With().
		Str("version", Version).
		Str("component", "client").Logger()
}

func main() {
	// Initialize root command with version and build info
	rootCmd := commands.NewRootCommand(Version, gitCommit, buildDate)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("Command failed")
		os.Exit(1)
	}
}
