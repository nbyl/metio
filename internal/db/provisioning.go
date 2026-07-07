package db

import (
	"encoding/json"
	"fmt"
	"time"
)

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

func (s ProvisioningState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *ProvisioningState) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "PENDING":
		*s = ProvisioningStatePending
	case "IN_PROGRESS":
		*s = ProvisioningStateInProgress
	case "COMPLETED":
		*s = ProvisioningStateCompleted
	case "FAILED":
		*s = ProvisioningStateFailed
	default:
		return fmt.Errorf("unknown ProvisioningState: %s", str)
	}
	return nil
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

func (o ProvisioningOperation) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.String())
}

func (o *ProvisioningOperation) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "CREATE":
		*o = ProvisioningOperationCreate
	case "UPDATE":
		*o = ProvisioningOperationUpdate
	case "DESTROY":
		*o = ProvisioningOperationDestroy
	default:
		return fmt.Errorf("unknown ProvisioningOperation: %s", str)
	}
	return nil
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
	Name      string            `json:"name" firestore:"name"`
	Status    ProvisioningState `json:"status" firestore:"status"`
	Message   string            `json:"message" firestore:"message"`
	Timestamp time.Time         `json:"timestamp" firestore:"timestamp"`
}

type ProvisioningStatus struct {
	ID          string                `json:"id" firestore:"id"`
	Operation   ProvisioningOperation `json:"operation" firestore:"operation"`
	State       ProvisioningState     `json:"state" firestore:"state"`
	StartedAt   time.Time             `json:"startedAt" firestore:"started_at"`
	CompletedAt *time.Time            `json:"completedAt,omitempty" firestore:"completed_at,omitempty"`
	CurrentStep string                `json:"currentStep" firestore:"current_step"`
	Steps       []ProvisioningStep    `json:"steps" firestore:"steps"`
	Error       string                `json:"error,omitempty" firestore:"error,omitempty"`
	Outputs     map[string]string     `json:"outputs,omitempty" firestore:"outputs,omitempty"`
}
