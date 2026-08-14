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

// BackupConfig holds a per-server override for the backup schedule and Restic
// retention policy. Zero/empty values fall back to the deployment defaults
// (backup enabled, hourly interval, keep-within BackupRetentionDays), so
// existing servers are unaffected until a backup config is set for them.
type BackupConfig struct {
	Enabled        bool   `json:"enabled"`
	BackupSchedule string `json:"backupSchedule,omitempty"`
	KeepLast       int    `json:"keepLast,omitempty"`
	KeepHourly     int    `json:"keepHourly,omitempty"`
	KeepDaily      int    `json:"keepDaily,omitempty"`
	KeepWeekly     int    `json:"keepWeekly,omitempty"`
	KeepMonthly    int    `json:"keepMonthly,omitempty"`
	KeepYearly     int    `json:"keepYearly,omitempty"`
}

// IsValid validates a backup config. A nil config (deployment defaults) is
// always valid.
func (b *BackupConfig) IsValid() error {
	if b == nil {
		return nil
	}
	if b.BackupSchedule != "" && !isValidBackupSchedule(b.BackupSchedule) {
		return fmt.Errorf("backup schedule %q must be a duration with a single unit like 30m, 1h, 6h, 1d, 1w (units s, m, h, d, w)", b.BackupSchedule)
	}
	for name, v := range map[string]int{
		"keepLast":    b.KeepLast,
		"keepHourly":  b.KeepHourly,
		"keepDaily":   b.KeepDaily,
		"keepWeekly":  b.KeepWeekly,
		"keepMonthly": b.KeepMonthly,
		"keepYearly":  b.KeepYearly,
	} {
		if v < 0 {
			return fmt.Errorf("%s must not be negative, got %d", name, v)
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
