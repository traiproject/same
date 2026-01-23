package commands

import (
	"github.com/spf13/cobra"
	"go.trai.ch/same/internal/app"
)

func (c *CLI) newCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean the build cache and artifacts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			tools, _ := cmd.Flags().GetBool("tools")
			all, _ := cmd.Flags().GetBool("all")

			opts := app.CleanOptions{
				Build: false,
				Tools: false,
			}

			switch {
			case all:
				opts.Build = true
				opts.Tools = true
			case tools:
				opts.Tools = true
			default:
				// Default behavior: clean build artifacts
				opts.Build = true
			}

			return c.app.Clean(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolP("tools", "t", false, "Clean tool resolution and environment caches")
	cmd.Flags().BoolP("all", "a", false, "Clean all caches (build, tools, and environments)")

	return cmd
}
