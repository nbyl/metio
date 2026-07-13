package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_FirestoreBackend(t *testing.T) {
	original := requireDaprCheck
	requireDaprCheck = func() bool { return false }
	defer func() { requireDaprCheck = original }()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"healthy"}`, rec.Body.String())
}

func TestHealthHandler_DaprBackend_Healthy(t *testing.T) {
	originalCheck := daprHealthCheck
	originalRequire := requireDaprCheck
	daprHealthCheck = func() bool { return true }
	requireDaprCheck = func() bool { return true }
	defer func() {
		daprHealthCheck = originalCheck
		requireDaprCheck = originalRequire
	}()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"healthy"}`, rec.Body.String())
}

func TestHealthHandler_DaprBackend_Unhealthy(t *testing.T) {
	originalCheck := daprHealthCheck
	originalRequire := requireDaprCheck
	daprHealthCheck = func() bool { return false }
	requireDaprCheck = func() bool { return true }
	defer func() {
		daprHealthCheck = originalCheck
		requireDaprCheck = originalRequire
	}()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"status":"unhealthy"}`, rec.Body.String())
}
