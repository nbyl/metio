// Package config provides centralized configuration loading for the Metio application.
// It handles environment variables for database connections, instance identification,
// and deployment environment settings.
package config

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/compute/metadata"
	"github.com/spf13/viper"
	"gitlab.com/nbyl/metio/internal/db"
)

// Config holds the configuration needed for database connections and service identification.
type Config struct {
	Environment       string
	Region            string
	ProjectID         string
	MachineAgentImage string
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
		Environment:       environment,
		Region:            region,
		ProjectID:         viper.GetString("GCP_PROJECT"),
		MachineAgentImage: viper.GetString("MACHINE_AGENT_IMAGE"),
	}

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
	return db.NewConnection(ctx, c.ProjectID, c.DatabaseID())
}
