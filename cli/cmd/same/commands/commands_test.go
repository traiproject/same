package commands_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.trai.ch/same/cmd/same/commands"
	"go.trai.ch/same/internal/app"
	"go.trai.ch/same/internal/build"
)

type mockApp struct {
	runFunc func(ctx context.Context, targetNames []string, opts app.RunOptions) error
}

func (m *mockApp) Run(ctx context.Context, targetNames []string, opts app.RunOptions) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, targetNames, opts)
	}
	return nil
}

func TestCommands_Run(t *testing.T) {
	t.Run("wires flags correctly", func(t *testing.T) {
		var capturedOpts app.RunOptions
		var capturedTargets []string
		called := false

		mock := &mockApp{
			runFunc: func(_ context.Context, targetNames []string, opts app.RunOptions) error {
				capturedOpts = opts
				capturedTargets = targetNames
				called = true
				return nil
			},
		}

		cli := commands.New(mock)
		cli.SetArgs([]string{"run", "build", "--no-cache", "--inspect"})

		// We don't care about output here, just flag propagation
		err := cli.Execute(context.Background())
		require.NoError(t, err)
		assert.True(t, called)
		assert.True(t, capturedOpts.NoCache)
		assert.True(t, capturedOpts.Inspect)
		assert.Equal(t, []string{"build"}, capturedTargets)
	})

	t.Run("returns error on run failure", func(t *testing.T) {
		mock := &mockApp{
			runFunc: func(_ context.Context, _ []string, _ app.RunOptions) error {
				return errors.New("simulated error")
			},
		}

		cli := commands.New(mock)
		cli.SetArgs([]string{"run", "target"})
		// Silence output to avoid polluting test logs
		cli.SetOutput(new(bytes.Buffer), new(bytes.Buffer))

		err := cli.Execute(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated error")
	})

	t.Run("shows usage when no targets provided", func(t *testing.T) {
		mock := &mockApp{
			runFunc: func(_ context.Context, _ []string, _ app.RunOptions) error {
				panic("should not be called")
			},
		}

		cli := commands.New(mock)
		buf := new(bytes.Buffer)
		cli.SetOutput(buf, buf)
		cli.SetArgs([]string{"run"})

		err := cli.Execute(context.Background())
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Usage:")
	})
}

func TestCommands_Version(t *testing.T) {
	mock := &mockApp{}
	cli := commands.New(mock)

	buf := new(bytes.Buffer)
	cli.SetOutput(buf, buf)
	cli.SetArgs([]string{"version"})

	err := cli.Execute(context.Background())
	require.NoError(t, err)

	assert.Contains(t, buf.String(), build.Version)
}
