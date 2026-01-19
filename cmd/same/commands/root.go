// Package commands implements the CLI commands for the same build tool.
package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/build"
)

// CLI represents the command line interface for same.
type CLI struct {
	app     *app.App
	rootCmd *cobra.Command
}

// New creates a new CLI instance with the given app.
func New(a *app.App) *CLI {
	rootCmd := &cobra.Command{
		Use:           "same",
		Short:         "A modern build tool for monorepos",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       build.Version,
	}

	rootCmd.SetVersionTemplate(fmt.Sprintf(
		"{{.Name}} version {{.Version}} (commit: %s, date: %s)\n",
		build.Commit,
		build.Date,
	))
	rootCmd.InitDefaultVersionFlag()
	rootCmd.Flags().Lookup("version").Usage = "Print the application version"

	rootCmd.InitDefaultHelpFlag()
	rootCmd.Flags().Lookup("help").Usage = "Show help for command"

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

// SetArgs sets the arguments for the root command. Used for testing.
func (c *CLI) SetArgs(args []string) {
	c.rootCmd.SetArgs(args)
}
