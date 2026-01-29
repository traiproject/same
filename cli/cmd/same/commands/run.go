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
			inspectOnError, _ := cmd.Flags().GetBool("inspect-on-error")
			outputMode, _ := cmd.Flags().GetString("output-mode")
			ci, _ := cmd.Flags().GetBool("ci")
			noDaemon, _ := cmd.Flags().GetBool("no-daemon")

			// If --ci is set, override output-mode to "linear"
			if ci {
				outputMode = "linear"
			}

			return c.app.Run(cmd.Context(), args, app.RunOptions{
				NoCache:        noCache,
				Inspect:        inspect,
				InspectOnError: inspectOnError,
				OutputMode:     outputMode,
				NoDaemon:       noDaemon,
			})
		},
	}
	cmd.Flags().BoolP("no-cache", "n", false, "Bypass the build cache and force execution")
	cmd.Flags().BoolP("inspect", "i", false, "Inspect the TUI after build completion (prevents auto-exit)")
	cmd.Flags().Bool("inspect-on-error", true, "Keep TUI open if build fails")
	cmd.Flags().StringP("output-mode", "o", "auto", "Output mode: auto, tui, or linear")
	cmd.Flags().Bool("ci", false, "Use linear output mode (shorthand for --output-mode=linear)")
	cmd.Flags().Bool("no-daemon", false, "Bypass remote daemon execution and run locally")
	return cmd
}
