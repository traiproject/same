package domain

import "time"

// BuildInfo represents the build information for a task.
type BuildInfo struct {
	TaskName   string    `json:"task_name,omitzero"`
	InputHash  string    `json:"input_hash,omitzero"`
	OutputHash string    `json:"output_hash,omitzero"`
	Timestamp  time.Time `json:"timestamp,omitzero"`
}
