package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"go.trai.ch/same/api/daemon/v1"
	"go.trai.ch/same/internal/adapters/watcher"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC daemon service.
type Server struct {
	daemonv1.UnimplementedDaemonServiceServer
	lifecycle    *Lifecycle
	cache        *ServerCache
	configLoader ports.ConfigLoader
	envFactory   ports.EnvironmentFactory
	executor     ports.Executor
	watcherSvc   *WatcherService
	grpcServer   *grpc.Server
	listener     net.Listener
}

// WatcherService bundles the watcher, debouncer, and hash cache together.
type WatcherService struct {
	Watcher   ports.Watcher
	Debouncer *watcher.Debouncer
	HashCache ports.InputHashCache
}

// NewServer creates a new daemon server.
func NewServer(lifecycle *Lifecycle) *Server {
	s := &Server{
		lifecycle:  lifecycle,
		grpcServer: grpc.NewServer(),
	}
	daemonv1.RegisterDaemonServiceServer(s.grpcServer, s)
	return s
}

// NewServerWithDeps creates a new daemon server with dependencies for handling graph and environment requests.
func NewServerWithDeps(
	lifecycle *Lifecycle,
	configLoader ports.ConfigLoader,
	envFactory ports.EnvironmentFactory,
	executor ports.Executor,
) *Server {
	s := &Server{
		lifecycle:    lifecycle,
		cache:        NewServerCache(),
		configLoader: configLoader,
		envFactory:   envFactory,
		executor:     executor,
		grpcServer:   grpc.NewServer(),
	}
	daemonv1.RegisterDaemonServiceServer(s.grpcServer, s)
	return s
}

// Serve starts the gRPC server on the UDS.
func (s *Server) Serve(ctx context.Context) error {
	socketPath := domain.DefaultDaemonSocketPath()

	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, domain.DirPerm); err != nil {
		return zerr.Wrap(err, "failed to create daemon directory")
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return zerr.Wrap(err, "failed to remove stale socket")
	}

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return zerr.Wrap(err, "failed to listen on UDS")
	}
	s.listener = lis
	// Note: There's a brief window between socket creation and chmod where
	// the socket has default permissions. This is an acceptable trade-off
	// for code clarity. For defense-in-depth, consider setting umask before
	// Listen if this window becomes a concern.

	if err := os.Chmod(socketPath, domain.SocketPerm); err != nil {
		_ = lis.Close()
		return zerr.Wrap(err, "failed to set socket permissions")
	}

	if err := s.writePIDFile(); err != nil {
		return err
	}

	defer s.cleanup()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.grpcServer.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		s.grpcServer.GracefulStop()
		return ctx.Err()
	case <-s.lifecycle.ShutdownChan():
		s.grpcServer.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) cleanup() {
	_ = os.Remove(domain.DefaultDaemonSocketPath())
	_ = os.Remove(domain.DefaultDaemonPIDPath())
}

// Ping implements DaemonService.Ping.
func (s *Server) Ping(_ context.Context, _ *daemonv1.PingRequest) (*daemonv1.PingResponse, error) {
	return &daemonv1.PingResponse{
		IdleRemainingSeconds: int64(s.lifecycle.IdleRemaining().Seconds()),
	}, nil
}

// Status implements DaemonService.Status.
func (s *Server) Status(_ context.Context, _ *daemonv1.StatusRequest) (*daemonv1.StatusResponse, error) {
	pid := os.Getpid()
	const maxInt32 = 2147483647
	if pid > maxInt32 {
		pid = maxInt32
	}
	return &daemonv1.StatusResponse{
		Running: true,
		//nolint:gosec // G115: Safe conversion - pid is capped to maxInt32 above
		Pid:                  int32(pid),
		UptimeSeconds:        int64(s.lifecycle.Uptime().Seconds()),
		LastActivityUnix:     s.lifecycle.LastActivity().Unix(),
		IdleRemainingSeconds: int64(s.lifecycle.IdleRemaining().Seconds()),
	}, nil
}

// Shutdown implements DaemonService.Shutdown.
func (s *Server) Shutdown(_ context.Context, _ *daemonv1.ShutdownRequest) (*daemonv1.ShutdownResponse, error) {
	s.lifecycle.Shutdown()
	return &daemonv1.ShutdownResponse{Success: true}, nil
}

