package commands

import (
	"github.com/spf13/cobra"
	"go.trai.ch/bob/internal/app"
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
			force, _ := cmd.Flags().GetBool("force")
			inspect, _ := cmd.Flags().GetBool("inspect")
			return c.app.Run(cmd.Context(), args, app.RunOptions{
				Force:   force,
				Inspect: inspect,
			})
		},
	}
	return cmd
}
