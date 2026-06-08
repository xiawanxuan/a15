package models

import (
	"time"

	"github.com/google/uuid"
)

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertType string

const (
	AlertTypeTaskFailed      AlertType = "task_failed"
	AlertTypeTaskRetry       AlertType = "task_retry"
	AlertTypeNodeOffline     AlertType = "node_offline"
	AlertTypeNodeOverloaded  AlertType = "node_overloaded"
	AlertTypeArchiveFailed   AlertType = "archive_failed"
	AlertTypeSystemError     AlertType = "system_error"
)

type Alert struct {
	ID        string        `json:"id" yaml:"id"`
	Type      AlertType     `json:"type" yaml:"type"`
	Severity  AlertSeverity `json:"severity" yaml:"severity"`
	Title     string        `json:"title" yaml:"title"`
	Message   string        `json:"message" yaml:"message"`
	TaskID    string        `json:"task_id,omitempty" yaml:"task_id,omitempty"`
	NodeID    string        `json:"node_id,omitempty" yaml:"node_id,omitempty"`
	CreatedAt time.Time     `json:"created_at" yaml:"created_at"`
	Resolved  bool          `json:"resolved" yaml:"resolved"`
	ResolvedAt *time.Time   `json:"resolved_at,omitempty" yaml:"resolved_at,omitempty"`
}

func NewAlert(alertType AlertType, severity AlertSeverity, title, message, taskID, nodeID string) *Alert {
	return &Alert{
		ID:        uuid.New().String(),
		Type:      alertType,
		Severity:  severity,
		Title:     title,
		Message:   message,
		TaskID:    taskID,
		NodeID:    nodeID,
		CreatedAt: time.Now(),
		Resolved:  false,
	}
}

type NotificationChannel string

const (
	NotificationChannelEmail    NotificationChannel = "email"
	NotificationChannelWebhook  NotificationChannel = "webhook"
	NotificationChannelSlack    NotificationChannel = "slack"
	NotificationChannelDingTalk NotificationChannel = "dingtalk"
)

type NotificationConfig struct {
	Channels  []NotificationChannel `json:"channels" yaml:"channels"`
	Enabled   bool                  `json:"enabled" yaml:"enabled"`
	Email     EmailConfig           `json:"email,omitempty" yaml:"email,omitempty"`
	Webhook   WebhookConfig         `json:"webhook,omitempty" yaml:"webhook,omitempty"`
}

type EmailConfig struct {
	SMTPHost  string   `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort  int      `json:"smtp_port" yaml:"smtp_port"`
	Username  string   `json:"username" yaml:"username"`
	Password  string   `json:"password" yaml:"password"`
	From      string   `json:"from" yaml:"from"`
	Receivers []string `json:"receivers" yaml:"receivers"`
}

type WebhookConfig struct {
	URL     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Method  string            `json:"method" yaml:"method"`
}
