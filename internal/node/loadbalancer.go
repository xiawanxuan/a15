package node

import (
	"math/rand"
	"sort"
	"sync"

	"astro-scheduler/pkg/models"
)

type LoadBalancer struct {
	mu       sync.Mutex
	strategy string
	rrIndex  int
}

func NewLoadBalancer(strategy string) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		rrIndex:  0,
	}
}

func (lb *LoadBalancer) SelectNode(nodes []*models.Node) *models.Node {
	return lb.SelectNodeWithPriority(nodes, models.TaskPriorityMedium)
}

func (lb *LoadBalancer) SelectNodeWithPriority(nodes []*models.Node, priority models.TaskPriority) *models.Node {
	if len(nodes) == 0 {
		return nil
	}

	availableNodes := make([]*models.Node, 0)
	for _, node := range nodes {
		if node.Status == models.NodeStatusOnline || node.Status == models.NodeStatusIdle || node.Status == models.NodeStatusBusy {
			availableNodes = append(availableNodes, node)
		}
	}

	if len(availableNodes) == 0 {
		if len(nodes) > 0 {
			return nodes[0]
		}
		return nil
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	switch lb.strategy {
	case "round_robin":
		return lb.roundRobin(availableNodes)
	case "least_connections":
		return lb.leastConnections(availableNodes)
	case "weighted":
		return lb.weighted(availableNodes)
	case "priority_aware":
		return lb.priorityAware(availableNodes, priority)
	case "random":
		return lb.random(availableNodes)
	default:
		return lb.priorityAware(availableNodes, priority)
	}
}

func (lb *LoadBalancer) roundRobin(nodes []*models.Node) *models.Node {
	if len(nodes) == 0 {
		return nil
	}
	node := nodes[lb.rrIndex%len(nodes)]
	lb.rrIndex++
	return node
}

func (lb *LoadBalancer) leastConnections(nodes []*models.Node) *models.Node {
	if len(nodes) == 0 {
		return nil
	}

	var bestNode *models.Node
	minTasks := -1

	for _, node := range nodes {
		if minTasks == -1 || node.Stats.RunningTasks < minTasks {
			minTasks = node.Stats.RunningTasks
			bestNode = node
		}
	}

	return bestNode
}

func (lb *LoadBalancer) weighted(nodes []*models.Node) *models.Node {
	if len(nodes) == 0 {
		return nil
	}

	totalWeight := 0
	for _, node := range nodes {
		totalWeight += node.Weight
	}

	if totalWeight == 0 {
		return nodes[0]
	}

	r := rand.Intn(totalWeight)
	cumulative := 0

	for _, node := range nodes {
		cumulative += node.Weight
		if r < cumulative {
			return node
		}
	}

	return nodes[len(nodes)-1]
}

func (lb *LoadBalancer) random(nodes []*models.Node) *models.Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[rand.Intn(len(nodes))]
}

func (lb *LoadBalancer) priorityAware(nodes []*models.Node, priority models.TaskPriority) *models.Node {
	if len(nodes) == 0 {
		return nil
	}

	sortedNodes := make([]*models.Node, len(nodes))
	copy(sortedNodes, nodes)

	sort.Slice(sortedNodes, func(i, j int) bool {
		loadI := float64(sortedNodes[i].Stats.RunningTasks) / float64(sortedNodes[i].Weight)
		loadJ := float64(sortedNodes[j].Stats.RunningTasks) / float64(sortedNodes[j].Weight)
		return loadI < loadJ
	})

	priorityWeight := float64(priority) / 10.0
	if priorityWeight > 1.0 {
		priorityWeight = 1.0
	}

	candidates := sortedNodes
	if priority >= models.TaskPriorityHigh && len(sortedNodes) > 1 {
		bestCount := max(1, len(sortedNodes)/2)
		candidates = sortedNodes[:bestCount]
	} else if priority <= models.TaskPriorityLow && len(sortedNodes) > 1 {
		startIdx := len(sortedNodes) / 2
		if startIdx >= len(sortedNodes) {
			startIdx = len(sortedNodes) - 1
		}
		candidates = sortedNodes[startIdx:]
	}

	if len(candidates) == 0 {
		return sortedNodes[0]
	}

	return candidates[rand.Intn(len(candidates))]
}

func (lb *LoadBalancer) SelectNodeByCapability(nodes []*models.Node, taskType models.TaskType, priority models.TaskPriority) *models.Node {
	if len(nodes) == 0 {
		return nil
	}

	filteredNodes := make([]*models.Node, 0)
	for _, node := range nodes {
		if node.Status == models.NodeStatusOffline || node.Status == models.NodeStatusDisabled {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	return lb.SelectNodeWithPriority(filteredNodes, priority)
}

func (lb *LoadBalancer) SetStrategy(strategy string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategy = strategy
}

func (lb *LoadBalancer) GetStrategy() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.strategy
}
