package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
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

type LogExportQuery struct {
	TaskIDs    []string
	TaskStatus models.TaskStatus
	NodeID     string
	StartTime  *time.Time
	EndTime    *time.Time
	EventType  string
	Format     string
}

func (s *TaskStore) ExportTaskLogs(query LogExportQuery) ([]byte, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filteredEvents []*models.TaskEvent

	for _, event := range s.events {
		if len(query.TaskIDs) > 0 {
			found := false
			for _, id := range query.TaskIDs {
				if event.TaskID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if query.NodeID != "" && event.NodeID != query.NodeID {
			continue
		}

		if query.EventType != "" && event.EventType != query.EventType {
			continue
		}

		if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
			continue
		}

		if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
			continue
		}

		if task, exists := s.tasks[event.TaskID]; exists && query.TaskStatus != "" {
			if task.Status != query.TaskStatus {
				continue
			}
		}

		filteredEvents = append(filteredEvents, event)
	}

	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].Timestamp.Before(filteredEvents[j].Timestamp)
	})

	format := query.Format
	if format == "" {
		format = "json"
	}

	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(filteredEvents, "", "  ")
		return data, "application/json", err
	case "csv":
		return s.exportToCSV(filteredEvents), "text/csv", nil
	default:
		return nil, "", fmt.Errorf("unsupported format: %s", format)
	}
}

func (s *TaskStore) exportToCSV(events []*models.TaskEvent) []byte {
	var buf []byte

	buf = append(buf, []byte("TaskID,EventType,Status,Timestamp,NodeID,Message,Data\n")...)

	for _, event := range events {
		dataStr := ""
		if event.Data != nil {
			dataBytes, _ := json.Marshal(event.Data)
			dataStr = string(dataBytes)
		}

		row := []string{
			event.TaskID,
			event.EventType,
			string(event.Status),
			event.Timestamp.Format(time.RFC3339),
			event.NodeID,
			event.Message,
			dataStr,
		}

		for i, field := range row {
			if i > 0 {
				buf = append(buf, ',')
			}
			if strings.ContainsAny(field, ",\"\n\r") {
				buf = append(buf, '"')
				for _, c := range field {
					if c == '"' {
						buf = append(buf, '"', '"')
					} else {
						buf = append(buf, byte(c))
					}
				}
				buf = append(buf, '"')
			} else {
				buf = append(buf, field...)
			}
		}
		buf = append(buf, '\n')
	}

	return buf
}

type TaskLogSummary struct {
	TotalEvents   int
	TotalTasks    int
	SuccessTasks  int
	FailedTasks   int
	RunningTasks  int
	PendingTasks  int
	AvgDurationMs int64
	MaxDurationMs int64
	MinDurationMs int64
	TotalDuration time.Duration
}

func (s *TaskStore) GetLogSummary(query LogExportQuery) TaskLogSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := TaskLogSummary{}
	taskDurations := make(map[string]int64)
	taskCount := make(map[string]bool)
	successCount := 0
	failedCount := 0
	runningCount := 0
	pendingCount := 0

	for _, task := range s.tasks {
		if len(query.TaskIDs) > 0 {
			found := false
			for _, id := range query.TaskIDs {
				if task.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if query.NodeID != "" && task.AssignedNode != query.NodeID {
			continue
		}

		if query.TaskStatus != "" && task.Status != query.TaskStatus {
			continue
		}

		taskCount[task.ID] = true

		switch task.Status {
		case models.TaskStatusCompleted:
			successCount++
			if task.StartedAt != nil && task.CompletedAt != nil {
				duration := task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
				taskDurations[task.ID] = duration
			}
		case models.TaskStatusFailed:
			failedCount++
		case models.TaskStatusRunning:
			runningCount++
		case models.TaskStatusPending, models.TaskStatusQueued:
			pendingCount++
		}
	}

	first := true
	var totalDuration int64
	for _, duration := range taskDurations {
		if first {
			summary.MinDurationMs = duration
			summary.MaxDurationMs = duration
			first = false
		} else {
			if duration < summary.MinDurationMs {
				summary.MinDurationMs = duration
			}
			if duration > summary.MaxDurationMs {
				summary.MaxDurationMs = duration
			}
		}
		totalDuration += duration
	}

	if len(taskDurations) > 0 {
		summary.AvgDurationMs = totalDuration / int64(len(taskDurations))
		summary.TotalDuration = time.Duration(totalDuration) * time.Millisecond
	}

	summary.TotalEvents = len(s.events)
	summary.TotalTasks = len(taskCount)
	summary.SuccessTasks = successCount
	summary.FailedTasks = failedCount
	summary.RunningTasks = runningCount
	summary.PendingTasks = pendingCount

	return summary
}
