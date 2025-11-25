package scheduler

import "go.trai.ch/bob/internal/core/domain"

// GetTaskStatusMap returns a copy of the internal task status map.
// This is exported for testing purposes only.
func (s *Scheduler) GetTaskStatusMap() map[domain.InternedString]TaskStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statusMap := make(map[domain.InternedString]TaskStatus, len(s.taskStatus))
	for k, v := range s.taskStatus {
		statusMap[k] = v
	}
	return statusMap
}
