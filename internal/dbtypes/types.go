package dbtypes

import "time"

type ServerState string

const (
	ServerStateStopped  ServerState = "STOPPED"
	ServerStateStarting ServerState = "STARTING"
	ServerStateRunning  ServerState = "RUNNING"
	ServerStateStopping ServerState = "STOPPING"
)

func (s ServerState) String() string {
	return string(s)
}

func (s ServerState) IsRunning() bool {
	return s == ServerStateRunning
}

func (s ServerState) IsStopped() bool {
	return s == ServerStateStopped
}

func (s ServerState) IsTransitioning() bool {
	return s == ServerStateStarting || s == ServerStateStopping
}

type Players struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

type Status struct {
	Players              Players     `json:"players"`
	Timestamp            time.Time   `json:"timestamp"`
	Uptime               string      `json:"uptime"`
	ServerState          ServerState `json:"server_state"`
	InstanceIP           string      `json:"instance_ip"`
	Version              string      `json:"version"`
	WhitelistEnabled     bool        `json:"whitelist_enabled"`
	ScheduledShutdown    *time.Time  `json:"scheduled_shutdown,omitempty"`
	PendingCommand       string      `json:"pending_command,omitempty"`
	PendingCommandResult string      `json:"pending_command_result,omitempty"`
	AgentVersion         string      `json:"agent_version,omitempty"`
}

type WhitelistEntry struct {
	Username string    `json:"username"`
	UUID     string    `json:"uuid"`
	AddedAt  time.Time `json:"added_at"`
	AddedBy  string    `json:"added_by"`
}

type WhitelistConfig struct {
	Enabled bool `json:"enabled"`
}
