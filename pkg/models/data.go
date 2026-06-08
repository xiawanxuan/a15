package models

import (
	"time"

	"github.com/google/uuid"
)

type DataFormat string

const (
	DataFormatFITS DataFormat = "fits"
	DataFormatJPEG DataFormat = "jpeg"
	DataFormatPNG  DataFormat = "png"
	DataFormatCSV  DataFormat = "csv"
	DataFormatJSON DataFormat = "json"
)

type ArchiveStatus string

const (
	ArchiveStatusPending  ArchiveStatus = "pending"
	ArchiveStatusArchived ArchiveStatus = "archived"
	ArchiveStatusFailed   ArchiveStatus = "failed"
)

type ObservationData struct {
	ID            string        `json:"id" yaml:"id"`
	TaskID        string        `json:"task_id" yaml:"task_id"`
	NodeID        string        `json:"node_id" yaml:"node_id"`
	Target        string        `json:"target" yaml:"target"`
	Format        DataFormat    `json:"format" yaml:"format"`
	Size          int64         `json:"size" yaml:"size"`
	FilePath      string        `json:"file_path,omitempty" yaml:"file_path,omitempty"`
	Checksum      string        `json:"checksum" yaml:"checksum"`
	ArchiveStatus ArchiveStatus `json:"archive_status" yaml:"archive_status"`
	Metadata      string        `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	ObservationTime time.Time   `json:"observation_time" yaml:"observation_time"`
	CreatedAt     time.Time     `json:"created_at" yaml:"created_at"`
	ArchivedAt    *time.Time    `json:"archived_at,omitempty" yaml:"archived_at,omitempty"`
}

func NewObservationData(taskID, nodeID, target string, format DataFormat, size int64, checksum, metadata string) *ObservationData {
	now := time.Now()
	return &ObservationData{
		ID:            uuid.New().String(),
		TaskID:        taskID,
		NodeID:        nodeID,
		Target:        target,
		Format:        format,
		Size:          size,
		Checksum:      checksum,
		ArchiveStatus: ArchiveStatusPending,
		Metadata:      metadata,
		ObservationTime: now,
		CreatedAt:     now,
	}
}

type ArchivePolicy struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	RetentionDays int    `json:"retention_days" yaml:"retention_days"`
	Compress      bool   `json:"compress" yaml:"compress"`
	BackupCount   int    `json:"backup_count" yaml:"backup_count"`
	StoragePath   string `json:"storage_path" yaml:"storage_path"`
}
