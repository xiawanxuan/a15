package node

import (
	"math/rand"
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
	if len(nodes) == 0 {
		return nil
	}

	availableNodes := make([]*models.Node, 0)
	for _, node := range nodes {
		if node.Status == models.NodeStatusOnline || node.Status == models.NodeStatusIdle {
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
	case "random":
		return lb.random(availableNodes)
	default:
		return lb.leastConnections(availableNodes)
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
