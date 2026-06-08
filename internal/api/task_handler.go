package api

import (
	"net/http"
	"strconv"
	"time"

	"astro-scheduler/internal/scheduler"
	"astro-scheduler/pkg/models"

	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	scheduler *scheduler.Scheduler
}

func NewTaskHandler(s *scheduler.Scheduler) *TaskHandler {
	return &TaskHandler{scheduler: s}
}

type CreateTaskRequest struct {
	Name       string              `json:"name" binding:"required"`
	Type       models.TaskType     `json:"type" binding:"required"`
	Priority   models.TaskPriority `json:"priority"`
	Target     string              `json:"target" binding:"required"`
	Duration   int                 `json:"duration"`
	Payload    string              `json:"payload"`
	MaxRetries int                 `json:"max_retries"`
}

func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Priority == 0 {
		req.Priority = models.TaskPriorityMedium
	}
	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}

	task := models.NewTask(
		req.Name,
		req.Type,
		req.Priority,
		req.Target,
		req.Duration,
		req.Payload,
		req.MaxRetries,
	)

	if err := h.scheduler.SubmitTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	task, exists := h.scheduler.GetTask(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) ListTasks(c *gin.Context) {
	status := models.TaskStatus(c.Query("status"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	tasks := h.scheduler.ListTasks(status, limit, offset)
	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"total":  len(tasks),
		"limit":  limit,
		"offset": offset,
	})
}

func (h *TaskHandler) GetTaskEvents(c *gin.Context) {
	taskID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	events := h.scheduler.GetTaskEvents(taskID, limit)
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

type UpdateTaskStatusRequest struct {
	Status   models.TaskStatus `json:"status" binding:"required"`
	NodeID   string            `json:"node_id"`
	Message  string            `json:"message"`
	DataID   string            `json:"data_id"`
}

func (h *TaskHandler) UpdateTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	var req UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Status {
	case models.TaskStatusRunning:
		if err := h.scheduler.MarkTaskRunning(taskID, req.NodeID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case models.TaskStatusCompleted:
		if err := h.scheduler.MarkTaskCompleted(taskID, req.DataID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case models.TaskStatusFailed:
		if err := h.scheduler.MarkTaskFailed(taskID, req.Message); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	task, _ := h.scheduler.GetTask(taskID)
	c.JSON(http.StatusOK, task)
}

func (h *TaskHandler) GetStats(c *gin.Context) {
	stats := h.scheduler.GetStats()
	c.JSON(http.StatusOK, stats)
}

type ExportLogsRequest struct {
	TaskIDs    []string `json:"task_ids"`
	TaskStatus string   `json:"task_status"`
	NodeID     string   `json:"node_id"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	EventType  string   `json:"event_type"`
	Format     string   `json:"format"`
}

func (h *TaskHandler) ExportLogs(c *gin.Context) {
	var req ExportLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := scheduler.LogExportQuery{
		TaskIDs:    req.TaskIDs,
		TaskStatus: models.TaskStatus(req.TaskStatus),
		NodeID:     req.NodeID,
		EventType:  req.EventType,
		Format:     req.Format,
	}

	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			query.StartTime = &t
		}
	}

	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			query.EndTime = &t
		}
	}

	data, contentType, err := h.scheduler.ExportTaskLogs(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := "task_logs"
	if query.Format == "csv" {
		filename += ".csv"
	} else {
		filename += ".json"
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

func (h *TaskHandler) GetLogSummary(c *gin.Context) {
	var req ExportLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := scheduler.LogExportQuery{
		TaskIDs:    req.TaskIDs,
		TaskStatus: models.TaskStatus(req.TaskStatus),
		NodeID:     req.NodeID,
		EventType:  req.EventType,
	}

	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			query.StartTime = &t
		}
	}

	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			query.EndTime = &t
		}
	}

	summary := h.scheduler.GetLogSummary(query)
	c.JSON(http.StatusOK, summary)
}
