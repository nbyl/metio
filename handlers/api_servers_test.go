package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/nbyl/metio/db"
)

func TestCreateServerRequestStruct(t *testing.T) {
	req := CreateServerRequest{
		Name:             "my-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-a",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.1",
		DiskSizeGB:       50,
		ShutdownSchedule: &ShutdownScheduleInput{
			Enabled:  true,
			Time:     "21:00",
			Timezone: "Europe/Berlin",
		},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "my-server", parsed["name"])
	assert.Equal(t, "europe-west3", parsed["region"])
	assert.Equal(t, "europe-west3-a", parsed["zone"])
	assert.Equal(t, "e2-small", parsed["machineType"])
	assert.Equal(t, "1.21.1", parsed["minecraftVersion"])
	assert.Equal(t, float64(50), parsed["diskSizeGB"])

	schedule := parsed["shutdownSchedule"].(map[string]interface{})
	assert.Equal(t, true, schedule["enabled"])
	assert.Equal(t, "21:00", schedule["time"])
	assert.Equal(t, "Europe/Berlin", schedule["timezone"])
}

func TestCreateServerRequest_MinimalFields(t *testing.T) {
	jsonStr := `{
		"name": "minimal-server",
		"region": "us-east1",
		"zone": "us-east1-b",
		"machineType": "e2-medium",
		"minecraftVersion": "1.20.4"
	}`

	var req CreateServerRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	assert.NoError(t, err)

	assert.Equal(t, "minimal-server", req.Name)
	assert.Equal(t, "us-east1", req.Region)
	assert.Equal(t, "e2-medium", req.MachineType)
	assert.Equal(t, 0, req.DiskSizeGB)
	assert.Nil(t, req.ShutdownSchedule)
}

func TestUpdateServerRequestStruct(t *testing.T) {
	name := "updated-server"
	region := "us-west1"
	diskSize := 100

	req := UpdateServerRequest{
		Name:       &name,
		Region:     &region,
		DiskSizeGB: &diskSize,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "updated-server", parsed["name"])
	assert.Equal(t, "us-west1", parsed["region"])
	assert.Equal(t, float64(100), parsed["diskSizeGB"])
	assert.NotContains(t, parsed, "zone")
	assert.NotContains(t, parsed, "machineType")
}

func TestUpdateServerRequest_PartialUpdate(t *testing.T) {
	jsonStr := `{"machineType": "n2-standard-4"}`

	var req UpdateServerRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	assert.NoError(t, err)

	assert.Nil(t, req.Name)
	assert.Nil(t, req.Region)
	assert.Nil(t, req.Zone)
	assert.NotNil(t, req.MachineType)
	assert.Equal(t, "n2-standard-4", *req.MachineType)
	assert.Nil(t, req.DiskSizeGB)
}

func TestServerConfigJSONStruct(t *testing.T) {
	cfg := ServerConfigJSON{
		Name:             "test-server",
		Region:           "europe-west3",
		Zone:             "europe-west3-b",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.3",
		DiskSizeGB:       25,
		ShutdownSchedule: &ShutdownScheduleInput{
			Enabled:  true,
			Time:     "22:00",
			Timezone: "UTC",
		},
		CreatedAt: "2026-03-31T10:00:00Z",
		UpdatedAt: "2026-03-31T12:00:00Z",
	}

	data, err := json.Marshal(cfg)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "test-server", parsed["name"])
	assert.Equal(t, "europe-west3", parsed["region"])
	assert.Equal(t, "europe-west3-b", parsed["zone"])
	assert.Equal(t, "e2-small", parsed["machineType"])
	assert.Equal(t, "1.21.3", parsed["minecraftVersion"])
	assert.Equal(t, float64(25), parsed["diskSizeGB"])
	assert.Equal(t, "2026-03-31T10:00:00Z", parsed["createdAt"])
	assert.Equal(t, "2026-03-31T12:00:00Z", parsed["updatedAt"])

	schedule := parsed["shutdownSchedule"].(map[string]interface{})
	assert.Equal(t, true, schedule["enabled"])
	assert.Equal(t, "22:00", schedule["time"])
}

