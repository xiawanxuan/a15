package config

import (
	"fmt"

	"astro-scheduler/pkg/models"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Log         LogConfig         `mapstructure:"log"`
	Scheduler   SchedulerConfig   `mapstructure:"scheduler"`
	Node        NodeConfig        `mapstructure:"node"`
	Archiver    ArchiverConfig    `mapstructure:"archiver"`
	Notification NotificationConfig `mapstructure:"notification"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

type SchedulerConfig struct {
	MaxRetries    int `mapstructure:"max_retries"`
	RetryInterval int `mapstructure:"retry_interval"`
}

type NodeConfig struct {
	HeartbeatTimeout int `mapstructure:"heartbeat_timeout"`
	CheckInterval    int `mapstructure:"check_interval"`
}

type ArchiverConfig struct {
	BasePath            string `mapstructure:"base_path"`
	DefaultRetentionDays int   `mapstructure:"default_retention_days"`
}

type NotificationConfig struct {
	Enabled  bool                       `mapstructure:"enabled"`
	Channels []models.NotificationChannel `mapstructure:"channels"`
	Email    models.EmailConfig         `mapstructure:"email"`
	Webhook  models.WebhookConfig       `mapstructure:"webhook"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (c *NotificationConfig) ToModel() models.NotificationConfig {
	return models.NotificationConfig{
		Channels: c.Channels,
		Enabled:  c.Enabled,
		Email:    c.Email,
		Webhook:  c.Webhook,
	}
}
