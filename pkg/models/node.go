package models

import (
	"time"
)

type NodeStatus string

const (
	NodeStatusOnline   NodeStatus = "online"
	NodeStatusOffline  NodeStatus = "offline"
	NodeStatusBusy     NodeStatus = "busy"
	NodeStatusIdle     NodeStatus = "idle"
	NodeStatusDisabled NodeStatus = "disabled"
)

type NodeCapability struct {
	TelescopeType  string `json:"telescope_type" yaml:"telescope_type"`
	MaxMagnitude   float64 `json:"max_magnitude" yaml:"max_magnitude"`
	SupportsSpectroscopy bool `json:"supports_spectroscopy" yaml:"supports_spectroscopy"`
	DataFormats    []string `json:"data_formats" yaml:"data_formats"`
}

type NodeStats struct {
	TotalTasks     int       `json:"total_tasks"`
	CompletedTasks int       `json:"completed_tasks"`
	FailedTasks    int       `json:"failed_tasks"`
	RunningTasks   int       `json:"running_tasks"`
	CPUUsage       float64   `json:"cpu_usage"`
	MemoryUsage    float64   `json:"memory_usage"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
}

type Node struct {
	ID            string         `json:"id" yaml:"id"`
	Name          string         `json:"name" yaml:"name"`
	Address       string         `json:"address" yaml:"address"`
	Status        NodeStatus     `json:"status" yaml:"status"`
	Capabilities  NodeCapability `json:"capabilities" yaml:"capabilities"`
	Stats         NodeStats      `json:"stats" yaml:"stats"`
	Weight        int            `json:"weight" yaml:"weight"`
	RegisteredAt  time.Time      `json:"registered_at" yaml:"registered_at"`
	LastSeen      time.Time      `json:"last_seen" yaml:"last_seen"`
	Tags          []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type NodeHeartbeat struct {
	NodeID     string    `json:"node_id"`
	Status     NodeStatus `json:"status"`
	CPUUsage   float64   `json:"cpu_usage"`
	MemoryUsage float64  `json:"memory_usage"`
	RunningTasks int     `json:"running_tasks"`
	Timestamp  time.Time `json:"timestamp"`
}
