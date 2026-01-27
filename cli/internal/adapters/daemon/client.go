// Package daemon implements the background daemon adapter for same.
// It provides gRPC server and client for inter-process communication over Unix Domain Sockets.
package daemon

import (
	"context"
	"time"

	"go.trai.ch/same/api/daemon/v1"
	"go.trai.ch/same/internal/core/domain"
	"go.trai.ch/same/internal/core/ports"
	"go.trai.ch/zerr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	target := "unix://" + socketPath

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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

// Close implements ports.DaemonClient.
func (c *Client) Close() error {
	return c.conn.Close()
}
