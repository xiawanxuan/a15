package archiver

import (
	"errors"
	"sort"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
)

type DataStore struct {
	mu       sync.RWMutex
	data     map[string]*models.ObservationData
	policies map[string]*models.ArchivePolicy

	targetIndex    map[string][]string
	taskIndex      map[string][]string
	nodeIndex      map[string][]string
	formatIndex    map[models.DataFormat][]string
	telescopeIndex map[string][]string
	filterIndex    map[string][]string
	metadataIndex  map[string]map[string][]string
}

func NewDataStore() *DataStore {
	return &DataStore{
		data:           make(map[string]*models.ObservationData),
		policies:       make(map[string]*models.ArchivePolicy),
		targetIndex:    make(map[string][]string),
		taskIndex:      make(map[string][]string),
		nodeIndex:      make(map[string][]string),
		formatIndex:    make(map[models.DataFormat][]string),
		telescopeIndex: make(map[string][]string),
		filterIndex:    make(map[string][]string),
		metadataIndex:  make(map[string]map[string][]string),
	}
}

func (s *DataStore) AddData(data *models.ObservationData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[data.ID]; exists {
		return errors.New("data already exists")
	}

	s.data[data.ID] = data
	s.addToIndexes(data)
	return nil
}

func (s *DataStore) addToIndexes(data *models.ObservationData) {
	s.targetIndex[data.Target] = append(s.targetIndex[data.Target], data.ID)
	s.taskIndex[data.TaskID] = append(s.taskIndex[data.TaskID], data.ID)
	s.nodeIndex[data.NodeID] = append(s.nodeIndex[data.NodeID], data.ID)
	s.formatIndex[data.Format] = append(s.formatIndex[data.Format], data.ID)

	if data.Telescope != "" {
		s.telescopeIndex[data.Telescope] = append(s.telescopeIndex[data.Telescope], data.ID)
	}
	if data.Filter != "" {
		s.filterIndex[data.Filter] = append(s.filterIndex[data.Filter], data.ID)
	}

	if data.MetadataMap != nil {
		for key, value := range data.MetadataMap {
			if s.metadataIndex[key] == nil {
				s.metadataIndex[key] = make(map[string][]string)
			}
			s.metadataIndex[key][value] = append(s.metadataIndex[key][value], data.ID)
		}
	}
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

type SearchQuery struct {
	Target     string
	TaskID     string
	NodeID     string
	Format     models.DataFormat
	Status     models.ArchiveStatus
	Telescope  string
	Filter     string
	Instrument string
	Metadata   map[string]string
	StartTime  *time.Time
	EndTime    *time.Time
	MinSize    int64
	MaxSize    int64
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

func (s *DataStore) SearchData(query SearchQuery) ([]*models.ObservationData, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidateIDs := make(map[string]bool)
	hasFilter := false

	if query.Target != "" {
		ids, exists := s.targetIndex[query.Target]
		if exists {
			for _, id := range ids {
				candidateIDs[id] = true
			}
			hasFilter = true
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if query.TaskID != "" {
		ids, exists := s.taskIndex[query.TaskID]
		if exists {
			if !hasFilter {
				for _, id := range ids {
					candidateIDs[id] = true
				}
				hasFilter = true
			} else {
				newCandidates := make(map[string]bool)
				for _, id := range ids {
					if candidateIDs[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if query.NodeID != "" {
		ids, exists := s.nodeIndex[query.NodeID]
		if exists {
			if !hasFilter {
				for _, id := range ids {
					candidateIDs[id] = true
				}
				hasFilter = true
			} else {
				newCandidates := make(map[string]bool)
				for _, id := range ids {
					if candidateIDs[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if query.Format != "" {
		ids, exists := s.formatIndex[query.Format]
		if exists {
			if !hasFilter {
				for _, id := range ids {
					candidateIDs[id] = true
				}
				hasFilter = true
			} else {
				newCandidates := make(map[string]bool)
				for _, id := range ids {
					if candidateIDs[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if query.Telescope != "" {
		ids, exists := s.telescopeIndex[query.Telescope]
		if exists {
			if !hasFilter {
				for _, id := range ids {
					candidateIDs[id] = true
				}
				hasFilter = true
			} else {
				newCandidates := make(map[string]bool)
				for _, id := range ids {
					if candidateIDs[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if query.Filter != "" {
		ids, exists := s.filterIndex[query.Filter]
		if exists {
			if !hasFilter {
				for _, id := range ids {
					candidateIDs[id] = true
				}
				hasFilter = true
			} else {
				newCandidates := make(map[string]bool)
				for _, id := range ids {
					if candidateIDs[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		} else {
			return []*models.ObservationData{}, 0
		}
	}

	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			if valueMap, keyExists := s.metadataIndex[key]; keyExists {
				if ids, valueExists := valueMap[value]; valueExists {
					if !hasFilter {
						for _, id := range ids {
							candidateIDs[id] = true
						}
						hasFilter = true
					} else {
						newCandidates := make(map[string]bool)
						for _, id := range ids {
							if candidateIDs[id] {
								newCandidates[id] = true
							}
						}
						candidateIDs = newCandidates
					}
				} else {
					return []*models.ObservationData{}, 0
				}
			} else {
				return []*models.ObservationData{}, 0
			}
		}
	}

	if !hasFilter {
		for id := range s.data {
			candidateIDs[id] = true
		}
	}

	results := make([]*models.ObservationData, 0)
	for id := range candidateIDs {
		data, exists := s.data[id]
		if !exists {
			continue
		}

		if query.Status != "" && data.ArchiveStatus != query.Status {
			continue
		}

		if query.StartTime != nil && data.ObservationTime.Before(*query.StartTime) {
			continue
		}

		if query.EndTime != nil && data.ObservationTime.After(*query.EndTime) {
			continue
		}

		if query.MinSize > 0 && data.Size < query.MinSize {
			continue
		}

		if query.MaxSize > 0 && data.Size > query.MaxSize {
			continue
		}

		results = append(results, data)
	}

	sortBy := query.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := query.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case "target":
			if sortOrder == "asc" {
				return results[i].Target < results[j].Target
			}
			return results[i].Target > results[j].Target
		case "size":
			if sortOrder == "asc" {
				return results[i].Size < results[j].Size
			}
			return results[i].Size > results[j].Size
		case "observation_time":
			if sortOrder == "asc" {
				return results[i].ObservationTime.Before(results[j].ObservationTime)
			}
			return results[i].ObservationTime.After(results[j].ObservationTime)
		default:
			if sortOrder == "asc" {
				return results[i].CreatedAt.Before(results[j].CreatedAt)
			}
			return results[i].CreatedAt.After(results[j].CreatedAt)
		}
	})

	total := len(results)

	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	} else if query.Offset >= len(results) {
		return []*models.ObservationData{}, total
	}

	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results, total
}

func (s *DataStore) GetDistinctTargets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targets := make([]string, 0, len(s.targetIndex))
	for target := range s.targetIndex {
		targets = append(targets, target)
	}
	return targets
}

func (s *DataStore) GetDistinctTelescopes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	telescopes := make([]string, 0, len(s.telescopeIndex))
	for telescope := range s.telescopeIndex {
		telescopes = append(telescopes, telescope)
	}
	return telescopes
}

func (s *DataStore) GetDistinctFilters() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filters := make([]string, 0, len(s.filterIndex))
	for filter := range s.filterIndex {
		filters = append(filters, filter)
	}
	return filters
}

func (s *DataStore) GetMetadataKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.metadataIndex))
	for key := range s.metadataIndex {
		keys = append(keys, key)
	}
	return keys
}

func (s *DataStore) GetMetadataValues(key string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	valueMap, exists := s.metadataIndex[key]
	if !exists {
		return []string{}
	}

	values := make([]string, 0, len(valueMap))
	for value := range valueMap {
		values = append(values, value)
	}
	return values
}

func (s *DataStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":         len(s.data),
		"pending":       0,
		"archived":      0,
		"failed":        0,
		"total_size":    int64(0),
		"targets_count": len(s.targetIndex),
		"telescopes_count": len(s.telescopeIndex),
		"filters_count": len(s.filterIndex),
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
