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
	Current int `json:"current" firestore:"current"`
	Max     int `json:"max" firestore:"max"`
}

type Status struct {
	Players              Players     `json:"players" firestore:"players"`
	Timestamp            time.Time   `json:"timestamp" firestore:"timestamp"`
	Uptime               string      `json:"uptime" firestore:"uptime"`
	ServerState          ServerState `json:"server_state" firestore:"server_state"`
	InstanceIP           string      `json:"instance_ip" firestore:"instance_ip"`
	Version              string      `json:"version" firestore:"version"`
	WhitelistEnabled     bool        `json:"whitelist_enabled" firestore:"whitelist_enabled"`
	ScheduledShutdown    *time.Time  `json:"scheduled_shutdown,omitempty" firestore:"scheduled_shutdown,omitempty"`
	PendingCommand       string      `json:"pending_command,omitempty" firestore:"pendingCommand,omitempty"`
	PendingCommandResult string      `json:"pending_command_result,omitempty" firestore:"pendingCommandResult,omitempty"`
	AgentVersion         string      `json:"agent_version,omitempty" firestore:"agent_version,omitempty"`
}

type WhitelistEntry struct {
	Username string    `json:"username" firestore:"username"`
	UUID     string    `json:"uuid" firestore:"uuid"`
	AddedAt  time.Time `json:"added_at" firestore:"added_at"`
	AddedBy  string    `json:"added_by" firestore:"added_by"`
}

type WhitelistConfig struct {
	Enabled bool `json:"enabled" firestore:"enabled"`
}
