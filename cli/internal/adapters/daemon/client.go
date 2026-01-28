// Package daemon implements the background daemon adapter for same.
// It provides gRPC server and client for inter-process communication over Unix Domain Sockets.
package daemon

import (
	"context"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"go.trai.ch/same/api/daemon/v1"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	// gRPC backoff configuration for fast connection establishment.
	grpcBaseDelay         = 50 * time.Millisecond
	grpcMaxDelay          = 200 * time.Millisecond
	grpcMinConnectTimeout = 100 * time.Millisecond
	grpcBackoffMultiplier = 1.5
)

// Client implements ports.DaemonClient.
type Client struct {
	conn   *grpc.ClientConn
	client daemonv1.DaemonServiceClient
}

// Dial connects to the daemon over UDS.
// Note: grpc.NewClient returns immediately; actual connection happens lazily on first RPC.
func Dial() (*Client, error) {
	socketPath := domain.DefaultDaemonSocketPath()
	absSocketPath, err := filepath.Abs(socketPath)
	if err != nil {
		return nil, zerr.Wrap(err, "failed to resolve absolute socket path")
	}
	target := "unix://" + absSocketPath

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  grpcBaseDelay,
				Multiplier: grpcBackoffMultiplier,
				MaxDelay:   grpcMaxDelay,
			},
			MinConnectTimeout: grpcMinConnectTimeout,
		}),
	)
	if err != nil {
		return nil, zerr.Wrap(err, "daemon client creation failed")
	}

	client := &Client{
		conn:   conn,
		client: daemonv1.NewDaemonServiceClient(conn),
	}
	return client, nil
}

// Ping implements ports.DaemonClient.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.client.Ping(ctx, &daemonv1.PingRequest{})
	return err
}

// Status implements ports.DaemonClient.
func (c *Client) Status(ctx context.Context) (*ports.DaemonStatus, error) {
	resp, err := c.client.Status(ctx, &daemonv1.StatusRequest{})
	if err != nil {
		return nil, err
	}
	return &ports.DaemonStatus{
		Running:       resp.Running,
		PID:           int(resp.Pid),
		Uptime:        time.Duration(resp.UptimeSeconds) * time.Second,
		LastActivity:  time.Unix(resp.LastActivityUnix, 0),
		IdleRemaining: time.Duration(resp.IdleRemainingSeconds) * time.Second,
	}, nil
}

// Shutdown implements ports.DaemonClient.
func (c *Client) Shutdown(ctx context.Context) error {
	_, err := c.client.Shutdown(ctx, &daemonv1.ShutdownRequest{Graceful: true})
	return err
}

// GetGraph implements ports.DaemonClient.
func (c *Client) GetGraph(
	ctx context.Context,
	cwd string,
	configMtimes map[string]int64,
) (graph *domain.Graph, cacheHit bool, err error) {
	// Build request
	req := &daemonv1.GetGraphRequest{
		Cwd: cwd,
	}
	for path, mtime := range configMtimes {
		req.ConfigMtimes = append(req.ConfigMtimes, &daemonv1.ConfigMtime{
			Path:          path,
			MtimeUnixNano: mtime,
		})
	}

	// Call gRPC
	resp, err := c.client.GetGraph(ctx, req)
	if err != nil {
		return nil, false, zerr.Wrap(err, "GetGraph RPC failed")
	}

	// Convert response to domain.Graph
	graph = domain.NewGraph()
	for _, taskProto := range resp.Tasks {
		task := &domain.Task{
			Name:            domain.NewInternedString(taskProto.Name),
			Command:         taskProto.Command,
			Inputs:          c.stringsToInternedStrings(taskProto.Inputs),
			Outputs:         c.stringsToInternedStrings(taskProto.Outputs),
			Tools:           taskProto.Tools,
			Dependencies:    c.stringsToInternedStrings(taskProto.Dependencies),
			Environment:     taskProto.Environment,
			WorkingDir:      domain.NewInternedString(taskProto.WorkingDir),
			RebuildStrategy: domain.RebuildStrategy(taskProto.RebuildStrategy),
		}
		if err := graph.AddTask(task); err != nil {
			return nil, false, zerr.Wrap(err, "failed to add task to graph")
		}
	}

	// Set root (important: must be set after all tasks are added)
	graph.SetRoot(resp.Root)

	// Validate the graph to compute executionOrder and dependents
	if err := graph.Validate(); err != nil {
		return nil, false, zerr.Wrap(err, "failed to validate reconstructed graph")
	}

	return graph, resp.CacheHit, nil
}

