package api

import (
	"net/http"
	"strconv"
	"time"

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

func (h *DataHandler) DownloadData(c *gin.Context) {
	dataID := c.Param("id")

	data, exists := h.archiver.GetData(dataID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "data not found"})
		return
	}

	if data.ArchiveStatus != models.ArchiveStatusArchived {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data not archived yet"})
		return
	}

	url, err := h.archiver.GetPresignedURL(dataID, 15*60*1000000000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if url != "" {
		c.JSON(http.StatusOK, gin.H{
			"download_url": url,
			"expires_in":   900,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   data,
		"bucket": data.FilePath,
	})
}

func (h *DataHandler) GetPresignedURL(c *gin.Context) {
	dataID := c.Param("id")
	expiresStr := c.DefaultQuery("expires", "3600")
	expiresSeconds, err := strconv.Atoi(expiresStr)
	if err != nil {
		expiresSeconds = 3600
	}

	url, err := h.archiver.GetPresignedURL(dataID, time.Duration(expiresSeconds)*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":        url,
		"expires_in": expiresSeconds,
	})
}

type SearchDataRequest struct {
	Target     string            `json:"target"`
	TaskID     string            `json:"task_id"`
	NodeID     string            `json:"node_id"`
	Format     string            `json:"format"`
	Status     string            `json:"status"`
	Telescope  string            `json:"telescope"`
	Filter     string            `json:"filter"`
	Instrument string            `json:"instrument"`
	Metadata   map[string]string `json:"metadata"`
	StartTime  string            `json:"start_time"`
	EndTime    string            `json:"end_time"`
	MinSize    int64             `json:"min_size"`
	MaxSize    int64             `json:"max_size"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	SortBy     string            `json:"sort_by"`
	SortOrder  string            `json:"sort_order"`
}

func (h *DataHandler) SearchData(c *gin.Context) {
	var req SearchDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := archiver.SearchQuery{
		Target:     req.Target,
		TaskID:     req.TaskID,
		NodeID:     req.NodeID,
		Format:     models.DataFormat(req.Format),
		Status:     models.ArchiveStatus(req.Status),
		Telescope:  req.Telescope,
		Filter:     req.Filter,
		Instrument: req.Instrument,
		Metadata:   req.Metadata,
		MinSize:    req.MinSize,
		MaxSize:    req.MaxSize,
		Limit:      req.Limit,
		Offset:     req.Offset,
		SortBy:     req.SortBy,
		SortOrder:  req.SortOrder,
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

	if query.Limit == 0 {
		query.Limit = 50
	}

	dataList, total := h.archiver.SearchData(query)

	c.JSON(http.StatusOK, gin.H{
		"data":   dataList,
		"total":  total,
		"limit":  query.Limit,
		"offset": query.Offset,
	})
}

func (h *DataHandler) GetDistinctTargets(c *gin.Context) {
	targets := h.archiver.GetDistinctTargets()
	c.JSON(http.StatusOK, gin.H{"targets": targets})
}

func (h *DataHandler) GetDistinctTelescopes(c *gin.Context) {
	telescopes := h.archiver.GetDistinctTelescopes()
	c.JSON(http.StatusOK, gin.H{"telescopes": telescopes})
}

func (h *DataHandler) GetDistinctFilters(c *gin.Context) {
	filters := h.archiver.GetDistinctFilters()
	c.JSON(http.StatusOK, gin.H{"filters": filters})
}

func (h *DataHandler) GetMetadataKeys(c *gin.Context) {
	keys := h.archiver.GetMetadataKeys()
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

func (h *DataHandler) GetMetadataValues(c *gin.Context) {
	key := c.Param("key")
	values := h.archiver.GetMetadataValues(key)
	c.JSON(http.StatusOK, gin.H{"values": values})
}
