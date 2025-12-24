// Package commands implements the CLI commands for the bob build tool.
package commands

import (
	"context"

	"github.com/spf13/cobra"
	"go.trai.ch/bob/internal/app"
)

// CLI represents the command line interface for bob.
type CLI struct {
	components *app.Components
	rootCmd    *cobra.Command
}

// New creates a new CLI instance with the given components.
func New(components *app.Components) *CLI {
	rootCmd := &cobra.Command{
		Use:           "bob",
		Short:         "A modern build tool for monorepos",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	c := &CLI{
		components: components,
		rootCmd:    rootCmd,
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
