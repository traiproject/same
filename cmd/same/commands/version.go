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
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("same version %s\n", build.Version)
		},
	}
}
