package db

import "time"

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
	CreatedAt                   time.Time         `json:"createdAt"`
	UpdatedAt                   time.Time         `json:"updatedAt"`
}
