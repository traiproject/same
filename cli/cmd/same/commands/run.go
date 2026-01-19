package commands

import (
	"github.com/spf13/cobra"
	"go.trai.ch/same/internal/app"
)

func (c *CLI) newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [targets...]",
		Short: "Run specified tasks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Display command usage help without returning an error
				_ = cmd.Help()
				return nil
			}
			noCache, _ := cmd.Flags().GetBool("no-cache")
			inspect, _ := cmd.Flags().GetBool("inspect")
			return c.app.Run(cmd.Context(), args, app.RunOptions{
				NoCache: noCache,
				Inspect: inspect,
			})
		},
	}
	cmd.Flags().BoolP("no-cache", "n", false, "Bypass the build cache and force execution")
	cmd.Flags().BoolP("inspect", "i", false, "Inspect the TUI after build completion (prevents auto-exit)")
	return cmd
}