func TestServerResponseStruct(t *testing.T) {
	response := ServerResponse{
		ID: "server-123-uuid",
		Config: ServerConfigJSON{
			Name:             "my-server",
			Region:           "us-central1",
			Zone:             "us-central1-a",
			MachineType:      "e2-medium",
			MinecraftVersion: "1.21.1",
			DiskSizeGB:       50,
			CreatedAt:        "2026-03-31T10:00:00Z",
			UpdatedAt:        "2026-03-31T10:00:00Z",
		},
		Status: &StatusResponse{
			Players: PlayersJSON{
				Current: 5,
				Max:     20,
			},
			Timestamp:        "2026-03-31T12:00:00Z",
			Uptime:           "2 days, 3:45",
			ServerState:      "RUNNING",
			InstanceIP:       "34.1.2.3",
			Version:          "1.21.1",
			WhitelistEnabled: true,
		},
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "server-123-uuid", parsed["id"])

	config := parsed["config"].(map[string]interface{})
	assert.Equal(t, "my-server", config["name"])

	status := parsed["status"].(map[string]interface{})
	assert.Equal(t, "RUNNING", status["serverState"])
	assert.Equal(t, "34.1.2.3", status["instanceIP"])

	players := status["players"].(map[string]interface{})
	assert.Equal(t, float64(5), players["current"])
	assert.Equal(t, float64(20), players["max"])
}

func TestServerResponse_WithoutStatus(t *testing.T) {
	response := ServerResponse{
		ID: "server-456",
		Config: ServerConfigJSON{
			Name:             "new-server",
			Region:           "europe-west3",
			Zone:             "europe-west3-c",
			MachineType:      "e2-small",
			MinecraftVersion: "1.21.10",
			DiskSizeGB:       10,
			CreatedAt:        "2026-03-31T10:00:00Z",
			UpdatedAt:        "2026-03-31T10:00:00Z",
		},
		Status: nil,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "server-456", parsed["id"])
	_, hasStatus := parsed["status"]
	assert.False(t, hasStatus, "omitempty should exclude nil status")
}

func TestShutdownScheduleInput_Disabled(t *testing.T) {
	input := ShutdownScheduleInput{
		Enabled: false,
	}

	data, err := json.Marshal(input)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, false, parsed["enabled"])
	_, hasTime := parsed["time"]
	assert.False(t, hasTime, "omitempty should exclude time when empty")
	_, hasTimezone := parsed["timezone"]
	assert.False(t, hasTimezone, "omitempty should exclude timezone when empty")
}

func TestShutdownScheduleToInput(t *testing.T) {
	dbSchedule := &db.ShutdownSchedule{
		Enabled:  true,
		Time:     "20:00",
		Timezone: "America/New_York",
	}

	input := shutdownScheduleToInput(dbSchedule)
	assert.NotNil(t, input)
	assert.True(t, input.Enabled)
	assert.Equal(t, "20:00", input.Time)
	assert.Equal(t, "America/New_York", input.Timezone)
}

func TestShutdownScheduleToInput_Nil(t *testing.T) {
	input := shutdownScheduleToInput(nil)
	assert.Nil(t, input)
}

func TestShutdownScheduleFromInput(t *testing.T) {
	input := &ShutdownScheduleInput{
		Enabled:  true,
		Time:     "18:30",
		Timezone: "Europe/London",
	}

	schedule := shutdownScheduleFromInput(input)
	assert.NotNil(t, schedule)
	assert.True(t, schedule.Enabled)
	assert.Equal(t, "18:30", schedule.Time)
	assert.Equal(t, "Europe/London", schedule.Timezone)
}

func TestShutdownScheduleFromInput_Nil(t *testing.T) {
	schedule := shutdownScheduleFromInput(nil)
	assert.Nil(t, schedule)
}

