// Package commands implements the CLI commands for the bob build tool.
package commands

import (
	"context"

	"github.com/spf13/cobra"
	"go.trai.ch/bob/internal/app"
)

// CLI represents the command line interface for bob.
type CLI struct {
	app     *app.App
	rootCmd *cobra.Command
}

// New creates a new CLI instance with the given app.
func New(a *app.App) *CLI {
	rootCmd := &cobra.Command{
		Use:           "bob",
		Short:         "A modern build tool for monorepos",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add persistent flags
	rootCmd.PersistentFlags().StringP("config", "c", "bob.yaml", "Path to configuration file")

	c := &CLI{
		app:     a,
		rootCmd: rootCmd,
	}

	rootCmd.AddCommand(c.newRunCmd())
	rootCmd.AddCommand(c.newVersionCmd())

	return c
}

// Execute runs the root command with the given context.
func (c *CLI) Execute(ctx context.Context) error {
	c.rootCmd.SetContext(ctx)
	return c.rootCmd.Execute()
}

// GetConfigPath returns the value of the config flag.
func (c *CLI) GetConfigPath() string {
	config, _ := c.rootCmd.PersistentFlags().GetString("config")
	return config
}

// SetArgs sets the arguments for the root command. Used for testing.
func (c *CLI) SetArgs(args []string) {
	c.rootCmd.SetArgs(args)
}
