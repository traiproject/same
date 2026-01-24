// Package app implements the application layer for same.
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
	"go.trai.ch/same/internal/adapters/detector"
	"go.trai.ch/same/internal/adapters/linear"
	"go.trai.ch/same/internal/adapters/telemetry"
	"go.trai.ch/same/internal/adapters/tui"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/same/internal/engine/scheduler"
	"go.trai.ch/zerr"
	"golang.org/x/sync/errgroup"
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
	disableTick  bool
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

// WithDisableTick disables the TUI tick loop.
// This is primarily used for testing with synctest to avoid goroutine deadlocks.
func (a *App) WithDisableTick() *App {
	a.disableTick = true
	return a
}

// RunOptions configuration for the Run method.
type RunOptions struct {
	NoCache    bool
	Inspect    bool
	OutputMode string
}

// Run executes the build process for the specified targets.
//
//nolint:cyclop // orchestration function
func (a *App) Run(ctx context.Context, targetNames []string, opts RunOptions) error {
	// 1. Load the graph
	graph, err := a.configLoader.Load(".")
	if err != nil {
		return zerr.Wrap(err, "failed to load configuration")
	}

	// 2. Validate targets
	if len(targetNames) == 0 {
		return domain.ErrNoTargetsSpecified
	}

	// 3. Initialize Renderer
	// Detect environment and resolve output mode
	autoMode := detector.DetectEnvironment()
	mode := detector.ResolveMode(autoMode, opts.OutputMode)

	var renderer ports.Renderer
	if mode == detector.ModeTUI {
		model := tui.NewModel(os.Stderr)
		if a.disableTick {
			model = model.WithDisableTick()
		}
		optsTea := append([]tea.ProgramOption{tea.WithContext(ctx)}, a.teaOptions...)
		renderer = tui.NewRenderer(&model, optsTea...)
	} else {
		renderer = linear.NewRenderer(os.Stdout, os.Stderr)
	}

	// 4. Initialize Telemetry
	// Create a bridge that sends OTel spans to the renderer.
	bridge := telemetry.NewBridge(renderer)

	// Configure the global OTel SDK to use our bridge for spans.
	// This ensures that when OTelTracer uses otel.Tracer(), it uses a provider
	// that forwards events to our bridge.
	setupOTel(bridge)

	// Create and configure the OTel Tracer adapter.
	// We inject the renderer so it can stream logs directly via the batcher.
	tracer := telemetry.NewOTelTracer("same").WithRenderer(renderer)
	defer func() {
		_ = tracer.Shutdown(ctx)
	}()

	// 5. Initialize Scheduler
	sched := scheduler.NewScheduler(
		a.executor,
		a.store,
		a.hasher,
		a.resolver,
		tracer,
		a.envFactory,
	)

	// 6. Run Renderer and Scheduler concurrently
	g, ctx := errgroup.WithContext(ctx)

	// Renderer Routine
	g.Go(func() error {
		if err := renderer.Start(ctx); err != nil {
			return err
		}
		// Wait blocks until the renderer has terminated.
		return renderer.Wait()
	})

	// Scheduler Routine
	g.Go(func() error {
		defer func() {
			// Handle panic recovery for the scheduler goroutine
			if r := recover(); r != nil {
				// Print panic info before renderer shutdown
				fmt.Fprintf(os.Stderr, "Scheduler panic: %v\n", r)
			}
			// Ensure renderer stops when scheduler finishes, UNLESS inspection mode is on.
			if !opts.Inspect {
				_ = renderer.Stop()
			}
		}()

		if err := sched.Run(ctx, graph, targetNames, runtime.NumCPU(), opts.NoCache); err != nil {
			return errors.Join(domain.ErrBuildExecutionFailed, err)
		}
		return nil
	})

	return g.Wait()
}

// CleanOptions configuration for the Clean method.
type CleanOptions struct {
	Build bool
	Tools bool
}

// Clean removes cache and build artifacts based on the provided options.
func (a *App) Clean(_ context.Context, options CleanOptions) error {
	var errs error

	// Helper to remove a directory and log the action
	remove := func(path string, name string) {
		// Log what we are doing
		a.logger.Info(fmt.Sprintf("removing %s...", name))
		if err := os.RemoveAll(path); err != nil {
			errs = errors.Join(errs, zerr.Wrap(err, fmt.Sprintf("failed to remove %s", name)))
			return
		}
		a.logger.Info(fmt.Sprintf("removed %s", name))
	}

	if options.Build {
		remove(domain.DefaultStorePath(), "build info store")
	}

	if options.Tools {
		remove(domain.DefaultNixHubCachePath(), "nix tool cache")
		remove(domain.DefaultEnvCachePath(), "environment cache")
	}

	return errs
}

// setupOTel configures the OpenTelemetry SDK with the renderer bridge.
func setupOTel(bridge *telemetry.Bridge) {
	// Create a new TracerProvider with the bridge as a SpanProcessor.
	// This ensures that all started spans are reported to the renderer.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bridge),
	)

	// Register it as the global provider.
	otel.SetTracerProvider(tp)
}
