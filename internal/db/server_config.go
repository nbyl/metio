package db

import "time"

type ShutdownSchedule struct {
	Enabled  bool   `json:"enabled" firestore:"enabled"`
	Time     string `json:"time" firestore:"time"`
	Timezone string `json:"timezone" firestore:"timezone"`
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
	ID                          string            `json:"-" firestore:"-"`
	Name                        string            `json:"name" firestore:"name"`
	Region                      string            `json:"region" firestore:"region"`
	Zone                        string            `json:"zone" firestore:"zone"`
	MachineType                 string            `json:"machineType" firestore:"machineType"`
	MinecraftVersion            string            `json:"minecraftVersion" firestore:"minecraftVersion"`
	DiskSizeGB                  int               `json:"diskSizeGB" firestore:"diskSizeGB"`
	InfraVersion                int               `json:"infraVersion,omitempty" firestore:"infraVersion,omitempty"`
	DeployedByControllerVersion string            `json:"deployedByControllerVersion,omitempty" firestore:"deployedByControllerVersion,omitempty"`
	MachineAgentImage           string            `json:"machineAgentImage,omitempty" firestore:"machineAgentImage,omitempty"`
	ExistingAddress             string            `json:"existingAddress,omitempty" firestore:"existingAddress,omitempty"`
	ShutdownSchedule            *ShutdownSchedule `json:"shutdownSchedule,omitempty" firestore:"shutdownSchedule,omitempty"`
	CreatedAt                   time.Time         `json:"createdAt" firestore:"createdAt"`
	UpdatedAt                   time.Time         `json:"updatedAt" firestore:"updatedAt"`
}
