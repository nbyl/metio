package db

import (
	"fmt"
	"time"
)

type ShutdownSchedule struct {
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time"`
	Timezone string `json:"timezone"`
}

func (s *ShutdownSchedule) IsValid() bool {
	if s == nil {
		return true
	}
	if !s.Enabled {
		return true
	}
	return s.Time != "" && s.Timezone != ""
}

// BackupKeepUnits are the supported Restic retention units for a per-server
// backup override.
var BackupKeepUnits = []string{"hourly", "daily", "weekly", "monthly", "yearly"}

// BackupConfig holds a per-server override for the backup schedule and Restic
// retention policy. Zero/empty values fall back to the deployment defaults
// (backup enabled, hourly interval, keep-within BackupRetentionDays), so
// existing servers are unaffected until a backup config is set for them.
type BackupConfig struct {
	Enabled             bool   `json:"enabled"`
	BackupIntervalHours int    `json:"backupIntervalHours,omitempty"`
	Keep                int    `json:"keep,omitempty"`
	KeepUnit            string `json:"keepUnit,omitempty"`
}

// IsValid validates a backup config. A nil config (deployment defaults) is
// always valid.
func (b *BackupConfig) IsValid() error {
	if b == nil {
		return nil
	}
	if b.BackupIntervalHours < 0 {
		return fmt.Errorf("backup interval must not be negative, got %d", b.BackupIntervalHours)
	}
	if b.Keep < 0 {
		return fmt.Errorf("keep must not be negative, got %d", b.Keep)
	}
	if b.Keep > 0 && b.KeepUnit == "" {
		return fmt.Errorf("keep unit is required when a keep policy is set")
	}
	if b.KeepUnit != "" {
		valid := false
		for _, unit := range BackupKeepUnits {
			if unit == b.KeepUnit {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("keep unit %q must be one of %v", b.KeepUnit, BackupKeepUnits)
		}
	}
	return nil
}

type ServerConfig struct {
	ID                          string            `json:"-"`
	Name                        string            `json:"name"`
	Region                      string            `json:"region"`
	Zone                        string            `json:"zone"`
	MachineType                 string            `json:"machineType"`
	MinecraftVersion            string            `json:"minecraftVersion"`
	DiskSizeGB                  int               `json:"diskSizeGB"`
	InfraVersion                int               `json:"infraVersion,omitempty"`
	DeployedByControllerVersion string            `json:"deployedByControllerVersion,omitempty"`
	MachineAgentImage           string            `json:"machineAgentImage,omitempty"`
	ExistingAddress             string            `json:"existingAddress,omitempty"`
	ShutdownSchedule            *ShutdownSchedule `json:"shutdownSchedule,omitempty"`
	Backup                      *BackupConfig     `json:"backup,omitempty"`
	CreatedAt                   time.Time         `json:"createdAt"`
	UpdatedAt                   time.Time         `json:"updatedAt"`
}
