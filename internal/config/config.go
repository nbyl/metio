// Package config provides centralized configuration loading for the Metio application.
// It handles environment variables for database connections, instance identification,
// and deployment environment settings.
package config

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"cloud.google.com/go/compute/metadata"
	"github.com/nbyl/metio/internal/db"
	"github.com/spf13/viper"
)

// Config holds the configuration needed for database connections and service identification.
type Config struct {
	Environment                      string
	Region                           string
	ProjectID                        string
	MachineAgentImage                string
	BackupImage                      string
	OperationMode                    string
	BaseURL                          string
	CloudTasksQueue                  string
	CloudTasksRegion                 string
	ControllerServiceAccount         string
	ControllerVersion                string
	DaprStateStoreName               string
	BackupResticPassword             string
	BackupDeletedServerRetentionDays int
}

// Default values for configuration
const (
	DefaultEnvironment                      = "development"
	DefaultRegion                           = "us-central1"
	DefaultBackupDeletedServerRetentionDays = 30
)

// Load reads configuration from environment variables via viper.
// Requires viper.AutomaticEnv() to be called first.
func Load() (Config, error) {
	environment := viper.GetString("ENVIRONMENT")
	if environment == "" {
		environment = DefaultEnvironment
	}

	region := viper.GetString("REGION")
	if region == "" {
		region = DefaultRegion
	}

	cfg := Config{
		Environment:              environment,
		Region:                   region,
		ProjectID:                viper.GetString("GCP_PROJECT"),
		MachineAgentImage:        viper.GetString("MACHINE_AGENT_IMAGE"),
		BackupImage:              viper.GetString("BACKUP_IMAGE"),
		OperationMode:            viper.GetString("OPERATION_MODE"),
		BaseURL:                  viper.GetString("BASE_URL"),
		CloudTasksQueue:          viper.GetString("CLOUD_TASKS_QUEUE"),
		CloudTasksRegion:         viper.GetString("CLOUD_TASKS_REGION"),
		ControllerServiceAccount: viper.GetString("CONTROLLER_SERVICE_ACCOUNT"),
		BackupResticPassword:     viper.GetString("BACKUP_RESTIC_PASSWORD"),
	}

	daprStateStoreName := viper.GetString("DAPR_STATE_STORE_NAME")
	if daprStateStoreName == "" {
		daprStateStoreName = "statestore"
	}
	cfg.DaprStateStoreName = daprStateStoreName

	backupDeletedServerRetentionDays := DefaultBackupDeletedServerRetentionDays
	if raw := viper.GetString("BACKUP_DELETED_SERVER_RETENTION_DAYS"); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days <= 0 {
			return Config{}, fmt.Errorf("BACKUP_DELETED_SERVER_RETENTION_DAYS must be a positive integer, got %q", raw)
		}
		backupDeletedServerRetentionDays = days
	}
	cfg.BackupDeletedServerRetentionDays = backupDeletedServerRetentionDays

	if cfg.MachineAgentImage == "" {
		log.Print("MACHINE_AGENT_IMAGE is not set")
		return cfg, fmt.Errorf("MACHINE_AGENT_IMAGE must be set")
	}

	return cfg, nil
}

// LoadWithMetadata reads configuration but fetches ProjectID from GCE metadata.
// Use this for services running on GCE instances (e.g., machine-agent).
func LoadWithMetadata() (Config, error) {
	cfg, _ := Load()

	projectID, err := metadata.ProjectID()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get project ID from metadata: %w", err)
	}
	cfg.ProjectID = projectID

	return cfg, nil
}

// NewDBConnection creates a database connection using this config.
func (c Config) NewDBConnection(ctx context.Context) (db.DB, error) {
	return db.NewDaprDB(ctx, c.DaprStateStoreName)
}
