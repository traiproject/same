// Package app implements the application layer for bob.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.trai.ch/same/internal/adapters/logger" //nolint:depguard // concrete type assertion
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/same/internal/engine/scheduler"
	"go.trai.ch/zerr"
	"golang.org/x/sync/errgroup"
)

const (
	logDirPerm  = 0o750
	logFilePerm = 0o600
)

// App represents the main application logic.
type App struct {
	configLoader ports.ConfigLoader
	executor     ports.Executor
	logger       ports.Logger
	store        ports.BuildInfoStore
	hasher       ports.Hasher
	resolver     ports.InputResolver
	envFactory   ports.EnvironmentFactory
	teaOptions   []tea.ProgramOption
}

// New creates a new App instance.
func New(
	loader ports.ConfigLoader,
	executor ports.Executor,
	log ports.Logger,
	store ports.BuildInfoStore,
	hasher ports.Hasher,
	resolver ports.InputResolver,
	envFactory ports.EnvironmentFactory,
) *App {
	return &App{
		configLoader: loader,
		executor:     executor,
		logger:       log,
		store:        store,
		hasher:       hasher,
		resolver:     resolver,
		envFactory:   envFactory,
	}
}

// WithTeaOptions adds bubbletea program options to the App.
// This is primarily used for testing to disable input/output.
func (a *App) WithTeaOptions(opts ...tea.ProgramOption) *App {
	a.teaOptions = append(a.teaOptions, opts...)
	return a
}

// RunOptions configuration for the Run method.
type RunOptions struct {
	Force   bool
	Inspect bool
}

// Run executes the build process for the specified targets.
func (a *App) Run(ctx context.Context, targetNames []string, opts RunOptions) error {
	// 0. Redirect Logs for TUI
	// We want to avoid polluting the terminal with app logs while the TUI is running.
	if err := os.MkdirAll(".same", logDirPerm); err != nil {
		return zerr.Wrap(err, "failed to create .same directory")
	}
	f, err := os.OpenFile(".same/debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, logFilePerm)
	if err != nil {
		return zerr.Wrap(err, "failed to open debug log")
	}
	defer func() {
		_ = f.Close()
	}()

	// If we have the concrete logger type, redirect it.
	if l, ok := a.logger.(*logger.Logger); ok {
		l.SetOutput(f)
		defer l.SetOutput(os.Stderr)
	}

	// 1. Load the graph
	graph, err := a.configLoader.Load(".")
	if err != nil {
		return zerr.Wrap(err, "failed to load configuration")
	}

	// 2. Validate targets
	if len(targetNames) == 0 {
		return domain.ErrNoTargetsSpecified
	}

	// 3. Initialize TUI
	// The TUI model holds the state of the UI.
	tuiModel := tui.NewModel()
	// The Program manages the TUI lifecycle.
	// We capture the program to clean it up if needed.
	optsTea := append([]tea.ProgramOption{tea.WithContext(ctx)}, a.teaOptions...)
	program := tea.NewProgram(&tuiModel, optsTea...)

	// 4. Initialize Telemetry
	// Create a bridge that sends OTel spans to the TUI program.
	bridge := telemetry.NewTUIBridge(program)

	// Configure the global OTel SDK to usage our bridge for spans.
	// This ensures that when OTelTracer uses otel.Tracer(), it uses a provider
	// that forwards events to our bridge.
	setupOTel(bridge)

	// Create and configure the OTel Tracer adapter.
	// We inject the program so it can stream logs directly via the batcher.
	tracer := telemetry.NewOTelTracer("same").WithProgram(program)

	// 5. Initialize Scheduler
	sched := scheduler.NewScheduler(
		a.executor,
		a.store,
		a.hasher,
		a.resolver,
		tracer,
		a.envFactory,
	)

	// 6. Run TUI and Scheduler concurrently
	// Use a cancelable context to coordinate shutdown.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// TUI Routine
	g.Go(func() error {
		// Program.Run blocks until the program exits.
		if _, err := program.Run(); err != nil {
			return err
		}
		// If TUI quits first (e.g. user triggers quit), ensure we cancel the scheduler.
		cancel()
		return nil
	})

	// Scheduler Routine
	g.Go(func() error {
		defer func() {
			// Handle panic recovery for the scheduler goroutine
			if r := recover(); r != nil {
				// We can't log easily here as TUI is running, but we should ensure quit.
				// Program shutdown will restore terminal.
				fmt.Printf("Scheduler panic: %v\n", r)
			}
			// Ensure TUI hits tea.Quit when scheduler finishes, UNLESS inspection mode is on.
			if !opts.Inspect {
				program.Quit()
			}
		}()

		if err := sched.Run(ctx, graph, targetNames, runtime.NumCPU(), opts.Force); err != nil {
			return errors.Join(domain.ErrBuildExecutionFailed, err)
		}
		return nil
	})

	return g.Wait()
}

// setupOTel configures the OpenTelemetry SDK with the TUI bridge.
func setupOTel(bridge *telemetry.TUIBridge) {
	// Create a new TracerProvider with the TUI bridge as a SpanProcessor.
	// This ensures that all started spans are reported to the TUI.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bridge),
	)

	// Register it as the global provider.
	otel.SetTracerProvider(tp)
}
