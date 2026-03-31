package db

import "time"

type ProvisioningState int

const (
	ProvisioningStatePending ProvisioningState = iota
	ProvisioningStateInProgress
	ProvisioningStateCompleted
	ProvisioningStateFailed
)

func (s ProvisioningState) String() string {
	switch s {
	case ProvisioningStatePending:
		return "PENDING"
	case ProvisioningStateInProgress:
		return "IN_PROGRESS"
	case ProvisioningStateCompleted:
		return "COMPLETED"
	case ProvisioningStateFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func (s ProvisioningState) FirestoreValue() string {
	switch s {
	case ProvisioningStatePending:
		return "pending"
	case ProvisioningStateInProgress:
		return "in_progress"
	case ProvisioningStateCompleted:
		return "completed"
	case ProvisioningStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type ProvisioningOperation int

const (
	ProvisioningOperationCreate ProvisioningOperation = iota
	ProvisioningOperationUpdate
	ProvisioningOperationDestroy
)

func (o ProvisioningOperation) String() string {
	switch o {
	case ProvisioningOperationCreate:
		return "CREATE"
	case ProvisioningOperationUpdate:
		return "UPDATE"
	case ProvisioningOperationDestroy:
		return "DESTROY"
	default:
		return "UNKNOWN"
	}
}

func (o ProvisioningOperation) FirestoreValue() string {
	switch o {
	case ProvisioningOperationCreate:
		return "create"
	case ProvisioningOperationUpdate:
		return "update"
	case ProvisioningOperationDestroy:
		return "destroy"
	default:
		return "unknown"
	}
}

type ProvisioningStep struct {
	Name      string            `firestore:"name"`
	Status    ProvisioningState `firestore:"status"`
	Message   string            `firestore:"message"`
	Timestamp time.Time         `firestore:"timestamp"`
}

type ProvisioningStatus struct {
	ID          string                `firestore:"id"`
	Operation   ProvisioningOperation `firestore:"operation"`
	State       ProvisioningState     `firestore:"state"`
	StartedAt   time.Time             `firestore:"started_at"`
	CompletedAt *time.Time            `firestore:"completed_at,omitempty"`
	CurrentStep string                `firestore:"current_step"`
	Steps       []ProvisioningStep    `firestore:"steps"`
	Error       string                `firestore:"error,omitempty"`
	Outputs     map[string]string     `firestore:"outputs,omitempty"`
}
