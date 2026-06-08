package node

import (
	"context"
	"errors"
	"fmt"
	"time"

	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type AlertManager interface {
	CreateAlert(alertType models.AlertType, severity models.AlertSeverity, title, message, taskID, nodeID string) (*models.Alert, error)
}

type NodeManager struct {
	store            *NodeStore
	alertManager     AlertManager
	loadBalancer     *LoadBalancer
	ctx              context.Context
	cancel           context.CancelFunc
	heartbeatTimeout time.Duration
	checkInterval    time.Duration
}

func NewNodeManager(store *NodeStore, alertManager AlertManager) *NodeManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &NodeManager{
		store:            store,
		alertManager:     alertManager,
		loadBalancer:     NewLoadBalancer("priority_aware"),
		ctx:              ctx,
		cancel:           cancel,
		heartbeatTimeout: 30 * time.Second,
		checkInterval:    10 * time.Second,
	}
}

func (m *NodeManager) Start() {
	utils.Sugar.Info("Node manager started")
	go m.healthCheckLoop()
}

func (m *NodeManager) Stop() {
	m.cancel()
	utils.Sugar.Info("Node manager stopped")
}

func (m *NodeManager) RegisterNode(node *models.Node) error {
	if node.Weight <= 0 {
		node.Weight = 100
	}
	if err := m.store.RegisterNode(node); err != nil {
		return err
	}

	utils.Sugar.Infof("Node %s registered successfully", node.ID)
	return nil
}

func (m *NodeManager) DeregisterNode(nodeID string) error {
	if err := m.store.DeregisterNode(nodeID); err != nil {
		return err
	}

	utils.Sugar.Infof("Node %s deregistered", nodeID)
	return nil
}

func (m *NodeManager) GetNode(nodeID string) (*models.Node, bool) {
	return m.store.GetNode(nodeID)
}

func (m *NodeManager) ListNodes(status models.NodeStatus) []*models.Node {
	return m.store.ListNodes(status)
}

func (m *NodeManager) GetAvailableNodes() []*models.Node {
	nodes := m.store.ListNodes("")
	available := make([]*models.Node, 0)

	for _, node := range nodes {
		if node.Status == models.NodeStatusOnline || node.Status == models.NodeStatusIdle || node.Status == models.NodeStatusBusy {
			available = append(available, node)
		}
	}

	return available
}

func (m *NodeManager) ProcessHeartbeat(heartbeat *models.NodeHeartbeat) error {
	if heartbeat.Timestamp.IsZero() {
		heartbeat.Timestamp = time.Now()
	}

	if err := m.store.UpdateHeartbeat(heartbeat); err != nil {
		return err
	}

	utils.Sugar.Debugf("Heartbeat received from node %s", heartbeat.NodeID)
	return nil
}

func (m *NodeManager) UpdateNodeTaskCount(nodeID string, delta int) error {
	return m.store.UpdateTaskCount(nodeID, delta)
}

func (m *NodeManager) IncrementCompletedTasks(nodeID string) {
	m.store.IncrementCompleted(nodeID)
}

func (m *NodeManager) IncrementFailedTasks(nodeID string) {
	m.store.IncrementFailed(nodeID)
}

func (m *NodeManager) healthCheckLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkNodeHealth()
		}
	}
}

func (m *NodeManager) checkNodeHealth() {
	offlineNodes := m.store.CheckOfflineNodes(m.heartbeatTimeout)

	for _, node := range offlineNodes {
		utils.Sugar.Warnf("Node %s is offline (last seen: %v)", node.ID, node.LastSeen)
		m.store.MarkNodeOffline(node.ID)

		if m.alertManager != nil {
			_, _ = m.alertManager.CreateAlert(
				models.AlertTypeNodeOffline,
				models.AlertSeverityError,
				fmt.Sprintf("Node Offline: %s", node.Name),
				fmt.Sprintf("Node %s has been offline for more than %v", node.ID, m.heartbeatTimeout),
				"",
				node.ID,
			)
		}
	}
}

func (m *NodeManager) GetStats() map[string]interface{} {
	return m.store.GetStats()
}

func (m *NodeManager) SetHeartbeatTimeout(timeout time.Duration) {
	m.heartbeatTimeout = timeout
}

func (m *NodeManager) SetCheckInterval(interval time.Duration) {
	m.checkInterval = interval
}

func (m *NodeManager) SetLoadBalancerStrategy(strategy string) {
	m.loadBalancer = NewLoadBalancer(strategy)
}

func (m *NodeManager) GetBestNodeForTask(task *models.Task) (*models.Node, error) {
	nodes := m.GetAvailableNodes()
	if len(nodes) == 0 {
		return nil, errors.New("no available nodes")
	}

	node := m.loadBalancer.SelectNodeWithPriority(nodes, task.Priority)
	if node == nil {
		return nil, errors.New("no suitable node found")
	}
	return node, nil
}

func (m *NodeManager) GetLoadBalancer() *LoadBalancer {
	return m.loadBalancer
}

func (m *NodeManager) SetLoadBalancerStrategy(strategy string) {
	m.loadBalancer.SetStrategy(strategy)
}