func (s *Server) writePIDFile() error {
	pidPath := domain.DefaultDaemonPIDPath()
	pid := os.Getpid()
	return os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), domain.PrivateFilePerm)
}

// GetGraph implements DaemonService.GetGraph.
// Note: ctx parameter satisfies the gRPC interface but is not currently used for cancellation
// because configLoader.Load() does not accept context. Future enhancement: add context support
// to ConfigLoader.Load() for proper cancellation propagation.
//
//nolint:revive // ctx satisfies gRPC interface; see above note for future improvement
func (s *Server) GetGraph(ctx context.Context, req *daemonv1.GetGraphRequest) (*daemonv1.GetGraphResponse, error) {
	// Guard: ensure server is configured for graph operations
	if s.cache == nil || s.configLoader == nil {
		return nil, status.Error(codes.FailedPrecondition, "server not configured for graph operations")
	}

	// Convert proto mtimes to map
	clientMtimes := make(map[string]int64)
	for _, mtime := range req.ConfigMtimes {
		clientMtimes[mtime.Path] = mtime.MtimeUnixNano
	}

	// Reset inactivity timer
	s.lifecycle.ResetTimer()

	// Check cache
	if graph, cacheHit := s.cache.GetGraph(req.Cwd, clientMtimes); cacheHit {
		return s.graphToResponse(graph, true), nil
	}

	// Cache miss or stale, load the graph
	graph, err := s.configLoader.Load(req.Cwd)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to load graph")
	}

	// Validate the graph to populate executionOrder for Walk()
	if err := graph.Validate(); err != nil {
		return nil, zerr.Wrap(err, "failed to validate graph")
	}

	// Store in cache
	entry := &domain.GraphCacheEntry{
		Graph:       graph,
		ConfigPaths: make([]string, 0, len(clientMtimes)),
		Mtimes:      clientMtimes,
	}
	for path := range clientMtimes {
		entry.ConfigPaths = append(entry.ConfigPaths, path)
	}
	s.cache.SetGraph(req.Cwd, entry)

	return s.graphToResponse(graph, false), nil
}

// GetEnvironment implements DaemonService.GetEnvironment.
func (s *Server) GetEnvironment(
	ctx context.Context,
	req *daemonv1.GetEnvironmentRequest,
) (*daemonv1.GetEnvironmentResponse, error) {
	// Guard: ensure server is configured for environment operations
	if s.cache == nil || s.envFactory == nil {
		return nil, status.Error(codes.FailedPrecondition, "server not configured for environment operations")
	}

	// Reset inactivity timer
	s.lifecycle.ResetTimer()

	// Check cache
	if envVars, cacheHit := s.cache.GetEnv(req.EnvId); cacheHit {
		return &daemonv1.GetEnvironmentResponse{
			CacheHit: true,
			EnvVars:  envVars,
		}, nil
	}

	// Cache miss, resolve environment
	envVars, err := s.envFactory.GetEnvironment(ctx, req.Tools)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to get environment")
	}

	// Store in cache
	s.cache.SetEnv(req.EnvId, envVars)

	return &daemonv1.GetEnvironmentResponse{
		CacheHit: false,
		EnvVars:  envVars,
	}, nil
}

// graphToResponse converts a domain.Graph to a GetGraphResponse proto message.
func (s *Server) graphToResponse(graph *domain.Graph, cacheHit bool) *daemonv1.GetGraphResponse {
	resp := &daemonv1.GetGraphResponse{
		CacheHit: cacheHit,
		Root:     graph.Root(),
	}

	// Convert tasks
	for task := range graph.Walk() {
		taskProto := &daemonv1.TaskProto{
			Name:            task.Name.String(),
			Command:         task.Command,
			Inputs:          s.internedStringsToStrings(task.Inputs),
			Outputs:         s.internedStringsToStrings(task.Outputs),
			Tools:           task.Tools,
			Dependencies:    s.internedStringsToStrings(task.Dependencies),
			Environment:     task.Environment,
			WorkingDir:      task.WorkingDir.String(),
			RebuildStrategy: string(task.RebuildStrategy),
		}
		resp.Tasks = append(resp.Tasks, taskProto)
	}

	return resp
}

