package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"astro-scheduler/pkg/models"
)

type SchedulerClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewSchedulerClient(baseURL string) *SchedulerClient {
	return &SchedulerClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SchedulerClient) CreateTask(name string, taskType models.TaskType, priority models.TaskPriority, target string, duration int, payload string) (*models.Task, error) {
	reqBody := map[string]interface{}{
		"name":     name,
		"type":     taskType,
		"priority": priority,
		"target":   target,
		"duration": duration,
		"payload":  payload,
	}

	var task models.Task
	if err := c.post("/tasks", reqBody, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

func (c *SchedulerClient) GetTask(taskID string) (*models.Task, error) {
	var task models.Task
	if err := c.get("/tasks/"+taskID, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

func (c *SchedulerClient) ListTasks(status models.TaskStatus, limit, offset int) ([]*models.Task, error) {
	path := fmt.Sprintf("/tasks?status=%s&limit=%d&offset=%d", status, limit, offset)
	var result struct {
		Tasks []*models.Task `json:"tasks"`
	}

	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return result.Tasks, nil
}

func (c *SchedulerClient) UpdateTaskStatus(taskID string, status models.TaskStatus, nodeID string, dataID string, message string) (*models.Task, error) {
	reqBody := map[string]interface{}{
		"status":   status,
		"node_id":  nodeID,
		"data_id":  dataID,
		"message":  message,
	}

	var task models.Task
	if err := c.put("/tasks/"+taskID+"/status", reqBody, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

func (c *SchedulerClient) RegisterNode(node *models.Node) error {
	return c.post("/nodes", node, nil)
}

func (c *SchedulerClient) DeregisterNode(nodeID string) error {
	return c.delete("/nodes/"+nodeID, nil)
}

func (c *SchedulerClient) SendHeartbeat(heartbeat *models.NodeHeartbeat) error {
	return c.post("/nodes/heartbeat", heartbeat, nil)
}

func (c *SchedulerClient) ListNodes(status models.NodeStatus) ([]*models.Node, error) {
	path := fmt.Sprintf("/nodes?status=%s", status)
	var result struct {
		Nodes []*models.Node `json:"nodes"`
	}

	if err := c.get(path, &result); err != nil {
		return nil, err
	}

	return result.Nodes, nil
}

func (c *SchedulerClient) AddData(data *models.ObservationData) (*models.ObservationData, error) {
	var result models.ObservationData
	if err := c.post("/data", data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *SchedulerClient) GetData(dataID string) (*models.ObservationData, error) {
	var data models.ObservationData
	if err := c.get("/data/"+dataID, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (c *SchedulerClient) GetDataByTask(taskID string) ([]*models.ObservationData, error) {
	var result struct {
		Data []*models.ObservationData `json:"data"`
	}

	if err := c.get("/data/task/"+taskID, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *SchedulerClient) GetDownloadURL(dataID string, expires int) (string, error) {
	path := fmt.Sprintf("/data/%s/presigned-url?expires=%d", dataID, expires)
	var result struct {
		URL       string `json:"url"`
		ExpiresIn int    `json:"expires_in"`
	}

	if err := c.get(path, &result); err != nil {
		return "", err
	}

	return result.URL, nil
}

func (c *SchedulerClient) DownloadDataInfo(dataID string) (string, int, error) {
	path := fmt.Sprintf("/data/%s/download", dataID)
	var result struct {
		DownloadURL string `json:"download_url"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := c.get(path, &result); err != nil {
		return "", 0, err
	}

	return result.DownloadURL, result.ExpiresIn, nil
}

func (c *SchedulerClient) CreateAlert(alert *models.Alert) (*models.Alert, error) {
	var result models.Alert
	if err := c.post("/alerts", alert, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *SchedulerClient) ResolveAlert(alertID string) error {
	return c.put("/alerts/"+alertID+"/resolve", nil, nil)
}

func (c *SchedulerClient) GetStats() (map[string]interface{}, error) {
	var stats map[string]interface{}
	if err := c.get("/stats", &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

func (c *SchedulerClient) HealthCheck() (bool, error) {
	var result struct {
		Status string `json:"status"`
	}

	if err := c.get("/health", &result); err != nil {
		return false, err
	}

	return result.Status == "ok", nil
}

type SearchDataRequest struct {
	Target     string            `json:"target"`
	TaskID     string            `json:"task_id"`
	NodeID     string            `json:"node_id"`
	Format     string            `json:"format"`
	Status     string            `json:"status"`
	Telescope  string            `json:"telescope"`
	Filter     string            `json:"filter"`
	Instrument string            `json:"instrument"`
	Metadata   map[string]string `json:"metadata"`
	StartTime  string            `json:"start_time"`
	EndTime    string            `json:"end_time"`
	MinSize    int64             `json:"min_size"`
	MaxSize    int64             `json:"max_size"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	SortBy     string            `json:"sort_by"`
	SortOrder  string            `json:"sort_order"`
}

type SearchDataResult struct {
	Data  []*models.ObservationData `json:"data"`
	Total int                       `json:"total"`
	Limit int                       `json:"limit"`
	Offset int                      `json:"offset"`
}

func (c *SchedulerClient) SearchData(req SearchDataRequest) (*SearchDataResult, error) {
	var result SearchDataResult
	if err := c.post("/data/search", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *SchedulerClient) GetDistinctTargets() ([]string, error) {
	var result struct {
		Targets []string `json:"targets"`
	}
	if err := c.get("/data/targets", &result); err != nil {
		return nil, err
	}
	return result.Targets, nil
}

func (c *SchedulerClient) GetDistinctTelescopes() ([]string, error) {
	var result struct {
		Telescopes []string `json:"telescopes"`
	}
	if err := c.get("/data/telescopes", &result); err != nil {
		return nil, err
	}
	return result.Telescopes, nil
}

func (c *SchedulerClient) GetDistinctFilters() ([]string, error) {
	var result struct {
		Filters []string `json:"filters"`
	}
	if err := c.get("/data/filters", &result); err != nil {
		return nil, err
	}
	return result.Filters, nil
}

func (c *SchedulerClient) GetMetadataKeys() ([]string, error) {
	var result struct {
		Keys []string `json:"keys"`
	}
	if err := c.get("/data/metadata/keys", &result); err != nil {
		return nil, err
	}
	return result.Keys, nil
}

func (c *SchedulerClient) GetMetadataValues(key string) ([]string, error) {
	var result struct {
		Values []string `json:"values"`
	}
	if err := c.get("/data/metadata/values/"+key, &result); err != nil {
		return nil, err
	}
	return result.Values, nil
}

type ExportLogsRequest struct {
	TaskIDs    []string `json:"task_ids"`
	TaskStatus string   `json:"task_status"`
	NodeID     string   `json:"node_id"`
	StartTime  string   `json:"start_time"`
	EndTime    string   `json:"end_time"`
	EventType  string   `json:"event_type"`
	Format     string   `json:"format"`
}

type LogSummary struct {
	TotalEvents   int   `json:"TotalEvents"`
	TotalTasks    int   `json:"TotalTasks"`
	SuccessTasks  int   `json:"SuccessTasks"`
	FailedTasks   int   `json:"FailedTasks"`
	RunningTasks  int   `json:"RunningTasks"`
	PendingTasks  int   `json:"PendingTasks"`
	AvgDurationMs int64 `json:"AvgDurationMs"`
	MaxDurationMs int64 `json:"MaxDurationMs"`
	MinDurationMs int64 `json:"MinDurationMs"`
}

func (c *SchedulerClient) GetLogSummary(req ExportLogsRequest) (*LogSummary, error) {
	var result LogSummary
	if err := c.post("/tasks/logs/summary", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *SchedulerClient) ExportLogs(req ExportLogsRequest) ([]byte, string, error) {
	url := c.BaseURL + "/tasks/logs/export"

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, "", err
	}

	contentType := resp.Header.Get("Content-Type")
	return buf.Bytes(), contentType, nil
}

func (c *SchedulerClient) get(path string, result interface{}) error {
	url := c.BaseURL + path
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func (c *SchedulerClient) post(path string, body interface{}, result interface{}) error {
	url := c.BaseURL + path

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	resp, err := c.HTTPClient.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func (c *SchedulerClient) put(path string, body interface{}, result interface{}) error {
	url := c.BaseURL + path

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}

func (c *SchedulerClient) delete(path string, result interface{}) error {
	url := c.BaseURL + path

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error: %d", resp.StatusCode)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}

	return nil
}
