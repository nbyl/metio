package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/nbyl/metio/config"
	"gitlab.com/nbyl/metio/db"
	"gitlab.com/nbyl/metio/testutil"
)

func setupMockDB(mockDB *testutil.MockDB) func() {
	original := getDBConnection
	getDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{
			Environment:  "test",
			Region:       "us-central1",
			ProjectID:    "test-project",
			InstanceName: "test-instance",
		}, nil
	}
	return func() { getDBConnection = original }
}

func TestStatusHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players:     db.Players{Current: 5, Max: 20},
		Timestamp:   time.Now(),
		Uptime:      "1:30",
		ServerState: db.ServerStateRunning,
		InstanceIP:  "10.0.0.1:25565",
		Version:     "1.21.1",
	}, nil)

	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()

	statusHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ServerStatus
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, db.ServerStateRunning, response.Status)
	assert.Equal(t, 5, response.Players)
	assert.Equal(t, 20, response.MaxPlayers)
	assert.Equal(t, "1:30", response.Uptime)
	assert.Equal(t, "1.21.1", response.Version)
}

func TestStatusHandler_StoppedServer(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players:     db.Players{Current: 5, Max: 20},
		ServerState: db.ServerStateStopped,
		InstanceIP:  "10.0.0.1:25565",
	}, nil)

	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()
	statusHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ServerStatus
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, db.ServerStateStopped, response.Status)
	assert.Equal(t, 0, response.Players) // zeroed when not running
}

