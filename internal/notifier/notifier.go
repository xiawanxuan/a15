package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/utils"
)

type Notifier struct {
	store   *AlertStore
	config  models.NotificationConfig
	ctx     context.Context
	cancel  context.CancelFunc
	alertCh chan *models.Alert
}

func NewNotifier(store *AlertStore, config models.NotificationConfig) *Notifier {
	ctx, cancel := context.WithCancel(context.Background())
	return &Notifier{
		store:   store,
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
		alertCh: make(chan *models.Alert, 100),
	}
}

func (n *Notifier) Start() {
	utils.Sugar.Info("Notifier started")
	go n.processAlerts()
}

func (n *Notifier) Stop() {
	n.cancel()
	utils.Sugar.Info("Notifier stopped")
}

func (n *Notifier) CreateAlert(alertType models.AlertType, severity models.AlertSeverity, title, message, taskID, nodeID string) (*models.Alert, error) {
	alert := models.NewAlert(alertType, severity, title, message, taskID, nodeID)

	if err := n.store.AddAlert(alert); err != nil {
		return nil, err
	}

	if n.config.Enabled {
		n.alertCh <- alert
	}

	utils.Sugar.Infof("Alert created: %s - %s", alert.ID, alert.Title)
	return alert, nil
}

func (n *Notifier) processAlerts() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case alert := <-n.alertCh:
			n.sendNotifications(alert)
		}
	}
}

func (n *Notifier) sendNotifications(alert *models.Alert) {
	for _, channel := range n.config.Channels {
		switch channel {
		case models.NotificationChannelEmail:
			go n.sendEmail(alert)
		case models.NotificationChannelWebhook:
			go n.sendWebhook(alert)
		case models.NotificationChannelSlack:
			go n.sendSlack(alert)
		case models.NotificationChannelDingTalk:
			go n.sendDingTalk(alert)
		}
	}
}

func (n *Notifier) sendEmail(alert *models.Alert) {
	if n.config.Email.SMTPHost == "" {
		return
	}

	utils.Sugar.Debugf("Sending email notification for alert %s", alert.ID)
}

func (n *Notifier) sendWebhook(alert *models.Alert) {
	if n.config.Webhook.URL == "" {
		return
	}

	method := n.config.Webhook.Method
	if method == "" {
		method = "POST"
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		utils.Sugar.Errorf("Failed to marshal alert for webhook: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(n.ctx, method, n.config.Webhook.URL, bytes.NewBuffer(payload))
	if err != nil {
		utils.Sugar.Errorf("Failed to create webhook request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range n.config.Webhook.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.Sugar.Errorf("Failed to send webhook notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		utils.Sugar.Warnf("Webhook returned status code: %d", resp.StatusCode)
	}

	utils.Sugar.Debugf("Webhook notification sent for alert %s", alert.ID)
}

func (n *Notifier) sendSlack(alert *models.Alert) {
	utils.Sugar.Debugf("Sending Slack notification for alert %s", alert.ID)
}

func (n *Notifier) sendDingTalk(alert *models.Alert) {
	utils.Sugar.Debugf("Sending DingTalk notification for alert %s", alert.ID)
}

func (n *Notifier) GetAlert(alertID string) (*models.Alert, bool) {
	return n.store.GetAlert(alertID)
}

func (n *Notifier) ListAlerts(severity models.AlertSeverity, resolved bool, limit, offset int) []*models.Alert {
	return n.store.ListAlerts(severity, resolved, limit, offset)
}

func (n *Notifier) ResolveAlert(alertID string) error {
	return n.store.ResolveAlert(alertID)
}

func (n *Notifier) GetStats() map[string]interface{} {
	return n.store.GetStats()
}

func (n *Notifier) UpdateConfig(config models.NotificationConfig) {
	n.config = config
}
