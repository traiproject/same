package tui

import "github.com/vito/progrock"

// MsgVertexStarted is sent when a new vertex (task) begins execution.
type MsgVertexStarted struct {
	ID   string
	Name string
}

// MsgVertexCompleted is sent when a vertex finishes execution.
type MsgVertexCompleted struct {
	ID  string
	Err error
}

// MsgLogReceived is sent when a log line is received from a vertex.
type MsgLogReceived struct {
	VertexID string
	Stream   progrock.LogStream
	Text     string
}

// MsgTapeUpdate wraps the raw update from progrock.
type MsgTapeUpdate struct {
	Update *progrock.StatusUpdate
}

// MsgTapeEnded is sent when the tape stream has ended.
type MsgTapeEnded struct{}
