package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"astro-scheduler/pkg/models"
)

const (
	schedulerURL = "http://localhost:8080/api/v1"
	nodeID       = "node-demo-001"
	nodeName     = "Demo Telescope Node"
	nodeAddress  = "192.168.1.100:9000"
)

func main() {
	fmt.Println("Starting demo node client...")

	if err := registerNode(); err != nil {
		fmt.Printf("Failed to register node: %v\n", err)
		return
	}

	fmt.Println("Node registered successfully")

	go sendHeartbeats()

	processTasks()
}

func registerNode() error {
	node := map[string]interface{}{
		"id":       nodeID,
		"name":     nodeName,
		"address":  nodeAddress,
		"weight":   100,
		"capabilities": map[string]interface{}{
			"telescope_type":       "reflector",
			"max_magnitude":        18.5,
			"supports_spectroscopy": true,
			"data_formats":          []string{"fits", "jpeg", "png"},
		},
		"tags": []string{"optical", "northern-hemisphere"},
	}

	body, _ := json.Marshal(node)
	resp, err := http.Post(schedulerURL+"/nodes", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func sendHeartbeats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		heartbeat := models.NodeHeartbeat{
			NodeID:       nodeID,
			Status:       models.NodeStatusIdle,
			CPUUsage:     25.5,
			MemoryUsage:  40.2,
			RunningTasks: 0,
			Timestamp:    time.Now(),
		}

		body, _ := json.Marshal(heartbeat)
		resp, err := http.Post(schedulerURL+"/nodes/heartbeat", "application/json", bytes.NewBuffer(body))
		if err != nil {
			fmt.Printf("Heartbeat error: %v\n", err)
			continue
		}
		resp.Body.Close()

		fmt.Printf("Heartbeat sent at %v\n", time.Now().Format("15:04:05"))
	}
}

func processTasks() {
	fmt.Println("Waiting for tasks...")
	select {}
}
