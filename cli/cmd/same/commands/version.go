package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.trai.ch/same/internal/build"
)

func (c *CLI) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the application version",
		Run: func(cmd *cobra.Command, _ []string) {
			cmdo := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(cmdo, "same version %s (commit: %s, date: %s)\n", build.Version, build.Commit, build.Date)
		},
	}
}