// GetEnvironment implements ports.DaemonClient.
func (c *Client) GetEnvironment(
	ctx context.Context,
	envID string,
	tools map[string]string,
) (envVars []string, cacheHit bool, err error) {
	req := &daemonv1.GetEnvironmentRequest{
		EnvId: envID,
		Tools: tools,
	}

	resp, err := c.client.GetEnvironment(ctx, req)
	if err != nil {
		return nil, false, zerr.Wrap(err, "GetEnvironment RPC failed")
	}

	return resp.EnvVars, resp.CacheHit, nil
}

// GetInputHash implements ports.DaemonClient.
func (c *Client) GetInputHash(
	ctx context.Context,
	taskName, root string,
	env map[string]string,
) (ports.InputHashResult, error) {
	req := &daemonv1.GetInputHashRequest{
		TaskName:    taskName,
		Root:        root,
		Environment: env,
	}

	resp, err := c.client.GetInputHash(ctx, req)
	if err != nil {
		return ports.InputHashResult{State: ports.HashUnknown}, zerr.Wrap(err, "GetInputHash RPC failed")
	}

	// Convert the proto enum to the ports.InputHashState
	var state ports.InputHashState
	switch resp.State {
	case daemonv1.GetInputHashResponse_READY:
		state = ports.HashReady
	case daemonv1.GetInputHashResponse_PENDING:
		state = ports.HashPending
	default:
		state = ports.HashUnknown
	}

	return ports.InputHashResult{
		State: state,
		Hash:  resp.Hash,
	}, nil
}

// ExecuteTask implements ports.DaemonClient.
// Note: stderr is intentionally merged into stdout for PTY mode. This is because
// PTY sessions combine both output streams by design. For non-PTY scenarios,
// consider separate stderr handling in the Executor implementation.
func (c *Client) ExecuteTask(
	ctx context.Context,
	task *domain.Task,
	nixEnv []string,
	stdout, _ io.Writer,
) error {
	// Build request
	const (
		defaultPtyRows = 24
		defaultPtyCols = 80
	)

	req := &daemonv1.ExecuteTaskRequest{
		TaskName:        task.Name.String(),
		Command:         task.Command,
		WorkingDir:      task.WorkingDir.String(),
		TaskEnvironment: task.Environment,
		NixEnvironment:  nixEnv,
		PtyRows:         defaultPtyRows,
		PtyCols:         defaultPtyCols,
	}

	// Start streaming RPC
	stream, err := c.client.ExecuteTask(ctx, req)
	if err != nil {
		return zerr.Wrap(err, "ExecuteTask RPC failed")
	}

	// Receive and forward log chunks
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return c.handleExecuteError(err, stream)
		}
		if _, writeErr := stdout.Write(resp.Data); writeErr != nil {
			return zerr.Wrap(writeErr, "failed to write log chunk")
		}
	}

	// Check trailer for success case
	trailer := stream.Trailer()
	if exitStr := trailer.Get("x-exit-code"); len(exitStr) > 0 {
		exitCode, err := strconv.Atoi(exitStr[0])
		if err != nil {
			return zerr.Wrap(err, "malformed exit code in trailer")
		}
		if exitCode != 0 {
			return zerr.With(domain.ErrTaskExecutionFailed, "exit_code", exitCode)
		}
	}

	return nil
}

// handleExecuteError extracts the exit code from a failed ExecuteTask RPC.
func (c *Client) handleExecuteError(err error, stream grpc.ClientStream) error {
	st, ok := status.FromError(err)
	if !ok {
		return zerr.Wrap(err, "ExecuteTask failed")
	}

	// For non-zero exit codes, we get UNKNOWN status
	if st.Code() == codes.Unknown {
		// Try to extract exit code from trailer
		trailer := stream.Trailer()
		if exitStr := trailer.Get("x-exit-code"); len(exitStr) > 0 {
			exitCode, parseErr := strconv.Atoi(exitStr[0])
			if parseErr != nil {
				wrapped := zerr.Wrap(parseErr, "malformed exit code in trailer")
				return zerr.With(wrapped, "original_error", err.Error())
			}
			return zerr.With(domain.ErrTaskExecutionFailed, "exit_code", exitCode)
		}
		// If no trailer, return the status error
		return zerr.Wrap(err, "ExecuteTask failed with unknown error")
	}

	return zerr.Wrap(err, "ExecuteTask failed")
}

// stringsToInternedStrings converts a slice of strings to InternedString.
func (c *Client) stringsToInternedStrings(strs []string) []domain.InternedString {
	result := make([]domain.InternedString, len(strs))
	for i, s := range strs {
		result[i] = domain.NewInternedString(s)
	}
	return result
}

// Close implements ports.DaemonClient.
func (c *Client) Close() error {
	return c.conn.Close()
}
