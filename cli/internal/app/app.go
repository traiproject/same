// Package app implements the application layer for same.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.trai.ch/same/internal/adapters/daemon"
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
	connector    ports.DaemonConnector
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
	connector ports.DaemonConnector,
) *App {
	return &App{
		configLoader: loader,
		executor:     executor,
		logger:       log,
		store:        store,
		hasher:       hasher,
		resolver:     resolver,
		envFactory:   envFactory,
		connector:    connector,
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

// SetLogJSON enables or disables JSON logging output.
// When enabled, logs are output as JSON. When disabled, pretty-printed logs are used.
func (a *App) SetLogJSON(enable bool) {
	a.logger.SetJSON(enable)
}

// RunOptions configuration for the Run method.
type RunOptions struct {
	NoCache        bool
	Inspect        bool
	InspectOnError bool
	OutputMode     string
	NoDaemon       bool // When true, bypass remote daemon execution
}

// Run executes the build process for the specified targets.
//
//nolint:cyclop // orchestration function
func (a *App) Run(ctx context.Context, targetNames []string, opts RunOptions) error {
	// 0. Get absolute path of current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return zerr.Wrap(err, "failed to get current working directory")
	}

	// 1. Discover workspace root
	root, err := a.configLoader.DiscoverRoot(cwd)
	if err != nil {
		return zerr.Wrap(err, "failed to discover workspace root")
	}

	// 2. Connect to daemon (if available and not disabled) and load graph from daemon or fallback to local
	var graph *domain.Graph
	var client ports.DaemonClient
	var daemonAvailable bool

	if !opts.NoDaemon {
		var clientErr error
		client, clientErr = a.connector.Connect(ctx, root)
		if clientErr == nil && client != nil {
			// Daemon is available, try to get graph from daemon
			daemonAvailable = true
			defer func() {
				_ = client.Close()
			}()

			// Discover config paths and mtimes
			mtimes, mtimeErr := a.configLoader.DiscoverConfigPaths(cwd)
			if mtimeErr != nil {
				return zerr.Wrap(mtimeErr, "failed to discover config paths")
			}

			// Try to get graph from daemon
			graph, _, err = client.GetGraph(ctx, cwd, mtimes)
			if err != nil {
				// On daemon error, we'll fall through to local loading
				graph = nil
			}
		}
	}

	// Load graph locally if not already loaded from daemon
	if graph == nil || opts.NoDaemon {
		graph, err = a.configLoader.Load(cwd)
		if err != nil {
			return zerr.Wrap(err, "failed to load configuration")
		}
	}

	// 3. Validate targets
	if len(targetNames) == 0 {
		return domain.ErrNoTargetsSpecified
	}

	// 4. Initialize Renderer
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

	// 5. Initialize Telemetry
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

	// 6. Initialize Scheduler
	sched := scheduler.NewScheduler(
		a.executor,
		a.store,
		a.hasher,
		a.resolver,
		tracer,
		a.envFactory,
	).WithNoDaemon(opts.NoDaemon)

	// Pass daemon client to scheduler if available
	if daemonAvailable {
		sched.WithDaemon(client)
	}

	// 7. Run Renderer and Scheduler concurrently
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
			// Calculate keepOpen state: renderer should stay open if
			// 1. Inspect mode is enabled OR
			// 2. InspectOnError is enabled AND an error occurred
			keepOpen := opts.Inspect || (opts.InspectOnError && schedErr != nil)
			// Stop renderer if keepOpen is false
			if !keepOpen {
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
	// Discover workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return zerr.Wrap(err, "failed to get current working directory")
	}

	root, err := a.configLoader.DiscoverRoot(cwd)
	if err != nil {
		return zerr.Wrap(err, "failed to discover workspace root")
	}

	var errs error

	// Helper to remove a directory and log the action
	remove := func(path string, name string) {
		// Log what we are doing
		if err := os.RemoveAll(path); err != nil {
			errs = errors.Join(errs, zerr.Wrap(err, fmt.Sprintf("failed to remove %s", name)))
			return
		}
		a.logger.Info(fmt.Sprintf("removed %s", name))
	}

	if options.Build {
		remove(filepath.Join(root, domain.DefaultStorePath()), "build info store")
	}

	if options.Tools {
		remove(filepath.Join(root, domain.DefaultNixHubCachePath()), "nix tool cache")
		remove(filepath.Join(root, domain.DefaultEnvCachePath()), "environment cache")
	}

	return errs
}

// setupOTel configures the OpenTelemetry SDK with the renderer bridge.
func setupOTel(bridge *telemetry.Bridge) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bridge),
	)

	otel.SetTracerProvider(tp)
}

// ServeDaemon starts the daemon server.
func (a *App) ServeDaemon(ctx context.Context) error {
	lifecycle := daemon.NewLifecycle(domain.DaemonInactivityTimeout)
	server := daemon.NewServerWithDeps(
		lifecycle,
		a.configLoader,
		a.envFactory,
		a.executor,
	)

	a.logger.Info("daemon starting")

	if err := server.Serve(ctx); err != nil {
		return zerr.Wrap(err, "daemon server error")
	}

	a.logger.Info("daemon stopped")
	return nil
}

// DaemonStatus returns the current daemon status.
func (a *App) DaemonStatus(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return zerr.Wrap(err, "failed to get current working directory")
	}

	root, err := a.configLoader.DiscoverRoot(cwd)
	if err != nil {
		return zerr.Wrap(err, "failed to discover workspace root")
	}

	if !a.connector.IsRunning(root) {
		a.logger.Info("Running: false")
		return nil
	}

	client, err := a.connector.Connect(ctx, root)
	if err != nil {
		return zerr.Wrap(err, "failed to connect to daemon")
	}
	defer func() {
		_ = client.Close()
	}()

	status, err := client.Status(ctx)
	if err != nil {
		return zerr.Wrap(err, "failed to get daemon status")
	}

	a.logger.Info(fmt.Sprintf("Running: %v", status.Running))
	a.logger.Info(fmt.Sprintf("PID: %d", status.PID))
	a.logger.Info(fmt.Sprintf("Uptime: %v", status.Uptime))
	ago := time.Since(status.LastActivity).Truncate(time.Second)
	a.logger.Info(fmt.Sprintf("Last Activity: %s (%s ago)", status.LastActivity.Format("15:04:05"), ago))
	a.logger.Info(fmt.Sprintf("Idle Remaining: %v", status.IdleRemaining))

	return nil
}

// StopDaemon stops the daemon.
func (a *App) StopDaemon(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return zerr.Wrap(err, "failed to get current working directory")
	}

	root, err := a.configLoader.DiscoverRoot(cwd)
	if err != nil {
		return zerr.Wrap(err, "failed to discover workspace root")
	}

	client, err := a.connector.Connect(ctx, root)
	if err != nil {
		return zerr.Wrap(err, "failed to connect to daemon")
	}
	defer func() {
		_ = client.Close()
	}()

	a.logger.Info("stopping daemon")
	if err := client.Shutdown(ctx); err != nil {
		return zerr.Wrap(err, "failed to stop daemon")
	}

	a.logger.Info("daemon stopped")
	return nil
}
