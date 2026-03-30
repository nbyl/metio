package db

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
	Current int `firestore:"current"`
	Max     int `firestore:"max"`
}

type Status struct {
	Players           Players     `firestore:"players"`
	Timestamp         time.Time   `firestore:"timestamp"`
	Uptime            string      `firestore:"uptime"`
	ServerState       ServerState `firestore:"server_state"`
	InstanceIP        string      `firestore:"instance_ip"`
	Version           string      `firestore:"version"`
	WhitelistEnabled  bool        `firestore:"whitelist_enabled"`
	ScheduledShutdown *time.Time  `firestore:"scheduled_shutdown,omitempty"`
}

// WhitelistEntry represents a player in the whitelist
type WhitelistEntry struct {
	Username string    `firestore:"username"`
	UUID     string    `firestore:"uuid"`
	AddedAt  time.Time `firestore:"added_at"`
	AddedBy  string    `firestore:"added_by"`
}

// WhitelistConfig represents the whitelist configuration
type WhitelistConfig struct {
	Enabled bool `firestore:"enabled"`
}

type OperationState int

const (
	OperationStatePending OperationState = iota
	OperationStateRunning
	OperationStateCompleted
	OperationStateFailed
	OperationStateCancelled
)

func (s OperationState) String() string {
	switch s {
	case OperationStatePending:
		return "PENDING"
	case OperationStateRunning:
		return "RUNNING"
	case OperationStateCompleted:
		return "COMPLETED"
	case OperationStateFailed:
		return "FAILED"
	case OperationStateCancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

type OperationType int

const (
	OperationTypeCreate OperationType = iota
	OperationTypeUpdate
	OperationTypeDelete
)

func (t OperationType) String() string {
	switch t {
	case OperationTypeCreate:
		return "CREATE"
	case OperationTypeUpdate:
		return "UPDATE"
	case OperationTypeDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

type OperationStep struct {
	Name        string `firestore:"name"`
	Description string `firestore:"description"`
	Completed   bool   `firestore:"completed"`
	Error       string `firestore:"error,omitempty"`
}

type Operation struct {
	ID          string            `firestore:"id"`
	Type        OperationType     `firestore:"type"`
	State       OperationState    `firestore:"state"`
	CurrentStep string            `firestore:"current_step"`
	Steps       []OperationStep   `firestore:"steps"`
	Error       string            `firestore:"error,omitempty"`
	CreatedAt   time.Time         `firestore:"created_at"`
	UpdatedAt   time.Time         `firestore:"updated_at"`
	Outputs     map[string]string `firestore:"outputs,omitempty"`
}
