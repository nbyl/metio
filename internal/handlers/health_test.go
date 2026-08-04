package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthHandler_Healthy(t *testing.T) {
	original := daprHealthCheck
	daprHealthCheck = func() bool { return true }
	defer func() { daprHealthCheck = original }()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"healthy"}`, rec.Body.String())
}

func TestHealthHandler_Unhealthy(t *testing.T) {
	original := daprHealthCheck
	daprHealthCheck = func() bool { return false }
	defer func() { daprHealthCheck = original }()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"status":"unhealthy"}`, rec.Body.String())
}
