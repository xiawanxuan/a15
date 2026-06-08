package node

import (
	"errors"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
)

type NodeStore struct {
	mu    sync.RWMutex
	nodes map[string]*models.Node
}

func NewNodeStore() *NodeStore {
	return &NodeStore{
		nodes: make(map[string]*models.Node),
	}
}

func (s *NodeStore) RegisterNode(node *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[node.ID]; exists {
		return errors.New("node already registered")
	}

	node.RegisteredAt = time.Now()
	node.LastSeen = time.Now()
	node.Status = models.NodeStatusOnline
	s.nodes[node.ID] = node
	return nil
}

func (s *NodeStore) GetNode(nodeID string) (*models.Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, exists := s.nodes[nodeID]
	return node, exists
}

func (s *NodeStore) UpdateNode(node *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[node.ID]; !exists {
		return errors.New("node not found")
	}

	s.nodes[node.ID] = node
	return nil
}

func (s *NodeStore) DeregisterNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; !exists {
		return errors.New("node not found")
	}

	delete(s.nodes, nodeID)
	return nil
}

func (s *NodeStore) ListNodes(status models.NodeStatus) []*models.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.Node, 0)
	for _, node := range s.nodes {
		if status != "" && node.Status != status {
			continue
		}
		result = append(result, node)
	}

	return result
}

func (s *NodeStore) UpdateHeartbeat(heartbeat *models.NodeHeartbeat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[heartbeat.NodeID]
	if !exists {
		return errors.New("node not found")
	}

	node.LastSeen = heartbeat.Timestamp
	node.Status = heartbeat.Status
	node.Stats.CPUUsage = heartbeat.CPUUsage
	node.Stats.MemoryUsage = heartbeat.MemoryUsage
	node.Stats.RunningTasks = heartbeat.RunningTasks
	node.Stats.LastHeartbeat = heartbeat.Timestamp

	return nil
}

func (s *NodeStore) UpdateTaskCount(nodeID string, delta int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return errors.New("node not found")
	}

	if delta > 0 {
		node.Stats.TotalTasks += delta
	}
	node.Stats.RunningTasks += delta
	if node.Stats.RunningTasks < 0 {
		node.Stats.RunningTasks = 0
	}

	if node.Stats.RunningTasks > 0 {
		node.Status = models.NodeStatusBusy
	} else if node.Status == models.NodeStatusBusy {
		node.Status = models.NodeStatusIdle
	}

	return nil
}

func (s *NodeStore) IncrementCompleted(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[nodeID]; exists {
		node.Stats.CompletedTasks++
	}
}

func (s *NodeStore) IncrementFailed(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[nodeID]; exists {
		node.Stats.FailedTasks++
	}
}

func (s *NodeStore) CheckOfflineNodes(timeout time.Duration) []*models.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	offlineNodes := make([]*models.Node, 0)
	now := time.Now()

	for _, node := range s.nodes {
		if node.Status == models.NodeStatusOffline || node.Status == models.NodeStatusDisabled {
			continue
		}

		if now.Sub(node.LastSeen) > timeout {
			offlineNodes = append(offlineNodes, node)
		}
	}

	return offlineNodes
}

func (s *NodeStore) MarkNodeOffline(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[nodeID]; exists {
		node.Status = models.NodeStatusOffline
	}
}

func (s *NodeStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":    len(s.nodes),
		"online":   0,
		"offline":  0,
		"busy":     0,
		"idle":     0,
		"disabled": 0,
	}

	for _, node := range s.nodes {
		switch node.Status {
		case models.NodeStatusOnline:
			stats["online"] = stats["online"].(int) + 1
		case models.NodeStatusOffline:
			stats["offline"] = stats["offline"].(int) + 1
		case models.NodeStatusBusy:
			stats["busy"] = stats["busy"].(int) + 1
		case models.NodeStatusIdle:
			stats["idle"] = stats["idle"].(int) + 1
		case models.NodeStatusDisabled:
			stats["disabled"] = stats["disabled"].(int) + 1
		}
	}

	return stats
}
