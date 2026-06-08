package api

import (
	"net/http"

	"astro-scheduler/internal/node"
	"astro-scheduler/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NodeHandler struct {
	manager *node.NodeManager
}

func NewNodeHandler(m *node.NodeManager) *NodeHandler {
	return &NodeHandler{manager: m}
}

type RegisterNodeRequest struct {
	ID           string                `json:"id"`
	Name         string                `json:"name" binding:"required"`
	Address      string                `json:"address" binding:"required"`
	Capabilities models.NodeCapability `json:"capabilities"`
	Weight       int                   `json:"weight"`
	Tags         []string              `json:"tags"`
}

func (h *NodeHandler) RegisterNode(c *gin.Context) {
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Weight <= 0 {
		req.Weight = 100
	}

	node := &models.Node{
		ID:           req.ID,
		Name:         req.Name,
		Address:      req.Address,
		Capabilities: req.Capabilities,
		Weight:       req.Weight,
		Tags:         req.Tags,
	}

	if err := h.manager.RegisterNode(node); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, node)
}

func (h *NodeHandler) DeregisterNode(c *gin.Context) {
	nodeID := c.Param("id")

	if err := h.manager.DeregisterNode(nodeID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "node deregistered"})
}

func (h *NodeHandler) GetNode(c *gin.Context) {
	nodeID := c.Param("id")
	node, exists := h.manager.GetNode(nodeID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	c.JSON(http.StatusOK, node)
}

func (h *NodeHandler) ListNodes(c *gin.Context) {
	status := models.NodeStatus(c.Query("status"))
	nodes := h.manager.ListNodes(status)

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"total": len(nodes),
	})
}

func (h *NodeHandler) Heartbeat(c *gin.Context) {
	var heartbeat models.NodeHeartbeat
	if err := c.ShouldBindJSON(&heartbeat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if heartbeat.NodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id is required"})
		return
	}

	if err := h.manager.ProcessHeartbeat(&heartbeat); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "heartbeat received"})
}

func (h *NodeHandler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
