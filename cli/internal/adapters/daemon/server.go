package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"go.trai.ch/same/api/daemon/v1"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"google.golang.org/grpc"
)

// Server implements the gRPC daemon service.
type Server struct {
	daemonv1.UnimplementedDaemonServiceServer
	lifecycle    *Lifecycle
	cache        *ServerCache
	configLoader ports.ConfigLoader
	envFactory   ports.EnvironmentFactory
	grpcServer   *grpc.Server
	listener     net.Listener
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
) *Server {
	s := &Server{
		lifecycle:    lifecycle,
		cache:        NewServerCache(),
		configLoader: configLoader,
		envFactory:   envFactory,
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
	s.lifecycle.ResetTimer()
	return &daemonv1.PingResponse{
		IdleRemainingSeconds: int64(s.lifecycle.IdleRemaining().Seconds()),
	}, nil
}

// Status implements DaemonService.Status.
func (s *Server) Status(_ context.Context, _ *daemonv1.StatusRequest) (*daemonv1.StatusResponse, error) {
	s.lifecycle.ResetTimer()
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
//
//nolint:revive // ctx is used to satisfy the interface but not actively used in this method
func (s *Server) GetGraph(ctx context.Context, req *daemonv1.GetGraphRequest) (*daemonv1.GetGraphResponse, error) {
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
