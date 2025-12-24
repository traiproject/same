package commands

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/vito/progrock"
	progrock_adapter "go.trai.ch/bob/internal/adapters/telemetry/progrock" //nolint:depguard // Run command composes specific adapters
	"go.trai.ch/bob/internal/app"
	"go.trai.ch/bob/internal/engine/scheduler"
	"go.trai.ch/bob/internal/tui"
)

// pipeSource adapts progrock.Reader to tui.TapeSource.
type pipeSource struct {
	progrock.Reader
}

func (s *pipeSource) Read() (*progrock.StatusUpdate, error) {
	update, ok := s.ReadStatus()
	if !ok {
		return nil, io.EOF
	}
	// Return the update
	return update, nil
}

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

			// 1. Initialize TUI and Recorder via progrock.Pipe
			// progrock.Pipe returns (Reader, Writer)
			read, write := progrock.Pipe()

			rec := progrock_adapter.NewRecorder(write)
			tuiModel := tui.NewModel(&pipeSource{read})
			program := tea.NewProgram(tuiModel, tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.ErrOrStderr()))

			// 2. Reconstruct Scheduler and App with new telemetry
			sched := scheduler.NewScheduler(
				c.components.Executor,
				c.components.Store,
				c.components.Hasher,
				c.components.Resolver,
				c.components.Logger,
				c.components.EnvFactory,
				rec,
			)

			// Create a new App instance for this run
			runApp := app.New(c.components.ConfigLoader, sched, rec)

			force, _ := cmd.Flags().GetBool("force")

			// 3. Run App and TUI concurrently
			errCh := make(chan error, 1)
			go func() {
				// Close the recorder (and thus the pipe writer) when done
				// This signals EOF to the reader/TUI
				defer func() { _ = rec.Close() }()
				defer program.Quit()

				if err := runApp.Run(cmd.Context(), args, force); err != nil {
					errCh <- err
				}
				close(errCh)
			}()

			// Run TUI (blocking)
			if _, err := program.Run(); err != nil {
				return err
			}

			// Check for app error
			if err := <-errCh; err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Force rebuild, bypassing cache")
	return cmd
}
