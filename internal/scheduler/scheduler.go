package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"astro-scheduler/pkg/lock"
	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type NodeManager interface {
	GetAvailableNodes() []*models.Node
	GetNode(nodeID string) (*models.Node, bool)
	UpdateNodeTaskCount(nodeID string, delta int) error
}

type AlertManager interface {
	CreateAlert(alertType models.AlertType, severity models.AlertSeverity, title, message, taskID, nodeID string) (*models.Alert, error)
}

type Scheduler struct {
	store        *TaskStore
	nodeManager  NodeManager
	alertManager AlertManager
	lock         lock.DistributedLock
	queue        PriorityQueue
	mu           sync.Mutex
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	dispatchCh   chan *models.Task
	retryCh      chan *models.Task
	eventCh      chan *models.TaskEvent
	lockTTL      time.Duration
}

func NewScheduler(store *TaskStore, nodeManager NodeManager, alertManager AlertManager, distLock lock.DistributedLock) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		store:        store,
		nodeManager:  nodeManager,
		alertManager: alertManager,
		lock:         distLock,
		queue:        make(PriorityQueue, 0),
		running:      false,
		ctx:          ctx,
		cancel:       cancel,
		dispatchCh:   make(chan *models.Task, 100),
		retryCh:      make(chan *models.Task, 50),
		eventCh:      make(chan *models.TaskEvent, 200),
		lockTTL:      10 * time.Second,
	}
}

func (s *Scheduler) Start() {
	s.running = true
	utils.Sugar.Info("Scheduler started")

	go s.dispatchLoop()
	go s.retryLoop()
	go s.eventLoop()
}

func (s *Scheduler) Stop() {
	s.running = false
	s.cancel()
	utils.Sugar.Info("Scheduler stopped")
}

func (s *Scheduler) SubmitTask(task *models.Task) error {
	if err := s.store.AddTask(task); err != nil {
		return err
	}

	s.mu.Lock()
	heap.Push(&s.queue, task)
	s.mu.Unlock()

	utils.Sugar.Infof("Task %s submitted to queue", task.ID)
	return nil
}

func (s *Scheduler) dispatchLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.dispatchTasks()
		}
	}
}

func (s *Scheduler) dispatchTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.queue.Len() == 0 {
		return
	}

	nodes := s.nodeManager.GetAvailableNodes()
	if len(nodes) == 0 {
		utils.Sugar.Warn("No available nodes for task dispatch")
		return
	}

	for s.queue.Len() > 0 {
		task := s.queue.Peek()
		if task == nil {
			break
		}

		lockKey := fmt.Sprintf("task:dispatch:%s", task.ID)
		locked, err := s.lock.TryLock(s.ctx, lockKey, s.lockTTL)
		if err != nil {
			utils.Sugar.Errorf("Failed to acquire lock for task %s: %v", task.ID, err)
			break
		}
		if !locked {
			task = heap.Pop(&s.queue).(*models.Task)
			utils.Sugar.Debugf("Task %s already being dispatched by another instance, skipping", task.ID)
			continue
		}

		node := s.selectBestNode(task, nodes)
		if node == nil {
			utils.Sugar.Warnf("No suitable node found for task %s", task.ID)
			_ = s.lock.Unlock(s.ctx, lockKey)
			break
		}

		task = heap.Pop(&s.queue).(*models.Task)

		if err := s.store.AssignTask(task.ID, node.ID); err != nil {
			utils.Sugar.Errorf("Failed to assign task %s: %v", task.ID, err)
			heap.Push(&s.queue, task)
			_ = s.lock.Unlock(s.ctx, lockKey)
			continue
		}

		if err := s.nodeManager.UpdateNodeTaskCount(node.ID, 1); err != nil {
			utils.Sugar.Errorf("Failed to update node task count: %v", err)
		}

		s.dispatchCh <- task
		_ = s.lock.Unlock(s.ctx, lockKey)
		utils.Sugar.Infof("Task %s dispatched to node %s", task.ID, node.ID)
	}
}

func (s *Scheduler) selectBestNode(task *models.Task, nodes []*models.Node) *models.Node {
	var bestNode *models.Node
	minLoad := -1

	for _, node := range nodes {
		if node.Status != models.NodeStatusOnline && node.Status != models.NodeStatusIdle {
			continue
		}

		load := node.Stats.RunningTasks * 100 / node.Weight
		if minLoad == -1 || load < minLoad {
			minLoad = load
			bestNode = node
		}
	}

	return bestNode
}

func (s *Scheduler) retryLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-s.retryCh:
			s.handleRetry(task)
		}
	}
}

