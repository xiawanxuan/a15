package config

import (
	"fmt"

	"astro-scheduler/pkg/lock"
	"astro-scheduler/pkg/models"
	"astro-scheduler/pkg/storage"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig       `mapstructure:"server"`
	Log         LogConfig          `mapstructure:"log"`
	Scheduler   SchedulerConfig    `mapstructure:"scheduler"`
	Node        NodeConfig         `mapstructure:"node"`
	Archiver    ArchiverConfig     `mapstructure:"archiver"`
	Notification NotificationConfig `mapstructure:"notification"`
	Lock        LockConfig         `mapstructure:"lock"`
	Storage     StorageConfig      `mapstructure:"storage"`
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
	Bucket              string `mapstructure:"bucket"`
}

type LockConfig struct {
	Type     string `mapstructure:"type"`
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

type StorageConfig struct {
	Type      string `mapstructure:"type"`
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	BasePath  string `mapstructure:"base_path"`
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

func (c *LockConfig) ToLockConfig() lock.LockConfig {
	return lock.LockConfig{
		Type:     c.Type,
		Address:  c.Address,
		Password: c.Password,
		DB:       c.DB,
		Prefix:   c.Prefix,
	}
}

func (c *StorageConfig) ToStorageConfig() storage.StorageConfig {
	return storage.StorageConfig{
		Type:      c.Type,
		Endpoint:  c.Endpoint,
		AccessKey: c.AccessKey,
		SecretKey: c.SecretKey,
		Bucket:    c.Bucket,
		Region:    c.Region,
		UseSSL:    c.UseSSL,
		BasePath:  c.BasePath,
	}
}
