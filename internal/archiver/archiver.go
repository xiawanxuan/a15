package archiver

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"astro-scheduler/pkg/lock"
	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/storage"
	"astro-scheduler/pkg/utils"
)

type AlertManager interface {
	CreateAlert(alertType models.AlertType, severity models.AlertSeverity, title, message, taskID, nodeID string) (*models.Alert, error)
}

type Archiver struct {
	store        *DataStore
	alertManager AlertManager
	objectStorage storage.ObjectStorage
	distLock     lock.DistributedLock
	bucket       string
	basePath     string
	ctx          context.Context
	cancel       context.CancelFunc
	archiveCh    chan string
	lockTTL      time.Duration
}

func NewArchiver(
	store *DataStore,
	alertManager AlertManager,
	objectStorage storage.ObjectStorage,
	distLock lock.DistributedLock,
	bucket string,
	basePath string,
) *Archiver {
	ctx, cancel := context.WithCancel(context.Background())
	return &Archiver{
		store:         store,
		alertManager:  alertManager,
		objectStorage: objectStorage,
		distLock:      distLock,
		bucket:        bucket,
		basePath:      basePath,
		ctx:           ctx,
		cancel:        cancel,
		archiveCh:     make(chan string, 100),
		lockTTL:       5 * time.Minute,
	}
}

func (a *Archiver) Start() {
	utils.Sugar.Info("Archiver started")

	if a.objectStorage == nil {
		utils.Sugar.Warn("Object storage not configured, archiver will run in limited mode")
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
	lockKey := fmt.Sprintf("archive:data:%s", dataID)
	locked, err := a.distLock.TryLock(a.ctx, lockKey, a.lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire archive lock: %w", err)
	}
	if !locked {
		utils.Sugar.Debugf("Data %s already being archived by another instance", dataID)
		return nil
	}
	defer func() {
		_ = a.distLock.Unlock(a.ctx, lockKey)
	}()

	data, exists := a.store.GetData(dataID)
	if !exists {
		return fmt.Errorf("data not found: %s", dataID)
	}

	if data.ArchiveStatus == models.ArchiveStatusArchived {
		utils.Sugar.Debugf("Data %s already archived", dataID)
		return nil
	}

	if a.objectStorage == nil {
		return fmt.Errorf("object storage not configured")
	}

	objectKey := a.generateObjectKey(data)

	dataContent := a.generateDataContent(data)

	contentType := a.getContentType(data.Format)
	err = a.objectStorage.PutObject(
		a.ctx,
		a.bucket,
		objectKey,
		bytes.NewReader(dataContent),
		int64(len(dataContent)),
		contentType,
	)
	if err != nil {
		a.handleArchiveFailure(data, err)
		return err
	}

	if err := a.store.MarkArchived(dataID, objectKey); err != nil {
		return err
	}

	utils.Sugar.Infof("Data %s archived successfully to s3://%s/%s", dataID, a.bucket, objectKey)
	return nil
}

func (a *Archiver) generateObjectKey(data *models.ObservationData) string {
	dateDir := data.ObservationTime.Format("2006/01/02")
	fileName := fmt.Sprintf("%s_%s.%s", data.Target, data.ID, data.Format)

	if a.basePath != "" {
		return fmt.Sprintf("%s/%s/%s", a.basePath, dateDir, fileName)
	}
	return fmt.Sprintf("%s/%s", dateDir, fileName)
}

func (a *Archiver) generateDataContent(data *models.ObservationData) []byte {
	header := fmt.Sprintf(
		"ASTRO_OBSERVATION_DATA\nID: %s\nTaskID: %s\nNodeID: %s\nTarget: %s\nFormat: %s\nSize: %d\nChecksum: %s\nObservationTime: %s\nMetadata: %s\n",
		data.ID,
		data.TaskID,
		data.NodeID,
		data.Target,
		data.Format,
		data.Size,
		data.Checksum,
		data.ObservationTime.Format(time.RFC3339),
		data.Metadata,
	)
	return []byte(header)
}

func (a *Archiver) getContentType(format models.DataFormat) string {
	switch format {
	case models.DataFormatFITS:
		return "application/fits"
	case models.DataFormatJPEG:
		return "image/jpeg"
	case models.DataFormatPNG:
		return "image/png"
	case models.DataFormatCSV:
		return "text/csv"
	case models.DataFormatJSON:
		return "application/json"
	default:
		return "application/octet-stream"
	}
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

func (a *Archiver) DownloadData(dataID string) ([]byte, *models.ObservationData, error) {
	data, exists := a.store.GetData(dataID)
	if !exists {
		return nil, nil, fmt.Errorf("data not found: %s", dataID)
	}

	if data.ArchiveStatus != models.ArchiveStatusArchived || a.objectStorage == nil {
		return nil, data, fmt.Errorf("data not available for download")
	}

	reader, _, err := a.objectStorage.GetObject(a.ctx, a.bucket, data.FilePath)
	if err != nil {
		return nil, data, err
	}
	defer reader.Close()

	content, err := bytes.NewBuffer(nil).ReadFrom(reader)
	if err != nil {
		return nil, data, err
	}

	result := make([]byte, content)
	_, _ = reader.Read(result)

	return result, data, nil
}

func (a *Archiver) GetPresignedURL(dataID string, expires time.Duration) (string, error) {
	data, exists := a.store.GetData(dataID)
	if !exists {
		return "", fmt.Errorf("data not found: %s", dataID)
	}

	if data.ArchiveStatus != models.ArchiveStatusArchived || a.objectStorage == nil {
		return "", fmt.Errorf("data not available for download")
	}

	return a.objectStorage.PresignedGetURL(a.ctx, a.bucket, data.FilePath, expires)
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
	cleanupLockKey := "archive:cleanup"
	locked, err := a.distLock.TryLock(a.ctx, cleanupLockKey, 1*time.Hour)
	if err != nil || !locked {
		utils.Sugar.Debug("Cleanup already running on another instance, skipping")
		return
	}
	defer func() {
		_ = a.distLock.Unlock(a.ctx, cleanupLockKey)
	}()

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
	stats := a.store.GetStats()
	stats["storage_type"] = "object"
	stats["bucket"] = a.bucket
	return stats
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

func (a *Archiver) SearchData(query SearchQuery) ([]*models.ObservationData, int) {
	return a.store.SearchData(query)
}

func (a *Archiver) GetDistinctTargets() []string {
	return a.store.GetDistinctTargets()
}

func (a *Archiver) GetDistinctTelescopes() []string {
	return a.store.GetDistinctTelescopes()
}

func (a *Archiver) GetDistinctFilters() []string {
	return a.store.GetDistinctFilters()
}

func (a *Archiver) GetMetadataKeys() []string {
	return a.store.GetMetadataKeys()
}

func (a *Archiver) GetMetadataValues(key string) []string {
	return a.store.GetMetadataValues(key)
}

func (a *Archiver) GetObjectStorage() storage.ObjectStorage {
	return a.objectStorage
}
