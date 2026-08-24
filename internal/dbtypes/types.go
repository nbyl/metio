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

type BackupStatus string

const (
	BackupStatusCompleted BackupStatus = "COMPLETED"
	BackupStatusFailed    BackupStatus = "FAILED"
)

func (b BackupStatus) String() string {
	return string(b)
}

type Backup struct {
	ID               string       `json:"id"`
	ServerID         string       `json:"server_id"`
	ServerName       string       `json:"server_name"`
	SnapshotID       string       `json:"snapshot_id"`
	RepositoryPrefix string       `json:"repository_prefix"`
	CreatedAt        time.Time    `json:"created_at"`
	DurationSeconds  int64        `json:"duration_seconds"`
	FileCount        int64        `json:"file_count"`
	RepositorySize   int64        `json:"repository_size"`
	MinecraftVersion string       `json:"minecraft_version"`
	Status           BackupStatus `json:"status"`
	ServerDeletedAt  *time.Time   `json:"server_deleted_at,omitempty"`
	RetentionUntil   *time.Time   `json:"retention_until,omitempty"`
}
