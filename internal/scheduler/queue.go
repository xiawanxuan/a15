package scheduler

import (
	"container/heap"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
)

type PriorityQueue []*models.Task

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	task := x.(*models.Task)
	*pq = append(*pq, task)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return task
}

func (pq *PriorityQueue) Peek() *models.Task {
	if len(*pq) == 0 {
		return nil
	}
	return (*pq)[0]
}
