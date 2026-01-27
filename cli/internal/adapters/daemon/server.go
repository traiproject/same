package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"go.trai.ch/same/api/daemon/v1"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/zerr"
	"google.golang.org/grpc"
)

// Server implements the gRPC daemon service.
type Server struct {
	daemonv1.UnimplementedDaemonServiceServer
	lifecycle  *Lifecycle
	grpcServer *grpc.Server
	listener   net.Listener
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
