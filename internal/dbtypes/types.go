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
	Players              Players           `json:"players"`
	Timestamp            time.Time         `json:"timestamp"`
	Uptime               string            `json:"uptime"`
	ServerState          ServerState       `json:"server_state"`
	InstanceIP           string            `json:"instance_ip"`
	Version              string            `json:"version"`
	WhitelistEnabled     bool              `json:"whitelist_enabled"`
	ScheduledShutdown    *time.Time        `json:"scheduled_shutdown,omitempty"`
	PendingCommand       string            `json:"pending_command,omitempty"`
	PendingCommandArgs   map[string]string `json:"pending_command_args,omitempty"`
	PendingCommandResult string            `json:"pending_command_result,omitempty"`
	AgentVersion         string            `json:"agent_version,omitempty"`
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

// ModrinthPlatform is the only valid ModpackConfig platform today. It is
// retained as a named constant because the field exists to make a future
// second pack source an additive change.
const ModrinthPlatform = "modrinth"

// ModpackConfig references a modpack a server boots with (ADR-0006). A server
// is either vanilla (Modpack nil) or pack-driven: when a pack is set, the
// Minecraft version is pack-controlled and stored empty. Platform is retained
// even though only "modrinth" is valid today so a second pack source later is
// an additive change rather than a schema migration.
type ModpackConfig struct {
	// Platform is the pack source, e.g. "modrinth".
	Platform string `json:"platform"`
	// ProjectID identifies the pack on the platform.
	ProjectID string `json:"projectId"`
	// VersionID pins a specific version of the pack; empty means latest.
	VersionID string `json:"versionId,omitempty"`
}

type BackupSourceConfig struct {
	Region           string         `json:"region"`
	Zone             string         `json:"zone"`
	MachineType      string         `json:"machine_type"`
	DiskSizeGB       int            `json:"disk_size_gb"`
	MinecraftVersion string         `json:"minecraft_version"`
	Modpack          *ModpackConfig `json:"modpack,omitempty"`
}

type Backup struct {
	ID               string              `json:"id"`
	ServerID         string              `json:"server_id"`
	ServerName       string              `json:"server_name"`
	SnapshotID       string              `json:"snapshot_id"`
	RepositoryPrefix string              `json:"repository_prefix"`
	CreatedAt        time.Time           `json:"created_at"`
	DurationSeconds  int64               `json:"duration_seconds"`
	FileCount        int64               `json:"file_count"`
	RepositorySize   int64               `json:"repository_size"`
	MinecraftVersion string              `json:"minecraft_version"`
	Status           BackupStatus        `json:"status"`
	ServerDeletedAt  *time.Time          `json:"server_deleted_at,omitempty"`
	RetentionUntil   *time.Time          `json:"retention_until,omitempty"`
	SourceConfig     *BackupSourceConfig `json:"source_config,omitempty"`
}
