package scheduler

import (
	"errors"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*models.Task
	events []*models.TaskEvent
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks:  make(map[string]*models.Task),
		events: make([]*models.TaskEvent, 0),
	}
}

func (s *TaskStore) AddTask(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return errors.New("task already exists")
	}

	s.tasks[task.ID] = task
	s.addEvent(task.ID, "created", task.Status, "", "Task created")
	return nil
}

func (s *TaskStore) GetTask(taskID string) (*models.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	return task, exists
}

func (s *TaskStore) UpdateTaskStatus(taskID string, status models.TaskStatus, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return errors.New("task not found")
	}

	task.Status = status
	now := time.Now()

	switch status {
	case models.TaskStatusRunning:
		task.StartedAt = &now
	case models.TaskStatusCompleted:
		task.CompletedAt = &now
	case models.TaskStatusFailed:
		task.FailedAt = &now
		task.ErrorMessage = message
	}

	s.addEvent(taskID, "status_changed", status, task.AssignedNode, message)
	return nil
}

func (s *TaskStore) AssignTask(taskID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return errors.New("task not found")
	}

	task.AssignedNode = nodeID
	task.Status = models.TaskStatusQueued
	s.addEvent(taskID, "assigned", models.TaskStatusQueued, nodeID, "Task assigned to node")
	return nil
}

func (s *TaskStore) IncrementRetry(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return errors.New("task not found")
	}

	task.RetryCount++
	task.Status = models.TaskStatusRetrying
	s.addEvent(taskID, "retry", models.TaskStatusRetrying, task.AssignedNode, "Task retry initiated")
	return nil
}

func (s *TaskStore) ListTasks(status models.TaskStatus, limit, offset int) []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.Task, 0)
	count := 0

	for _, task := range s.tasks {
		if status != "" && task.Status != status {
			continue
		}

		if count < offset {
			count++
			continue
		}

		result = append(result, task)
		count++

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

func (s *TaskStore) GetTaskEvents(taskID string, limit int) []*models.TaskEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.TaskEvent, 0)
	count := 0

	for i := len(s.events) - 1; i >= 0; i-- {
		if s.events[i].TaskID == taskID {
			result = append([]*models.TaskEvent{s.events[i]}, result...)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}

	return result
}

func (s *TaskStore) addEvent(taskID, eventType string, status models.TaskStatus, nodeID, message string) {
	event := &models.TaskEvent{
		TaskID:    taskID,
		EventType: eventType,
		Status:    status,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Message:   message,
	}
	s.events = append(s.events, event)
}

func (s *TaskStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":     len(s.tasks),
		"pending":   0,
		"queued":    0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"retrying":  0,
		"cancelled": 0,
	}

	for _, task := range s.tasks {
		switch task.Status {
		case models.TaskStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case models.TaskStatusQueued:
			stats["queued"] = stats["queued"].(int) + 1
		case models.TaskStatusRunning:
			stats["running"] = stats["running"].(int) + 1
		case models.TaskStatusCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case models.TaskStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		case models.TaskStatusRetrying:
			stats["retrying"] = stats["retrying"].(int) + 1
		case models.TaskStatusCancelled:
			stats["cancelled"] = stats["cancelled"].(int) + 1
		}
	}

	return stats
}
