package db

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisioningOperation_String(t *testing.T) {
	tests := []struct {
		op       ProvisioningOperation
		expected string
	}{
		{ProvisioningOperationCreate, "CREATE"},
		{ProvisioningOperationUpdate, "UPDATE"},
		{ProvisioningOperationDestroy, "DESTROY"},
		{ProvisioningOperationRestore, "RESTORE"},
		{ProvisioningOperation(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.op.String())
	}
}

func TestProvisioningOperation_MarshalJSON(t *testing.T) {
	data, err := json.Marshal(ProvisioningOperationRestore)
	require.NoError(t, err)
	assert.JSONEq(t, `"RESTORE"`, string(data))
}

func TestProvisioningOperation_UnmarshalJSON(t *testing.T) {
	var op ProvisioningOperation
	require.NoError(t, json.Unmarshal([]byte(`"RESTORE"`), &op))
	assert.Equal(t, ProvisioningOperationRestore, op)

	require.NoError(t, json.Unmarshal([]byte(`"CREATE"`), &op))
	assert.Equal(t, ProvisioningOperationCreate, op)

	err := json.Unmarshal([]byte(`"BOGUS"`), &op)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown ProvisioningOperation")
}

func TestProvisioningStatus_JSONRoundTrip(t *testing.T) {
	status := ProvisioningStatus{
		ID:        "srv-123",
		Operation: ProvisioningOperationRestore,
		State:     ProvisioningStateInProgress,
		Outputs:   map[string]string{"backupId": "b1", "snapshotId": "abc123"},
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)

	var got ProvisioningStatus
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, ProvisioningOperationRestore, got.Operation)
	assert.Equal(t, "b1", got.Outputs["backupId"])
	assert.Equal(t, "abc123", got.Outputs["snapshotId"])
}
