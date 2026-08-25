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
	ProvisioningOperationRestore
)

func (o ProvisioningOperation) String() string {
	switch o {
	case ProvisioningOperationCreate:
		return "CREATE"
	case ProvisioningOperationUpdate:
		return "UPDATE"
	case ProvisioningOperationDestroy:
		return "DESTROY"
	case ProvisioningOperationRestore:
		return "RESTORE"
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
	case "RESTORE":
		*o = ProvisioningOperationRestore
	default:
		return fmt.Errorf("unknown ProvisioningOperation: %s", str)
	}
	return nil
}

type ProvisioningStep struct {
	Name      string            `json:"name"`
	Status    ProvisioningState `json:"status"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
}

type ProvisioningStatus struct {
	ID          string                `json:"id"`
	Operation   ProvisioningOperation `json:"operation"`
	State       ProvisioningState     `json:"state"`
	StartedAt   time.Time             `json:"startedAt"`
	CompletedAt *time.Time            `json:"completedAt,omitempty"`
	CurrentStep string                `json:"currentStep"`
	Steps       []ProvisioningStep    `json:"steps"`
	Error       string                `json:"error,omitempty"`
	Outputs     map[string]string     `json:"outputs,omitempty"`
}
