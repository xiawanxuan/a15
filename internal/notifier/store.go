package notifier

import (
	"errors"
	"sync"
	"time"

	"astro-scheduler/pkg/models"
)

type AlertStore struct {
	mu     sync.RWMutex
	alerts map[string]*models.Alert
}

func NewAlertStore() *AlertStore {
	return &AlertStore{
		alerts: make(map[string]*models.Alert),
	}
}

func (s *AlertStore) AddAlert(alert *models.Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.alerts[alert.ID]; exists {
		return errors.New("alert already exists")
	}

	s.alerts[alert.ID] = alert
	return nil
}

func (s *AlertStore) GetAlert(alertID string) (*models.Alert, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alert, exists := s.alerts[alertID]
	return alert, exists
}

func (s *AlertStore) ResolveAlert(alertID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, exists := s.alerts[alertID]
	if !exists {
		return errors.New("alert not found")
	}

	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now

	return nil
}

func (s *AlertStore) ListAlerts(severity models.AlertSeverity, resolved bool, limit, offset int) []*models.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*models.Alert, 0)
	count := 0

	for _, alert := range s.alerts {
		if severity != "" && alert.Severity != severity {
			continue
		}

		if !resolved && alert.Resolved {
			continue
		}

		if count < offset {
			count++
			continue
		}

		result = append(result, alert)
		count++

		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

func (s *AlertStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total":     len(s.alerts),
		"active":    0,
		"resolved":  0,
		"info":      0,
		"warning":   0,
		"error":     0,
		"critical":  0,
	}

	for _, alert := range s.alerts {
		if alert.Resolved {
			stats["resolved"] = stats["resolved"].(int) + 1
		} else {
			stats["active"] = stats["active"].(int) + 1
		}

		switch alert.Severity {
		case models.AlertSeverityInfo:
			stats["info"] = stats["info"].(int) + 1
		case models.AlertSeverityWarning:
			stats["warning"] = stats["warning"].(int) + 1
		case models.AlertSeverityError:
			stats["error"] = stats["error"].(int) + 1
		case models.AlertSeverityCritical:
			stats["critical"] = stats["critical"].(int) + 1
		}
	}

	return stats
}
