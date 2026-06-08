package archiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type AlertManager interface {
	CreateAlert(alertType models.AlertType, severity models.AlertSeverity, title, message, taskID, nodeID string) (*models.Alert, error)
}

type Archiver struct {
	store        *DataStore
	alertManager AlertManager
	basePath     string
	ctx          context.Context
	cancel       context.CancelFunc
	archiveCh    chan string
}

func NewArchiver(store *DataStore, alertManager AlertManager, basePath string) *Archiver {
	ctx, cancel := context.WithCancel(context.Background())
	return &Archiver{
		store:        store,
		alertManager: alertManager,
		basePath:     basePath,
		ctx:          ctx,
		cancel:       cancel,
		archiveCh:    make(chan string, 100),
	}
}

func (a *Archiver) Start() {
	utils.Sugar.Info("Archiver started")

	if err := os.MkdirAll(a.basePath, 0755); err != nil {
		utils.Sugar.Errorf("Failed to create archive directory: %v", err)
	}

	go a.archiveLoop()
	go a.cleanupLoop()
}

func (a *Archiver) Stop() {
	a.cancel()
	utils.Sugar.Info("Archiver stopped")
}

func (a *Archiver) AddData(data *models.ObservationData) error {
	if err := a.store.AddData(data); err != nil {
		return err
	}

	a.archiveCh <- data.ID
	utils.Sugar.Infof("Data %s added for archiving", data.ID)
	return nil
}

func (a *Archiver) archiveLoop() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case dataID := <-a.archiveCh:
			if err := a.processArchive(dataID); err != nil {
				utils.Sugar.Errorf("Archive failed for data %s: %v", dataID, err)
			}
		}
	}
}

func (a *Archiver) processArchive(dataID string) error {
	data, exists := a.store.GetData(dataID)
	if !exists {
		return fmt.Errorf("data not found: %s", dataID)
	}

	if data.ArchiveStatus != models.ArchiveStatusPending {
		return nil
	}

	filePath := a.generateArchivePath(data)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		a.handleArchiveFailure(data, err)
		return err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := a.createArchiveFile(data, filePath); err != nil {
			a.handleArchiveFailure(data, err)
			return err
		}
	}

	if err := a.store.MarkArchived(dataID, filePath); err != nil {
		return err
	}

	utils.Sugar.Infof("Data %s archived successfully to %s", dataID, filePath)
	return nil
}

func (a *Archiver) createArchiveFile(data *models.ObservationData, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header := fmt.Sprintf("ASTRO_OBSERVATION_DATA\nID: %s\nTaskID: %s\nTarget: %s\nFormat: %s\nSize: %d\nChecksum: %s\nObservationTime: %s\nMetadata: %s\n",
		data.ID, data.TaskID, data.Target, data.Format, data.Size, data.Checksum, data.ObservationTime.Format(time.RFC3339), data.Metadata)

	_, err = file.WriteString(header)
	return err
}

func (a *Archiver) handleArchiveFailure(data *models.ObservationData, err error) {
	_ = a.store.MarkFailed(data.ID, err.Error())

	if a.alertManager != nil {
		_, _ = a.alertManager.CreateAlert(
			models.AlertTypeArchiveFailed,
			models.AlertSeverityError,
			"Archive Failed",
			fmt.Sprintf("Failed to archive data %s: %v", data.ID, err),
			data.TaskID,
			data.NodeID,
		)
	}
}

func (a *Archiver) generateArchivePath(data *models.ObservationData) string {
	dateDir := data.ObservationTime.Format("2006/01/02")
	fileName := fmt.Sprintf("%s_%s.%s", data.Target, data.ID, data.Format)
	return filepath.Join(a.basePath, dateDir, fileName)
}

func (a *Archiver) cleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.cleanupOldData()
		}
	}
}

func (a *Archiver) cleanupOldData() {
	utils.Sugar.Info("Running data cleanup...")

	policies := a.store.ListPolicies()
	for _, policy := range policies {
		if policy.RetentionDays <= 0 {
			continue
		}

		cutoffDate := time.Now().AddDate(0, 0, -policy.RetentionDays)
		utils.Sugar.Infof("Cleaning data older than %v for policy %s", cutoffDate, policy.Name)
	}
}

func (a *Archiver) GetData(dataID string) (*models.ObservationData, bool) {
	return a.store.GetData(dataID)
}

func (a *Archiver) ListData(target string, format models.DataFormat, status models.ArchiveStatus, limit, offset int) []*models.ObservationData {
	return a.store.ListData(target, format, status, limit, offset)
}

func (a *Archiver) GetDataByTask(taskID string) []*models.ObservationData {
	return a.store.GetDataByTask(taskID)
}

func (a *Archiver) GetStats() map[string]interface{} {
	return a.store.GetStats()
}

func (a *Archiver) AddPolicy(policy *models.ArchivePolicy) error {
	return a.store.AddPolicy(policy)
}

func (a *Archiver) ListPolicies() []*models.ArchivePolicy {
	return a.store.ListPolicies()
}

func (a *Archiver) DeletePolicy(policyID string) error {
	return a.store.DeletePolicy(policyID)
}
