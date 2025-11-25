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
			return c.app.Run(cmd.Context(), args)
		},
	}
}
