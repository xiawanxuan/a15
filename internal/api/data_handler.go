package api

import (
	"net/http"
	"strconv"

	"astro-scheduler/internal/archiver"
	"astro-scheduler/pkg/models"

	"github.com/gin-gonic/gin"
)

type DataHandler struct {
	archiver *archiver.Archiver
}

func NewDataHandler(a *archiver.Archiver) *DataHandler {
	return &DataHandler{archiver: a}
}

type AddDataRequest struct {
	TaskID   string           `json:"task_id" binding:"required"`
	NodeID   string           `json:"node_id" binding:"required"`
	Target   string           `json:"target" binding:"required"`
	Format   models.DataFormat `json:"format" binding:"required"`
	Size     int64            `json:"size"`
	Checksum string           `json:"checksum"`
	Metadata string           `json:"metadata"`
}

func (h *DataHandler) AddData(c *gin.Context) {
	var req AddDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data := models.NewObservationData(
		req.TaskID,
		req.NodeID,
		req.Target,
		req.Format,
		req.Size,
		req.Checksum,
		req.Metadata,
	)

	if err := h.archiver.AddData(data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, data)
}

func (h *DataHandler) GetData(c *gin.Context) {
	dataID := c.Param("id")
	data, exists := h.archiver.GetData(dataID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "data not found"})
		return
	}

	c.JSON(http.StatusOK, data)
}

func (h *DataHandler) ListData(c *gin.Context) {
	target := c.Query("target")
	format := models.DataFormat(c.Query("format"))
	status := models.ArchiveStatus(c.Query("status"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	dataList := h.archiver.ListData(target, format, status, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"data":   dataList,
		"total":  len(dataList),
		"limit":  limit,
		"offset": offset,
	})
}

func (h *DataHandler) GetDataByTask(c *gin.Context) {
	taskID := c.Param("task_id")
	dataList := h.archiver.GetDataByTask(taskID)

	c.JSON(http.StatusOK, gin.H{
		"data":  dataList,
		"total": len(dataList),
	})
}

func (h *DataHandler) GetStats(c *gin.Context) {
	stats := h.archiver.GetStats()
	c.JSON(http.StatusOK, stats)
}

type CreatePolicyRequest struct {
	Name          string `json:"name" binding:"required"`
	RetentionDays int    `json:"retention_days"`
	Compress      bool   `json:"compress"`
	BackupCount   int    `json:"backup_count"`
	StoragePath   string `json:"storage_path"`
}

func (h *DataHandler) CreatePolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := &models.ArchivePolicy{
		ID:            "policy-" + req.Name,
		Name:          req.Name,
		RetentionDays: req.RetentionDays,
		Compress:      req.Compress,
		BackupCount:   req.BackupCount,
		StoragePath:   req.StoragePath,
	}

	if err := h.archiver.AddPolicy(policy); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *DataHandler) ListPolicies(c *gin.Context) {
	policies := h.archiver.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

func (h *DataHandler) DeletePolicy(c *gin.Context) {
	policyID := c.Param("id")

	if err := h.archiver.DeletePolicy(policyID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}
