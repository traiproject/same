package scheduler

import (
	"context"
	"maps"

	"go.trai.ch/bob/internal/core/domain"
)

// GetTaskStatusMap returns a copy of the internal task status map.
// This is exported for testing purposes only.
func (s *Scheduler) GetTaskStatusMap() map[domain.InternedString]TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statusMap := make(map[domain.InternedString]TaskStatus, len(s.taskStatus))
	maps.Copy(statusMap, s.taskStatus)
	return statusMap
}

// CheckTaskCache exports checkTaskCache for testing purposes.
func (s *Scheduler) CheckTaskCache(
	ctx context.Context,
	task *domain.Task,
	root string,
) (skipped bool, hash string, err error) {
	return s.checkTaskCache(ctx, task, root)
}