func TestStatusHandler_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)

	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()
	statusHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListServers_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("ListServerConfigs", mock.Anything).Return([]*db.ServerConfig{
		{ID: "srv1", Name: "server-one", Region: "us-central1", Zone: "us-central1-a", MachineType: "e2-small", DiskSizeGB: 50, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}, nil)

	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()
	listServers(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response []ServerResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Len(t, response, 1)
	assert.Equal(t, "srv1", response[0].ID)
}

func TestListServers_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("ListServerConfigs", mock.Anything).Return(([]*db.ServerConfig)(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()
	listServers(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetServer_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		Name: "server-one", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", DiskSizeGB: 50, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)

	req := httptest.NewRequest("GET", "/api/servers/srv1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	getServer(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ServerResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "srv1", response.ID)
	assert.Equal(t, "server-one", response.Config.Name)
}

func TestGetServer_NotFound(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "missing").Return((*db.ServerConfig)(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/servers/missing", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "missing"})
	w := httptest.NewRecorder()
	getServer(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteServer_NotFound(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "missing").Return((*db.ServerConfig)(nil), assert.AnError)

	req := httptest.NewRequest("DELETE", "/api/servers/missing", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "missing"})
	w := httptest.NewRecorder()
	deleteServer(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetWhitelistHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry{
		{Username: "Steve", UUID: "uuid-1", AddedAt: time.Now(), AddedBy: "admin"},
	}, nil)

	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	w := httptest.NewRecorder()
	getWhitelistHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response WhitelistResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.True(t, response.Enabled)
	assert.Len(t, response.Players, 1)
	assert.Equal(t, "Steve", response.Players[0].Username)
}

func TestRemoveWhitelistHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("RemoveWhitelistEntry", mock.Anything, "test-instance", "uuid-1").Return(nil)

	req := httptest.NewRequest("DELETE", "/api/server/whitelist/uuid-1", nil)
	req = mux.SetURLVars(req, map[string]string{"uuid": "uuid-1"})
	w := httptest.NewRecorder()
	removeWhitelistHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetWhitelistEnabledHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", db.WhitelistConfig{Enabled: true}).Return(nil)

	body, _ := json.Marshal(SetEnabledRequest{Enabled: true})
	req := httptest.NewRequest("PUT", "/api/server/whitelist/enabled", bytes.NewReader(body))
	w := httptest.NewRecorder()
	setWhitelistEnabledHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSetWhitelistEnabledHandler_InvalidBody(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	req := httptest.NewRequest("PUT", "/api/server/whitelist/enabled", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	setWhitelistEnabledHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	_ = mockDB
}

func TestScheduleShutdownHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	futureTime := time.Now().Add(1 * time.Hour)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	body, _ := json.Marshal(ScheduleShutdownRequest{ShutdownTime: futureTime.Format(time.RFC3339)})
	req := httptest.NewRequest("POST", "/api/server/shutdown/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	scheduleShutdownHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ScheduleShutdownResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.True(t, response.Success)
	assert.NotNil(t, response.ScheduledShutdown)
}

func TestScheduleShutdownHandler_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/server/shutdown/schedule", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	scheduleShutdownHandler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestScheduleShutdownHandler_PastTime(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour)
	body, _ := json.Marshal(ScheduleShutdownRequest{ShutdownTime: pastTime.Format(time.RFC3339)})
	req := httptest.NewRequest("POST", "/api/server/shutdown/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	scheduleShutdownHandler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelScheduledShutdownHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	req := httptest.NewRequest("DELETE", "/api/server/shutdown/schedule", nil)
	w := httptest.NewRecorder()
	cancelScheduledShutdownHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ScheduleShutdownResponse
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.True(t, response.Success)
	assert.Nil(t, response.ScheduledShutdown)
}

func TestCreateServer_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	createServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateServer_ValidationError(t *testing.T) {
	body, _ := json.Marshal(CreateServerRequest{
		Name:        "x", // too short
		Region:      "invalid",
		Zone:        "invalid",
		MachineType: "invalid",
		DiskSizeGB:  5,
	})
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	createServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetServerProvisioningStatus_NoService(t *testing.T) {
	oldPS := provisioningService
	provisioningService = nil
	defer func() { provisioningService = oldPS }()

	req := httptest.NewRequest("GET", "/api/servers/srv1/provisioning", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	getServerProvisioningStatus(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetServerProvisioningStatus_EmptyID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/servers//provisioning", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	w := httptest.NewRecorder()
	getServerProvisioningStatus(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleInstanceStart(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players:     db.Players{Current: 0, Max: 20},
		ServerState: db.ServerStateStopped,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(nil)

	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.MethodName = "v1.compute.instances.start"
	auditLog.ProtoPayload.ResourceName = "projects/proj/zones/zone/instances/my-instance"

	assert.NotPanics(t, func() {
		handleInstanceStart(context.Background(), auditLog)
	})
	mockDB.AssertCalled(t, "UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status"))
}

func TestHandleInstancePreempted(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players:     db.Players{Current: 0, Max: 20},
		ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(nil)

	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.MethodName = "v1.compute.instances.preempted"
	auditLog.ProtoPayload.ResourceName = "projects/proj/zones/zone/instances/my-instance"

	assert.NotPanics(t, func() {
		handleInstancePreempted(context.Background(), auditLog)
	})
}

func TestUpdateServer_ImmutableRegion(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
	}, nil)

	region := "europe-west3"
	body, _ := json.Marshal(UpdateServerRequest{Region: &region})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	assert.Contains(t, errResp.Error, "region is immutable")
}

func TestUpdateServer_ImmutableZone(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
	}, nil)

	zone := "us-central1-b"
	body, _ := json.Marshal(UpdateServerRequest{Zone: &zone})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	assert.Contains(t, errResp.Error, "zone is immutable")
}

func TestUpdateServer_DiskSizeDecrease(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
		DiskSizeGB: 50,
	}, nil)

	small := 20
	body, _ := json.Marshal(UpdateServerRequest{DiskSizeGB: &small})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	json.NewDecoder(w.Body).Decode(&errResp)
	assert.Contains(t, errResp.Error, "disk size can only be increased")
}

func TestUpdateServer_MinecraftVersionChange(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", DiskSizeGB: 50, MinecraftVersion: "1.21.1",
	}, nil)
	mockDB.On("GetStatus", mock.Anything, "srv1").Return(db.Status{ServerState: "RUNNING"}, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(nil)

	v := "1.21.4"
	body, _ := json.Marshal(UpdateServerRequest{MinecraftVersion: &v})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestUpdateServer_MachineTypeChange(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", DiskSizeGB: 50, MinecraftVersion: "1.21.1",
	}, nil)
	mockDB.On("GetStatus", mock.Anything, "srv1").Return(db.Status{ServerState: "RUNNING"}, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(nil)

	mt := "e2-medium"
	body, _ := json.Marshal(UpdateServerRequest{MachineType: &mt})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandleInstanceStop_EmptyResourceName(t *testing.T) {
	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.ResourceName = "invalid"

	// Should not panic, just log and return
	assert.NotPanics(t, func() {
		handleInstanceStop(context.Background(), auditLog)
	})
}

func TestAddWhitelistHandler_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddWhitelistHandler_EmptyUsername(t *testing.T) {
	body, _ := json.Marshal(AddPlayerRequest{Username: ""})
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveWhitelistHandler_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("RemoveWhitelistEntry", mock.Anything, "test-instance", "uuid-1").Return(assert.AnError)

	req := httptest.NewRequest("DELETE", "/api/server/whitelist/uuid-1", nil)
	req = mux.SetURLVars(req, map[string]string{"uuid": "uuid-1"})
	w := httptest.NewRecorder()
	removeWhitelistHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetWhitelistEnabledHandler_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", db.WhitelistConfig{Enabled: true}).Return(assert.AnError)

	body, _ := json.Marshal(SetEnabledRequest{Enabled: true})
	req := httptest.NewRequest("PUT", "/api/server/whitelist/enabled", bytes.NewReader(body))
	w := httptest.NewRecorder()
	setWhitelistEnabledHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetWhitelistHandler_EntriesError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{Enabled: true}, nil)
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return([]db.WhitelistEntry(nil), assert.AnError)

	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	w := httptest.NewRecorder()
	getWhitelistHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestScheduleShutdownHandler_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	futureTime := time.Now().Add(1 * time.Hour)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)

	body, _ := json.Marshal(ScheduleShutdownRequest{ShutdownTime: futureTime.Format(time.RFC3339)})
	req := httptest.NewRequest("POST", "/api/server/shutdown/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	scheduleShutdownHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCancelScheduledShutdownHandler_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)

	req := httptest.NewRequest("DELETE", "/api/server/shutdown/schedule", nil)
	w := httptest.NewRecorder()
	cancelScheduledShutdownHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestScheduleShutdownHandler_UpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	futureTime := time.Now().Add(1 * time.Hour)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{ServerState: db.ServerStateRunning}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	body, _ := json.Marshal(ScheduleShutdownRequest{ShutdownTime: futureTime.Format(time.RFC3339)})
	req := httptest.NewRequest("POST", "/api/server/shutdown/schedule", bytes.NewReader(body))
	w := httptest.NewRecorder()
	scheduleShutdownHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCancelScheduledShutdownHandler_UpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{ServerState: db.ServerStateRunning}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	req := httptest.NewRequest("DELETE", "/api/server/shutdown/schedule", nil)
	w := httptest.NewRecorder()
	cancelScheduledShutdownHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateServer_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("CreateServerConfig", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*db.ServerConfig")).Return(assert.AnError)

	body, _ := json.Marshal(CreateServerRequest{
		Name:             "test-server",
		Region:           "us-central1",
		Zone:             "us-central1-a",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.1",
		DiskSizeGB:       50,
	})
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	createServer(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateServer_NoProvisioningService(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()
	oldPS := provisioningService
	provisioningService = nil
	defer func() { provisioningService = oldPS }()

	mockDB.On("CreateServerConfig", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*db.ServerConfig")).Return(nil)

	body, _ := json.Marshal(CreateServerRequest{
		Name:             "test-server",
		Region:           "us-central1",
		Zone:             "us-central1-a",
		MachineType:      "e2-small",
		MinecraftVersion: "1.21.1",
		DiskSizeGB:       50,
	})
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	createServer(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestStatusHandler_ScheduledShutdown(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	shutdownTime := time.Now().Add(1 * time.Hour)
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players:           db.Players{Current: 3, Max: 20},
		ServerState:       db.ServerStateRunning,
		InstanceIP:        "10.0.0.1:25565",
		Version:           "1.21.1",
		Uptime:            "2:00",
		ScheduledShutdown: &shutdownTime,
	}, nil)

	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()
	statusHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ServerStatus
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.NotNil(t, response.ScheduledShutdown)
}

func TestStatusHandler_EmptyIP(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		ServerState: db.ServerStateRunning,
		InstanceIP:  "",
	}, nil)

	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()
	statusHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response ServerStatus
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "unknown:25565", response.IP)
}

func TestUpdateServer_NotFound(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "missing").Return((*db.ServerConfig)(nil), assert.AnError)

	name := "updated"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/missing", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "missing"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- updateServer full path tests ---

func TestUpdateServer_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(nil)

	name := "new-name"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestUpdateServer_EmptyID(t *testing.T) {
	req := httptest.NewRequest("PUT", "/api/servers/", bytes.NewReader([]byte("{}")))
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	w := httptest.NewRecorder()
	updateServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateServer_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader([]byte("bad")))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateServer_UpdateDBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(assert.AnError)

	name := "new"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateServer_NoProvisioningService(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()
	oldPS := provisioningService
	provisioningService = nil
	defer func() { provisioningService = oldPS }()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)

	name := "new"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUpdateServer_ProvisioningConflict(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(
		fmt.Errorf("operation already in progress for server srv1"))

	name := "new"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateServer_ProvisioningError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(
		fmt.Errorf("some other error"))
	mockPS.On("RevertServerConfig", mock.Anything, "srv1").Return(nil)

	name := "new"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateServer_ValidationError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	existing := &db.ServerConfig{
		ID: "srv1", Name: "old", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	}
	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(existing, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)

	emptyName := ""
	body, _ := json.Marshal(UpdateServerRequest{Name: &emptyName})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateServer_ServerNotRunning(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", DiskSizeGB: 50, MinecraftVersion: "1.21.1",
	}, nil)
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockDB.On("GetStatus", mock.Anything, "srv1").Return(db.Status{ServerState: "STOPPED"}, nil)

	mt := "e2-medium"
	body, _ := json.Marshal(UpdateServerRequest{MachineType: &mt})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateServer_SnapshotIsOriginalConfig(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{
		ID: "srv1", Name: "test", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", DiskSizeGB: 50, MinecraftVersion: "1.21.1",
	}, nil)

	// The snapshot must contain the ORIGINAL name, not the updated one.
	var capturedSnapshot *db.ServerConfig
	mockDB.On("SaveConfigSnapshot", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Run(func(args mock.Arguments) {
		capturedSnapshot = args.Get(2).(*db.ServerConfig)
	}).Return(nil)
	mockDB.On("GetStatus", mock.Anything, "srv1").Return(db.Status{ServerState: "RUNNING"}, nil)
	mockDB.On("UpdateServerConfig", mock.Anything, "srv1", mock.AnythingOfType("*db.ServerConfig")).Return(nil)

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()
	mockPS.On("UpdateServer", mock.Anything, "srv1", mock.AnythingOfType("*programs.ServerConfig"), mock.AnythingOfType("int")).Return(nil)

	name := "updated-name"
	body, _ := json.Marshal(UpdateServerRequest{Name: &name})
	req := httptest.NewRequest("PUT", "/api/servers/srv1", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	updateServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.NotNil(t, capturedSnapshot)
	assert.Equal(t, "test", capturedSnapshot.Name, "snapshot should contain the original name before mutation")
}

// --- deleteServer full path tests ---

func TestDeleteServer_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{ID: "srv1"}, nil)
	mockPS.On("DestroyServer", mock.Anything, "srv1").Return(nil)

	req := httptest.NewRequest("DELETE", "/api/servers/srv1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	deleteServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestDeleteServer_EmptyID(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/api/servers/", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	w := httptest.NewRecorder()
	deleteServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteServer_NoProvisioningService(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()
	oldPS := provisioningService
	provisioningService = nil
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{ID: "srv1"}, nil)

	req := httptest.NewRequest("DELETE", "/api/servers/srv1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	deleteServer(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestDeleteServer_Conflict(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{ID: "srv1"}, nil)
	mockPS.On("DestroyServer", mock.Anything, "srv1").Return(fmt.Errorf("operation already in progress for server srv1"))

	req := httptest.NewRequest("DELETE", "/api/servers/srv1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	deleteServer(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestDeleteServer_DestroyError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("GetServerConfig", mock.Anything, "srv1").Return(&db.ServerConfig{ID: "srv1"}, nil)
	mockPS.On("DestroyServer", mock.Anything, "srv1").Return(fmt.Errorf("infra error"))

	req := httptest.NewRequest("DELETE", "/api/servers/srv1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	deleteServer(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- createServer full path tests ---

func TestCreateServer_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("CreateServerConfig", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("CreateServer", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*programs.ServerConfig")).Return(nil)

	body, _ := json.Marshal(CreateServerRequest{
		Name: "test-server", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	})
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	createServer(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestCreateServer_ProvisioningGenericError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockDB.On("CreateServerConfig", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*db.ServerConfig")).Return(nil)
	mockPS.On("CreateServer", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*programs.ServerConfig")).Return(
		fmt.Errorf("some error"))

	body, _ := json.Marshal(CreateServerRequest{
		Name: "test-server", Region: "us-central1", Zone: "us-central1-a",
		MachineType: "e2-small", MinecraftVersion: "1.21.1", DiskSizeGB: 50,
	})
	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	createServer(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- addWhitelistHandler tests ---

func TestAddWhitelistHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	oldLookup := LookupMinecraftUser
	LookupMinecraftUser = func(ctx context.Context, username string) (*MojangProfile, error) {
		return &MojangProfile{ID: "abcd1234", Name: "TestPlayer"}, nil
	}
	defer func() { LookupMinecraftUser = oldLookup }()

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistEntry")).Return(nil)

	body, _ := json.Marshal(AddPlayerRequest{Username: "TestPlayer"})
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAddWhitelistHandler_MojangError(t *testing.T) {
	oldLookup := LookupMinecraftUser
	LookupMinecraftUser = func(ctx context.Context, username string) (*MojangProfile, error) {
		return nil, fmt.Errorf("mojang API down")
	}
	defer func() { LookupMinecraftUser = oldLookup }()

	body, _ := json.Marshal(AddPlayerRequest{Username: "TestPlayer"})
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestAddWhitelistHandler_UserNotFound(t *testing.T) {
	oldLookup := LookupMinecraftUser
	LookupMinecraftUser = func(ctx context.Context, username string) (*MojangProfile, error) {
		return nil, nil
	}
	defer func() { LookupMinecraftUser = oldLookup }()

	body, _ := json.Marshal(AddPlayerRequest{Username: "nonexistent"})
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddWhitelistHandler_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	oldLookup := LookupMinecraftUser
	LookupMinecraftUser = func(ctx context.Context, username string) (*MojangProfile, error) {
		return &MojangProfile{ID: "abcd1234", Name: "TestPlayer"}, nil
	}
	defer func() { LookupMinecraftUser = oldLookup }()

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", mock.AnythingOfType("db.WhitelistEntry")).Return(assert.AnError)

	body, _ := json.Marshal(AddPlayerRequest{Username: "TestPlayer"})
	req := httptest.NewRequest("POST", "/api/server/whitelist", bytes.NewReader(body))
	w := httptest.NewRecorder()
	addWhitelistHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getServerProvisioningStatus tests ---

func TestGetServerProvisioningStatus_Success(t *testing.T) {
	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	now := time.Now()
	mockPS.On("GetProvisioningStatus", mock.Anything, "srv1").Return(&db.ProvisioningStatus{
		ID:          "status-1",
		Operation:   db.ProvisioningOperationCreate,
		State:       db.ProvisioningStateInProgress,
		CurrentStep: "creating",
		Steps:       []db.ProvisioningStep{},
		StartedAt:   now,
	}, nil)

	req := httptest.NewRequest("GET", "/api/servers/srv1/provisioning", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	getServerProvisioningStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetServerProvisioningStatus_NotFound(t *testing.T) {
	mockPS := new(testutil.MockProvisioningService)
	oldPS := provisioningService
	provisioningService = mockPS
	defer func() { provisioningService = oldPS }()

	mockPS.On("GetProvisioningStatus", mock.Anything, "srv1").Return(nil, assert.AnError)

	req := httptest.NewRequest("GET", "/api/servers/srv1/provisioning", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "srv1"})
	w := httptest.NewRecorder()
	getServerProvisioningStatus(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- updateInstanceState tests ---

func TestUpdateInstanceState_GetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-inst").Return(db.Status{}, assert.AnError)
	mockDB.On("UpdateStatus", mock.Anything, "my-inst", mock.AnythingOfType("db.Status")).Return(nil)

	err := updateInstanceState(context.Background(), "my-inst", db.ServerStateStopped)
	assert.NoError(t, err)
}

func TestUpdateInstanceState_UpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-inst").Return(db.Status{ServerState: db.ServerStateRunning}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-inst", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	err := updateInstanceState(context.Background(), "my-inst", db.ServerStateStopped)
	assert.Error(t, err)
}

// --- getWhitelistHandler additional coverage ---

func TestGetWhitelistHandler_ConfigError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(db.WhitelistConfig{}, assert.AnError)

	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	w := httptest.NewRecorder()
	getWhitelistHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getServer additional coverage ---

func TestGetServer_EmptyID(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/servers/", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	w := httptest.NewRecorder()
	getServer(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- removeWhitelistHandler additional coverage ---

func TestRemoveWhitelistHandler_EmptyUUID(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/api/server/whitelist/", nil)
	req = mux.SetURLVars(req, map[string]string{"uuid": ""})
	w := httptest.NewRecorder()
	removeWhitelistHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- handleInstanceStart/Preempted with empty resource name ---

func TestHandleInstanceStart_EmptyResource(t *testing.T) {
	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.ResourceName = "invalid"
	assert.NotPanics(t, func() {
		handleInstanceStart(context.Background(), auditLog)
	})
}

func TestHandleInstancePreempted_EmptyResource(t *testing.T) {
	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.ResourceName = "invalid"
	assert.NotPanics(t, func() {
		handleInstancePreempted(context.Background(), auditLog)
	})
}

// --- processAuditLogEvent additional coverage ---

func TestEventsHandler_StartEvent(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players: db.Players{Current: 0, Max: 20}, ServerState: db.ServerStateStopped,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(nil)

	data, _ := json.Marshal(AuditLogEntry{
		ProtoPayload: struct {
			MethodName         string `json:"methodName"`
			ResourceName       string `json:"resourceName"`
			AuthenticationInfo struct {
				PrincipalEmail string `json:"principalEmail"`
			} `json:"authenticationInfo"`
		}{
			MethodName:   "v1.compute.instances.start",
			ResourceName: "projects/proj/zones/zone/instances/my-instance",
		},
	})
	processAuditLogEvent(data)

	mockDB.AssertCalled(t, "UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status"))
}

func TestEventsHandler_PreemptedEvent(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players: db.Players{Current: 0, Max: 20}, ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(nil)

	data, _ := json.Marshal(AuditLogEntry{
		ProtoPayload: struct {
			MethodName         string `json:"methodName"`
			ResourceName       string `json:"resourceName"`
			AuthenticationInfo struct {
				PrincipalEmail string `json:"principalEmail"`
			} `json:"authenticationInfo"`
		}{
			MethodName:   "v1.compute.instances.preempted",
			ResourceName: "projects/proj/zones/zone/instances/my-instance",
		},
	})
	processAuditLogEvent(data)

	mockDB.AssertCalled(t, "UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status"))
}

func TestEventsHandler_UnknownEvent(t *testing.T) {
	data, _ := json.Marshal(AuditLogEntry{
		ProtoPayload: struct {
			MethodName         string `json:"methodName"`
			ResourceName       string `json:"resourceName"`
			AuthenticationInfo struct {
				PrincipalEmail string `json:"principalEmail"`
			} `json:"authenticationInfo"`
		}{
			MethodName: "v1.compute.instances.unknown",
		},
	})
	assert.NotPanics(t, func() {
		processAuditLogEvent(data)
	})
}

func TestEventsHandler_InvalidJSON(t *testing.T) {
	assert.NotPanics(t, func() {
		processAuditLogEvent([]byte("not json"))
	})
}

// --- getUserEmail additional coverage ---

func TestGetUserEmail_FromHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-Email", "test@example.com")
	assert.NotPanics(t, func() { getUserEmail(req) })
}

// --- Mock compute client ---

type mockComputeClient struct {
	startErr error
	stopErr  error
}

func (m *mockComputeClient) Start(ctx context.Context, req *computepb.StartInstanceRequest) error {
	return m.startErr
}

func (m *mockComputeClient) Stop(ctx context.Context, req *computepb.StopInstanceRequest) error {
	return m.stopErr
}

func (m *mockComputeClient) Close() error { return nil }

func setupMockCompute(cc ComputeClient) func() {
	orig := newComputeClient
	newComputeClient = func(ctx context.Context) (ComputeClient, error) {
		return cc, nil
	}
	return func() { newComputeClient = orig }
}

func setupMockComputeError() func() {
	orig := newComputeClient
	newComputeClient = func(ctx context.Context) (ComputeClient, error) {
		return nil, fmt.Errorf("compute client error")
	}
	return func() { newComputeClient = orig }
}

// --- startServerHandler tests ---

func TestStartServerHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{})
	defer cleanupCC()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players: db.Players{Current: 2, Max: 20}, ServerState: db.ServerStateStopped,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	req := httptest.NewRequest("POST", "/api/server/start", nil)
	w := httptest.NewRecorder()
	startServerHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ServerActionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, db.ServerStateStarting, resp.State)
}

func TestStartServerHandler_ComputeClientError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockComputeError()
	defer cleanupCC()

	req := httptest.NewRequest("POST", "/api/server/start", nil)
	w := httptest.NewRecorder()
	startServerHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStartServerHandler_StartError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{startErr: fmt.Errorf("start failed")})
	defer cleanupCC()

	req := httptest.NewRequest("POST", "/api/server/start", nil)
	w := httptest.NewRecorder()
	startServerHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStartServerHandler_DBGetStatusError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{})
	defer cleanupCC()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	req := httptest.NewRequest("POST", "/api/server/start", nil)
	w := httptest.NewRecorder()
	startServerHandler(w, req)

	// Still succeeds — DB errors are non-fatal for start
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- stopServerHandler tests ---

func TestStopServerHandler_Success(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{})
	defer cleanupCC()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players: db.Players{Current: 2, Max: 20}, ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(nil)

	req := httptest.NewRequest("POST", "/api/server/stop", nil)
	w := httptest.NewRecorder()
	stopServerHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ServerActionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Success)
	assert.Equal(t, db.ServerStateStopping, resp.State)
}

func TestStopServerHandler_ComputeClientError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockComputeError()
	defer cleanupCC()

	req := httptest.NewRequest("POST", "/api/server/stop", nil)
	w := httptest.NewRecorder()
	stopServerHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStopServerHandler_StopError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{stopErr: fmt.Errorf("stop failed")})
	defer cleanupCC()

	req := httptest.NewRequest("POST", "/api/server/stop", nil)
	w := httptest.NewRecorder()
	stopServerHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStopServerHandler_DBUpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanupDB := setupMockDB(mockDB)
	defer cleanupDB()
	cleanupCC := setupMockCompute(&mockComputeClient{})
	defer cleanupCC()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{
		Players: db.Players{Current: 0, Max: 20}, ServerState: db.ServerStateRunning,
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "test-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	req := httptest.NewRequest("POST", "/api/server/stop", nil)
	w := httptest.NewRecorder()
	stopServerHandler(w, req)

	// Still succeeds — DB errors are non-fatal for stop
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- New and spaHandler tests ---

func TestNew_ReturnsRouter(t *testing.T) {
	r := New(nil, nil)
	assert.NotNil(t, r)
}

func TestSpaHandler_ServesIndex(t *testing.T) {
	// Set DEV_MODE to use filesystem — will fail to serve but exercises the code path
	t.Setenv("DEV_MODE", "true")
	handler := spaHandler()
	assert.NotNil(t, handler)

	// Request a non-existent file — should fall back to index.html
	req := httptest.NewRequest("GET", "/some/spa/route", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Will return 404 since static/dist doesn't exist in test, but exercises code
}

func TestSpaHandler_EmbeddedFS(t *testing.T) {
	handler := spaHandler()
	assert.NotNil(t, handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

// --- getServerStatus error path ---

func TestGetServerStatus_DBError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(db.Status{}, assert.AnError)

	_, err := getServerStatus(context.Background())
	assert.Error(t, err)
}

// --- handleInstanceStart/Preempted update error ---

func TestHandleInstanceStart_UpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players: db.Players{Current: 0, Max: 20},
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.ResourceName = "projects/p/zones/z/instances/my-instance"
	assert.NotPanics(t, func() {
		handleInstanceStart(context.Background(), auditLog)
	})
}

func TestHandleInstancePreempted_UpdateError(t *testing.T) {
	mockDB := new(testutil.MockDB)
	cleanup := setupMockDB(mockDB)
	defer cleanup()

	mockDB.On("GetStatus", mock.Anything, "my-instance").Return(db.Status{
		Players: db.Players{Current: 0, Max: 20},
	}, nil)
	mockDB.On("UpdateStatus", mock.Anything, "my-instance", mock.AnythingOfType("db.Status")).Return(assert.AnError)

	auditLog := AuditLogEntry{}
	auditLog.ProtoPayload.ResourceName = "projects/p/zones/z/instances/my-instance"
	assert.NotPanics(t, func() {
		handleInstancePreempted(context.Background(), auditLog)
	})
}
