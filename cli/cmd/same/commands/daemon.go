package commands

import (
	"github.com/spf13/cobra"
)

func (c *CLI) newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background daemon",
	}

	cmd.AddCommand(c.newDaemonServeCmd())
	cmd.AddCommand(c.newDaemonStatusCmd())
	cmd.AddCommand(c.newDaemonStopCmd())

	return cmd
}

func (c *CLI) newDaemonServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "serve",
		Short:  "Start the daemon server (internal use)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.app.ServeDaemon(cmd.Context())
		},
	}
}

func (c *CLI) newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.app.DaemonStatus(cmd.Context())
		},
	}
}

func (c *CLI) newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.app.StopDaemon(cmd.Context())
		},
	}
}