// internedStringsToStrings converts a slice of InternedString to plain strings.
func (s *Server) internedStringsToStrings(interned []domain.InternedString) []string {
	result := make([]string, len(interned))
	for i, str := range interned {
		result[i] = str.String()
	}
	return result
}

// SetWatcherService sets the watcher service for the server.
// This must be called before Serve if the watcher service is needed.
func (s *Server) SetWatcherService(watcherSvc *WatcherService) {
	s.watcherSvc = watcherSvc
}

// GetInputHash implements DaemonService.GetInputHash.
//
//nolint:revive,unparam // ctx satisfies gRPC interface requirement
func (s *Server) GetInputHash(
	ctx context.Context,
	req *daemonv1.GetInputHashRequest,
) (*daemonv1.GetInputHashResponse, error) {
	s.lifecycle.ResetTimer()

	// Guard: ensure watcher service is configured
	if s.watcherSvc == nil {
		return nil, status.Error(codes.FailedPrecondition, "watcher service not initialized")
	}

	// Get the hash result from the cache using the request's context.
	// This avoids race conditions by passing root/env directly.
	result := s.watcherSvc.HashCache.GetInputHash(req.TaskName, req.Root, req.Environment)

	// Convert the ports.InputHashState to the proto enum
	var state daemonv1.GetInputHashResponse_State
	switch result.State {
	case ports.HashReady:
		state = daemonv1.GetInputHashResponse_READY
	case ports.HashPending:
		state = daemonv1.GetInputHashResponse_PENDING
	default:
		state = daemonv1.GetInputHashResponse_UNKNOWN
	}

	return &daemonv1.GetInputHashResponse{
		State: state,
		Hash:  result.Hash,
	}, nil
}

// streamWriter implements io.Writer for streaming task output.
type streamWriter struct {
	stream daemonv1.DaemonService_ExecuteTaskServer
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if err := w.stream.Send(&daemonv1.ExecuteTaskResponse{Data: p}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// getExitCode extracts the exit code from an error.
// It returns 0 for no error, or the actual exit code if the error
// contains one via zerr field, defaulting to 1 for generic errors.
func getExitCode(err error) int {
	if err == nil {
		return 0
	}

	// Check if this is a zerr with an exit_code field
	// zerr implements an interface that allows field extraction
	type fielder interface {
		Field(key string) (interface{}, bool)
	}

	var fieldErr fielder
	if errors.As(err, &fieldErr) {
		if code, found := fieldErr.Field("exit_code"); found {
			if exitCode, ok := code.(int); ok {
				return exitCode
			}
		}
	}

	// Default to exit code 1 for generic errors
	return 1
}

// ExecuteTask implements DaemonService.ExecuteTask.
func (s *Server) ExecuteTask(
	req *daemonv1.ExecuteTaskRequest,
	stream daemonv1.DaemonService_ExecuteTaskServer,
) error {
	// Reset inactivity timer
	s.lifecycle.ResetTimer()

	// Guard: ensure server is configured for task execution
	if s.executor == nil {
		return status.Error(codes.FailedPrecondition, "server not configured for task execution")
	}

	// Reconstruct domain.Task from request
	task := &domain.Task{
		Name:        domain.NewInternedString(req.TaskName),
		Command:     req.Command,
		WorkingDir:  domain.NewInternedString(req.WorkingDir),
		Environment: req.TaskEnvironment,
	}

	// Create streaming writer
	writer := &streamWriter{stream: stream}

	// Execute with PTY (via executor)
	err := s.executor.Execute(stream.Context(), task, req.NixEnvironment, writer, writer)

	// Extract exit code from error
	exitCode := getExitCode(err)

	// Set trailer with exit code
	stream.SetTrailer(metadata.Pairs("x-exit-code", strconv.Itoa(exitCode)))

	// Return error status for non-zero exit
	if exitCode != 0 {
		return status.Errorf(codes.Unknown, "task failed with exit code %d", exitCode)
	}
	return nil
}