func (s *Scheduler) handleRetry(task *models.Task) {
	retryLockKey := fmt.Sprintf("task:retry:%s", task.ID)
	locked, err := s.lock.TryLock(s.ctx, retryLockKey, 30*time.Second)
	if err != nil {
		utils.Sugar.Errorf("Failed to acquire retry lock for task %s: %v", task.ID, err)
		return
	}
	if !locked {
		utils.Sugar.Debugf("Task %s retry already in progress by another instance", task.ID)
		return
	}
	defer func() {
		_ = s.lock.Unlock(s.ctx, retryLockKey)
	}()

	currentTask, exists := s.store.GetTask(task.ID)
	if !exists {
		return
	}

	if currentTask.RetryCount >= currentTask.MaxRetries {
		utils.Sugar.Errorf("Task %s exceeded max retries (%d)", task.ID, currentTask.MaxRetries)

		if s.alertManager != nil {
			_, _ = s.alertManager.CreateAlert(
				models.AlertTypeTaskFailed,
				models.AlertSeverityCritical,
				fmt.Sprintf("Task Failed: %s", currentTask.Name),
				fmt.Sprintf("Task %s failed after %d retries. Error: %s", task.ID, currentTask.RetryCount, currentTask.ErrorMessage),
				task.ID,
				currentTask.AssignedNode,
			)
		}

		return
	}

	if err := s.store.IncrementRetry(task.ID); err != nil {
		utils.Sugar.Errorf("Failed to increment retry count for task %s: %v", task.ID, err)
		return
	}

	updatedTask, _ := s.store.GetTask(task.ID)
	retryDelay := time.Duration(updatedTask.RetryCount) * 5 * time.Second
	utils.Sugar.Infof("Task %s will be retried in %v (retry %d/%d)", task.ID, retryDelay, updatedTask.RetryCount, updatedTask.MaxRetries)

	if s.alertManager != nil {
		_, _ = s.alertManager.CreateAlert(
			models.AlertTypeTaskRetry,
			models.AlertSeverityWarning,
			fmt.Sprintf("Task Retry: %s", updatedTask.Name),
			fmt.Sprintf("Task %s is being retried (%d/%d)", task.ID, updatedTask.RetryCount, updatedTask.MaxRetries),
			task.ID,
			updatedTask.AssignedNode,
		)
	}

	go func() {
		time.Sleep(retryDelay)

		queuedTask, exists := s.store.GetTask(task.ID)
		if !exists {
			return
		}

		s.mu.Lock()
		heap.Push(&s.queue, queuedTask)
		s.mu.Unlock()

		utils.Sugar.Infof("Task %s re-queued for retry", task.ID)
	}()
}

func (s *Scheduler) eventLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.eventCh:
			utils.Sugar.Debugf("Task event: %s - %s", event.TaskID, event.EventType)
		}
	}
}

func (s *Scheduler) MarkTaskRunning(taskID, nodeID string) error {
	lockKey := fmt.Sprintf("task:status:%s", taskID)
	locked, err := s.lock.TryLock(s.ctx, lockKey, s.lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("task status update in progress by another instance")
	}
	defer func() {
		_ = s.lock.Unlock(s.ctx, lockKey)
	}()

	if err := s.store.UpdateTaskStatus(taskID, models.TaskStatusRunning, ""); err != nil {
		return err
	}
	utils.Sugar.Infof("Task %s is now running on node %s", taskID, nodeID)
	return nil
}

func (s *Scheduler) MarkTaskCompleted(taskID, dataID string) error {
	lockKey := fmt.Sprintf("task:status:%s", taskID)
	locked, err := s.lock.TryLock(s.ctx, lockKey, s.lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("task status update in progress by another instance")
	}
	defer func() {
		_ = s.lock.Unlock(s.ctx, lockKey)
	}()

	task, exists := s.store.GetTask(taskID)
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == models.TaskStatusCompleted {
		utils.Sugar.Warnf("Task %s already completed", taskID)
		return nil
	}

	if task.AssignedNode != "" {
		if err := s.nodeManager.UpdateNodeTaskCount(task.AssignedNode, -1); err != nil {
			utils.Sugar.Errorf("Failed to decrement node task count: %v", err)
		}
		s.nodeManager.IncrementCompletedTasks(task.AssignedNode)
	}

	if err := s.store.UpdateTaskStatus(taskID, models.TaskStatusCompleted, ""); err != nil {
		return err
	}

	task, _ = s.store.GetTask(taskID)
	task.DataID = dataID

	utils.Sugar.Infof("Task %s completed successfully", taskID)
	return nil
}

func (s *Scheduler) MarkTaskFailed(taskID, errorMessage string) error {
	lockKey := fmt.Sprintf("task:status:%s", taskID)
	locked, err := s.lock.TryLock(s.ctx, lockKey, s.lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("task status update in progress by another instance")
	}
	defer func() {
		_ = s.lock.Unlock(s.ctx, lockKey)
	}()

	task, exists := s.store.GetTask(taskID)
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status == models.TaskStatusFailed || task.Status == models.TaskStatusCompleted {
		utils.Sugar.Warnf("Task %s already in final state: %s", taskID, task.Status)
		return nil
	}

	if task.AssignedNode != "" {
		if err := s.nodeManager.UpdateNodeTaskCount(task.AssignedNode, -1); err != nil {
			utils.Sugar.Errorf("Failed to decrement node task count: %v", err)
		}
		s.nodeManager.IncrementFailedTasks(task.AssignedNode)
	}

	if err := s.store.UpdateTaskStatus(taskID, models.TaskStatusFailed, errorMessage); err != nil {
		return err
	}

	task, _ = s.store.GetTask(taskID)
	s.retryCh <- task

	utils.Sugar.Warnf("Task %s failed: %s", taskID, errorMessage)
	return nil
}

func (s *Scheduler) GetDispatchChannel() <-chan *models.Task {
	return s.dispatchCh
}

func (s *Scheduler) GetTask(taskID string) (*models.Task, bool) {
	return s.store.GetTask(taskID)
}

func (s *Scheduler) ListTasks(status models.TaskStatus, limit, offset int) []*models.Task {
	return s.store.ListTasks(status, limit, offset)
}

func (s *Scheduler) GetTaskEvents(taskID string, limit int) []*models.TaskEvent {
	return s.store.GetTaskEvents(taskID, limit)
}

func (s *Scheduler) GetStats() map[string]interface{} {
	return s.store.GetStats()
}
