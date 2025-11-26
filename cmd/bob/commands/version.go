package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.trai.ch/bob/internal/build"
)

func (c *CLI) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the application version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(build.Version)
		},
	}
}
