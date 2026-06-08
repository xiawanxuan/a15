package api

import (
	"net/http"
	"strconv"

	"astro-scheduler/internal/notifier"
	"astro-scheduler/pkg/models"

	"github.com/gin-gonic/gin"
)

type AlertHandler struct {
	notifier *notifier.Notifier
}

func NewAlertHandler(n *notifier.Notifier) *AlertHandler {
	return &AlertHandler{notifier: n}
}

type CreateAlertRequest struct {
	Type     models.AlertType     `json:"type" binding:"required"`
	Severity models.AlertSeverity `json:"severity" binding:"required"`
	Title    string               `json:"title" binding:"required"`
	Message  string               `json:"message" binding:"required"`
	TaskID   string               `json:"task_id"`
	NodeID   string               `json:"node_id"`
}

func (h *AlertHandler) CreateAlert(c *gin.Context) {
	var req CreateAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	alert, err := h.notifier.CreateAlert(req.Type, req.Severity, req.Title, req.Message, req.TaskID, req.NodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, alert)
}

func (h *AlertHandler) GetAlert(c *gin.Context) {
	alertID := c.Param("id")
	alert, exists := h.notifier.GetAlert(alertID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}

	c.JSON(http.StatusOK, alert)
}

func (h *AlertHandler) ListAlerts(c *gin.Context) {
	severity := models.AlertSeverity(c.Query("severity"))
	resolved := c.Query("resolved") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	alerts := h.notifier.ListAlerts(severity, resolved, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
		"limit":  limit,
		"offset": offset,
	})
}

func (h *AlertHandler) ResolveAlert(c *gin.Context) {
	alertID := c.Param("id")

	if err := h.notifier.ResolveAlert(alertID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "alert resolved"})
}

func (h *AlertHandler) GetStats(c *gin.Context) {
	stats := h.notifier.GetStats()
	c.JSON(http.StatusOK, stats)
}
