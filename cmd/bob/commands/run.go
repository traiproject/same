package commands

import (
	"github.com/spf13/cobra"
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
			return c.app.Run(cmd.Context(), args, force)
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Force rebuild, bypassing cache")
	return cmd
}
