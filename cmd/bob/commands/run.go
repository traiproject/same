package commands

import (
	"github.com/spf13/cobra"
)

func (c *CLI) newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [targets...]",
		Short: "Run specified tasks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Display command usage help without returning an error
				_ = cmd.Help()
				return nil
			}
			return c.app.Run(cmd.Context(), args)
		},
	}
}