func TestServerConfigToJSON(t *testing.T) {
	now := time.Now()
	dbConfig := &db.ServerConfig{
		ID:               "test-uuid",
		Name:             "converted-server",
		Region:           "asia-east1",
		Zone:             "asia-east1-a",
		MachineType:      "n2-standard-2",
		MinecraftVersion: "1.20.1",
		DiskSizeGB:       100,
		ShutdownSchedule: &db.ShutdownSchedule{
			Enabled:  true,
			Time:     "23:00",
			Timezone: "Asia/Tokyo",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	json := serverConfigToJSON(dbConfig)

	assert.Equal(t, "converted-server", json.Name)
	assert.Equal(t, "asia-east1", json.Region)
	assert.Equal(t, "asia-east1-a", json.Zone)
	assert.Equal(t, "n2-standard-2", json.MachineType)
	assert.Equal(t, "1.20.1", json.MinecraftVersion)
	assert.Equal(t, 100, json.DiskSizeGB)
	assert.NotNil(t, json.ShutdownSchedule)
	assert.True(t, json.ShutdownSchedule.Enabled)
	assert.Equal(t, "23:00", json.ShutdownSchedule.Time)
	assert.Equal(t, "Asia/Tokyo", json.ShutdownSchedule.Timezone)
}

func TestErrorResponseStruct(t *testing.T) {
	errResp := ErrorResponse{
		Error:   "validation_error",
		Message: "name must be between 3 and 63 characters",
	}

	data, err := json.Marshal(errResp)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "validation_error", parsed["error"])
	assert.Equal(t, "name must be between 3 and 63 characters", parsed["message"])
}

func TestStatusResponseStruct(t *testing.T) {
	scheduledShutdown := "2026-03-31T21:00:00Z"

	response := StatusResponse{
		Players: PlayersJSON{
			Current: 10,
			Max:     20,
		},
		Timestamp:         "2026-03-31T15:00:00Z",
		Uptime:            "5 days, 2:30",
		ServerState:       "RUNNING",
		InstanceIP:        "192.168.1.100",
		Version:           "1.21.1",
		WhitelistEnabled:  true,
		ScheduledShutdown: &scheduledShutdown,
	}

	data, err := json.Marshal(response)
	assert.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "RUNNING", parsed["serverState"])
	assert.Equal(t, "192.168.1.100", parsed["instanceIP"])
	assert.Equal(t, "5 days, 2:30", parsed["uptime"])
	assert.Equal(t, true, parsed["whitelistEnabled"])
	assert.Equal(t, "2026-03-31T21:00:00Z", parsed["scheduledShutdown"])

	players := parsed["players"].(map[string]interface{})
	assert.Equal(t, float64(10), players["current"])
	assert.Equal(t, float64(20), players["max"])
}

func TestClassifyUpdate_InPlace(t *testing.T) {
	cfg := &db.ServerConfig{
		Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1",
	}

	req := UpdateServerRequest{Name: strPtr("new-name")}
	assert.Equal(t, 0, classifyUpdate(req, cfg))

	req = UpdateServerRequest{DiskSizeGB: intPtr(100)}
	assert.Equal(t, 0, classifyUpdate(req, cfg))

	req = UpdateServerRequest{Name: strPtr("n"), DiskSizeGB: intPtr(200)}
	assert.Equal(t, 0, classifyUpdate(req, cfg))
}

func TestClassifyUpdate_Resize(t *testing.T) {
	cfg := &db.ServerConfig{
		Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1",
	}

	req := UpdateServerRequest{MachineType: strPtr("e2-medium")}
	assert.Equal(t, 1, classifyUpdate(req, cfg))
}

func TestClassifyUpdate_Recreate(t *testing.T) {
	cfg := &db.ServerConfig{
		Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1",
	}

	req := UpdateServerRequest{MinecraftVersion: strPtr("1.21.4")}
	assert.Equal(t, 2, classifyUpdate(req, cfg))
}

func TestClassifyUpdate_ResizeOverridesInPlace(t *testing.T) {
	cfg := &db.ServerConfig{
		Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1",
	}

	req := UpdateServerRequest{Name: strPtr("n"), MachineType: strPtr("e2-standard-2")}
	assert.Equal(t, 1, classifyUpdate(req, cfg))
}

func TestClassifyUpdate_RecreateOverridesOthers(t *testing.T) {
	cfg := &db.ServerConfig{
		Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1",
	}

	req := UpdateServerRequest{
		Name: strPtr("n"), MachineType: strPtr("e2-standard-2"),
		MinecraftVersion: strPtr("1.21.4"),
	}
	assert.Equal(t, 2, classifyUpdate(req, cfg))
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
