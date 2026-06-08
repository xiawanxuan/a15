package archiver

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type DataStore struct {
	mu    sync.RWMutex
	data  map[string]*models.ObservationData
	policies map[string]*models.ArchivePolicy
}

func NewDataStore() *DataStore {
	return &DataStore{
		data:     make(map[string]*models.ObservationData),
		policies: make(map[string]*models.ArchivePolicy),
	}
}

func (s *DataStore) AddData(data *models.ObservationData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[data.ID]; exists {
		return errors.New("data already exists")
	}

	s.data[data.ID] = data
	return nil
}

func (s *DataStore) GetData(dataID string) (*models.ObservationData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.data[dataID]
	return data, exists
}

func (s *DataStore) UpdateData(data *models.ObservationData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[data.ID]; !exists {
		return errors.New("data not found")
	}

	s.data[data.ID] = data
	return nil
}

func (s *DataStore) ListData(target string, format models.DataFormat, status models.ArchiveStatus, limit, offset int) []*models.ObservationData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.ObservationData, 0)
	count := 0

	for _, data := range s.data {
		if target != "" && data.Target != target {
			continue
		}
		if format != "" && data.Format != format {
			continue
		}
		if status != "" && data.ArchiveStatus != status {
			continue
		}

		if count < offset {
			count++
			continue
		}

		result = append(result, data)
		count++

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

func (s *DataStore) GetDataByTask(taskID string) []*models.ObservationData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.ObservationData, 0)
	for _, data := range s.data {
		if data.TaskID == taskID {
			result = append(result, data)
		}
	}

	return result
}

func (s *DataStore) MarkArchived(dataID string, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.data[dataID]
	if !exists {
		return errors.New("data not found")
	}

	now := time.Now()
	data.ArchiveStatus = models.ArchiveStatusArchived
	data.ArchivedAt = &now
	data.FilePath = filePath

	return nil
}

func (s *DataStore) MarkFailed(dataID string, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, exists := s.data[dataID]
	if !exists {
		return errors.New("data not found")
	}

	data.ArchiveStatus = models.ArchiveStatusFailed
	return nil
}

func (s *DataStore) GetPendingArchive() []*models.ObservationData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.ObservationData, 0)
	for _, data := range s.data {
		if data.ArchiveStatus == models.ArchiveStatusPending {
			result = append(result, data)
		}
	}

	return result
}

func (s *DataStore) AddPolicy(policy *models.ArchivePolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.policies[policy.ID]; exists {
		return errors.New("policy already exists")
	}

	s.policies[policy.ID] = policy
	return nil
}

func (s *DataStore) GetPolicy(policyID string) (*models.ArchivePolicy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, exists := s.policies[policyID]
	return policy, exists
}

func (s *DataStore) ListPolicies() []*models.ArchivePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.ArchivePolicy, 0, len(s.policies))
	for _, policy := range s.policies {
		result = append(result, policy)
	}

	return result
}

func (s *DataStore) DeletePolicy(policyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.policies[policyID]; !exists {
		return errors.New("policy not found")
	}

	delete(s.policies, policyID)
	return nil
}

func (s *DataStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":      len(s.data),
		"pending":    0,
		"archived":   0,
		"failed":     0,
		"total_size": int64(0),
	}

	for _, data := range s.data {
		switch data.ArchiveStatus {
		case models.ArchiveStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case models.ArchiveStatusArchived:
			stats["archived"] = stats["archived"].(int) + 1
		case models.ArchiveStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		}
		stats["total_size"] = stats["total_size"].(int64) + data.Size
	}

	return stats
}
