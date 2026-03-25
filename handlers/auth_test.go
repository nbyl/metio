package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Initialize viper with test values
	viper.SetDefault("SESSION_KEY", "test-session-key-32-bytes-long!!")
}

// setupTestSession creates a session store for testing
func setupTestSession() *sessions.CookieStore {
	store = sessions.NewCookieStore([]byte("test-session-key-32-bytes-long!!"))
	return store
}

func TestApiAuthMiddleware_Unauthenticated(t *testing.T) {
	setupTestSession()

	// Create a test handler that should not be reached
	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with apiAuthMiddleware
	handler := apiAuthMiddleware(nextHandler)

	// Make request without session
	req := httptest.NewRequest("GET", "/api/server/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 401 with JSON
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse JSON response
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "unauthorized", response["error"])

	// Next handler should not have been called
	assert.False(t, nextHandlerCalled)
}

func TestApiAuthMiddleware_Authenticated(t *testing.T) {
	testStore := setupTestSession()

	// Create a test handler that should be called
	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap with apiAuthMiddleware
	handler := apiAuthMiddleware(nextHandler)

	// First, create a session and get the cookie
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupW := httptest.NewRecorder()
	session, _ := testStore.Get(setupReq, sessionName)
	session.Values[userKey] = "test-user-id"
	session.Values[emailKey] = "test@example.com"
	session.Save(setupReq, setupW)

	// Now create the actual request with the session cookie
	req := httptest.NewRequest("GET", "/api/server/status", nil)
	for _, cookie := range setupW.Result().Cookies() {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should return 200 and call the next handler
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, nextHandlerCalled)
	assert.Equal(t, "success", w.Body.String())
}

func TestAuthMiddleware_Unauthenticated_Redirects(t *testing.T) {
	setupTestSession()

	// Create a test handler that should not be reached
	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with authMiddleware (not apiAuthMiddleware)
	handler := authMiddleware(nextHandler)

	// Make request without session
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should redirect to login
	assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))

	// Next handler should not have been called
	assert.False(t, nextHandlerCalled)
}

func TestIsUserAuthenticated_NoSession(t *testing.T) {
	setupTestSession()

	req := httptest.NewRequest("GET", "/", nil)
	assert.False(t, isUserAuthenticated(req))
}

func TestIsUserAuthenticated_WithSession(t *testing.T) {
	testStore := setupTestSession()

	// First, create and save a session
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupW := httptest.NewRecorder()
	session, _ := testStore.Get(setupReq, sessionName)
	session.Values[userKey] = "test-user-id"
	session.Save(setupReq, setupW)

	// Now create the actual request with the session cookie
	req := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range setupW.Result().Cookies() {
		req.AddCookie(cookie)
	}

	assert.True(t, isUserAuthenticated(req))
}

func TestMeHandler_Authenticated(t *testing.T) {
	testStore := setupTestSession()

	// First, create and save a session
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupW := httptest.NewRecorder()
	session, _ := testStore.Get(setupReq, sessionName)
	session.Values[userKey] = "test-user-id"
	session.Values[emailKey] = "test@example.com"
	session.Save(setupReq, setupW)

	// Now create the actual request with the session cookie
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	for _, cookie := range setupW.Result().Cookies() {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()

	meHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response AuthMeResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Authenticated)
	assert.Equal(t, "test@example.com", response.Email)
}

func TestMeHandler_Unauthenticated(t *testing.T) {
	setupTestSession()

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	w := httptest.NewRecorder()

	meHandler(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response AuthMeResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Authenticated)
	assert.Empty(t, response.Email)
}
