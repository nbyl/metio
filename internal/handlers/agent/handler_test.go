package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testSigningKey = []byte("test-secret-key-that-is-32-bytes-long!")

func init() {
	SetSigningKey(testSigningKey)
}

func TestMintAndVerify(t *testing.T) {
	token, err := MintToken("test-instance")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	sub, err := VerifyToken(token)
	require.NoError(t, err)
	assert.Equal(t, "test-instance", sub)
}

func TestExpiredToken(t *testing.T) {
	now := time.Now()
	claims := AgentClaims{
		jwt.RegisteredClaims{
			Subject:   "test-instance",
			Issuer:    "controller",
			IssuedAt:  jwt.NewNumericDate(now.Add(-48 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(testSigningKey)
	require.NoError(t, err)

	_, err = VerifyToken(tokenStr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestWrongInstanceToken(t *testing.T) {
	token, err := MintToken("instance-a")
	require.NoError(t, err)

	sub, err := VerifyToken(token)
	require.NoError(t, err)
	assert.Equal(t, "instance-a", sub)
	assert.NotEqual(t, "instance-b", sub)
}

func TestTamperedToken(t *testing.T) {
	token, err := MintToken("test-instance")
	require.NoError(t, err)

	tampered := token[:len(token)-4] + "xxxx"
	_, err = VerifyToken(tampered)
	assert.Error(t, err)
}

func TestInvalidSigningMethod(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS384, AgentClaims{
		jwt.RegisteredClaims{
			Subject: "test-instance",
		},
	})
	tokenStr, err := token.SignedString(testSigningKey)
	require.NoError(t, err)

	_, err = VerifyToken(tokenStr)
	assert.Error(t, err)
}

func TestDifferentSigningKey(t *testing.T) {
	orig := jwtSigningKey

	otherKey := []byte("different-key-that-is-also-32-bytes-total!")
	SetSigningKey(otherKey)
	token, err := MintToken("test-instance")
	require.NoError(t, err)

	SetSigningKey(orig)
	_, err = VerifyToken(token)
	assert.Error(t, err)
}

func TestAgentAuthMiddleware_ValidToken(t *testing.T) {
	token, err := MintToken("my-instance")
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/agent/my-instance/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"instance": "my-instance"})

	w := httptest.NewRecorder()
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	AgentAuthMiddleware(next).ServeHTTP(w, req)
	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentAuthMiddleware_MissingToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/agent/my-instance/status", nil)
	req = mux.SetURLVars(req, map[string]string{"instance": "my-instance"})

	w := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	AgentAuthMiddleware(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentAuthMiddleware_WrongInstance(t *testing.T) {
	token, err := MintToken("instance-a")
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/agent/instance-b/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"instance": "instance-b"})

	w := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	AgentAuthMiddleware(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAgentAuthMiddleware_ExpiredToken(t *testing.T) {
	now := time.Now()
	claims := AgentClaims{
		jwt.RegisteredClaims{
			Subject:   "my-instance",
			Issuer:    "controller",
			IssuedAt:  jwt.NewNumericDate(now.Add(-48 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(testSigningKey)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/agent/my-instance/status", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	req = mux.SetURLVars(req, map[string]string{"instance": "my-instance"})

	w := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called")
	})

	AgentAuthMiddleware(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func setupHandlerTest(t *testing.T) (*testutil.MockDB, *mux.Router) {
	t.Helper()

	mockDB := new(testutil.MockDB)
	GetDBConnection = func(ctx context.Context) (db.DB, config.Config, error) {
		return mockDB, config.Config{}, nil
	}
	StopInstance = func(ctx context.Context, project, zone, instance string) error {
		return nil
	}

	r := mux.NewRouter()
	agentRouter := r.PathPrefix("/agent").Subrouter()
	agentRouter.Use(AgentAuthMiddleware)
	RegisterRoutes(agentRouter)

	return mockDB, r
}

func TestHandleGetStatus(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	expectedStatus := db.Status{
		Players:   db.Players{Current: 1, Max: 20},
		ServerState: db.ServerStateRunning,
	}
	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(expectedStatus, nil)

	req := httptest.NewRequest("GET", "/agent/test-instance/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp db.Status
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, db.ServerStateRunning, resp.ServerState)
	mockDB.AssertExpectations(t)
}

func TestHandleUpdateStatus(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	status := db.Status{ServerState: db.ServerStateRunning}
	body, _ := json.Marshal(status)

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", status).Return(nil)

	req := httptest.NewRequest("PUT", "/agent/test-instance/status", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockDB.AssertExpectations(t)
}

func TestHandleGetWhitelistEntries(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	entries := []db.WhitelistEntry{
		{Username: "player1", UUID: "uuid-1"},
	}
	mockDB.On("GetWhitelistEntries", mock.Anything, "test-instance").Return(entries, nil)

	req := httptest.NewRequest("GET", "/agent/test-instance/whitelist", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []db.WhitelistEntry
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Len(t, resp, 1)
	mockDB.AssertExpectations(t)
}

func TestHandleGetWhitelistConfig(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	cfg := db.WhitelistConfig{Enabled: true}
	mockDB.On("GetWhitelistConfig", mock.Anything, "test-instance").Return(cfg, nil)

	req := httptest.NewRequest("GET", "/agent/test-instance/whitelist/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp db.WhitelistConfig
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.Enabled)
	mockDB.AssertExpectations(t)
}

func TestHandleSetWhitelistConfig(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	cfg := db.WhitelistConfig{Enabled: false}
	body, _ := json.Marshal(cfg)

	mockDB.On("SetWhitelistConfig", mock.Anything, "test-instance", cfg).Return(nil)

	req := httptest.NewRequest("PUT", "/agent/test-instance/whitelist/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockDB.AssertExpectations(t)
}

func TestHandleAddWhitelistEntry(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	entry := db.WhitelistEntry{Username: "newplayer", UUID: "uuid-new"}
	body, _ := json.Marshal(entry)

	mockDB.On("AddWhitelistEntry", mock.Anything, "test-instance", entry).Return(nil)

	req := httptest.NewRequest("POST", "/agent/test-instance/whitelist", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockDB.AssertExpectations(t)
}

func TestHandleStop(t *testing.T) {
	mockDB, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	stopCalled := false
	StopInstance = func(ctx context.Context, project, zone, instance string) error {
		stopCalled = true
		assert.Equal(t, "my-project", project)
		assert.Equal(t, "my-zone", zone)
		assert.Equal(t, "test-instance", instance)
		return nil
	}

	body := map[string]string{"project": "my-project", "zone": "my-zone"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/agent/test-instance/stop", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, stopCalled)
	mockDB.AssertExpectations(t)
}

func TestHandleStop_MissingFields(t *testing.T) {
	_, router := setupHandlerTest(t)
	token, _ := MintToken("test-instance")

	body := map[string]string{"project": "", "zone": ""}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/agent/test-instance/stop", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
