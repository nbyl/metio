// Package config provides centralized configuration loading for the Metio application.
// It handles environment variables for database connections, instance identification,
// and deployment environment settings.
package config

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/compute/metadata"
	"github.com/nbyl/metio/internal/db"
	"github.com/spf13/viper"
)

// Config holds the configuration needed for database connections and service identification.
type Config struct {
	Environment              string
	Region                   string
	ProjectID                string
	MachineAgentImage        string
	OperationMode            string
	BaseURL                  string
	CloudTasksQueue          string
	CloudTasksRegion         string
	ControllerServiceAccount string
	ControllerVersion        string
	DBBackend                string
	DaprStateStoreName       string
}

// Default values for configuration
const (
	DefaultEnvironment = "development"
	DefaultRegion      = "us-central1"
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
		OperationMode:            viper.GetString("OPERATION_MODE"),
		BaseURL:                  viper.GetString("BASE_URL"),
		CloudTasksQueue:          viper.GetString("CLOUD_TASKS_QUEUE"),
		CloudTasksRegion:         viper.GetString("CLOUD_TASKS_REGION"),
		ControllerServiceAccount: viper.GetString("CONTROLLER_SERVICE_ACCOUNT"),
	}

	dbBackend := viper.GetString("DB_BACKEND")
	if dbBackend == "" {
		dbBackend = "firestore"
	} else if dbBackend != "firestore" && dbBackend != "dapr" {
		return cfg, fmt.Errorf("DB_BACKEND must be 'firestore' or 'dapr', got %q", dbBackend)
	}
	cfg.DBBackend = dbBackend

	daprStateStoreName := viper.GetString("DAPR_STATE_STORE_NAME")
	if daprStateStoreName == "" {
		daprStateStoreName = "statestore"
	}
	cfg.DaprStateStoreName = daprStateStoreName

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

// DatabaseID returns the formatted Firestore database ID.
func (c Config) DatabaseID() string {
	return fmt.Sprintf("%s-%s-metio-db", c.Environment, c.Region)
}

// NewDBConnection creates a database connection using this config.
func (c Config) NewDBConnection(ctx context.Context) (db.DB, error) {
	switch c.DBBackend {
	case "dapr":
		return db.NewDaprDB(ctx, c.DaprStateStoreName)
	default:
		return db.NewConnection(ctx, c.ProjectID, c.DatabaseID())
	}
}
