package db

import "time"

type ShutdownSchedule struct {
	Enabled  bool   `firestore:"enabled"`
	Time     string `firestore:"time"`
	Timezone string `firestore:"timezone"`
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
	ID                          string            `firestore:"-"`
	Name                        string            `firestore:"name"`
	Region                      string            `firestore:"region"`
	Zone                        string            `firestore:"zone"`
	MachineType                 string            `firestore:"machineType"`
	MinecraftVersion            string            `firestore:"minecraftVersion"`
	DiskSizeGB                  int               `firestore:"diskSizeGB"`
	InfraVersion                int               `firestore:"infraVersion,omitempty"`
	DeployedByControllerVersion string            `firestore:"deployedByControllerVersion,omitempty"`
	ShutdownSchedule            *ShutdownSchedule `firestore:"shutdownSchedule,omitempty"`
	CreatedAt                   time.Time         `firestore:"createdAt"`
	UpdatedAt                   time.Time         `firestore:"updatedAt"`
}
