package models

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusRetrying  TaskStatus = "retrying"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type TaskPriority int

const (
	TaskPriorityLow    TaskPriority = 1
	TaskPriorityMedium TaskPriority = 5
	TaskPriorityHigh   TaskPriority = 10
)

type TaskType string

const (
	TaskTypeObservation TaskType = "observation"
	TaskTypeCalibration TaskType = "calibration"
	TaskTypeAnalysis    TaskType = "analysis"
)

type Task struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Type        TaskType     `json:"type" yaml:"type"`
	Priority    TaskPriority `json:"priority" yaml:"priority"`
	Status      TaskStatus   `json:"status" yaml:"status"`
	Target      string       `json:"target" yaml:"target"`
	Duration    int          `json:"duration" yaml:"duration"`
	Payload     string       `json:"payload,omitempty" yaml:"payload,omitempty"`
	AssignedNode string      `json:"assigned_node,omitempty" yaml:"assigned_node,omitempty"`
	RetryCount  int          `json:"retry_count" yaml:"retry_count"`
	MaxRetries  int          `json:"max_retries" yaml:"max_retries"`
	CreatedAt   time.Time    `json:"created_at" yaml:"created_at"`
	StartedAt   *time.Time   `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	FailedAt    *time.Time   `json:"failed_at,omitempty" yaml:"failed_at,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	DataID      string       `json:"data_id,omitempty" yaml:"data_id,omitempty"`
}

func NewTask(name string, taskType TaskType, priority TaskPriority, target string, duration int, payload string, maxRetries int) *Task {
	now := time.Now()
	return &Task{
		ID:         uuid.New().String(),
		Name:       name,
		Type:       taskType,
		Priority:   priority,
		Status:     TaskStatusPending,
		Target:     target,
		Duration:   duration,
		Payload:    payload,
		RetryCount: 0,
		MaxRetries: maxRetries,
		CreatedAt:  now,
	}
}

type TaskEvent struct {
	TaskID    string      `json:"task_id"`
	EventType string      `json:"event_type"`
	Status    TaskStatus  `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
	NodeID    string      `json:"node_id,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}
