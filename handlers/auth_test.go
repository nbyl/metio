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
	viper.SetDefault("ENVIRONMENT", "development")
}

// setupTestSession creates a session store for testing that mirrors
// the production getSessionStore() cookie options.
func setupTestSession() *sessions.CookieStore {
	store = sessions.NewCookieStore([]byte("test-session-key-32-bytes-long!!"))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   false, // development mode for tests
		SameSite: http.SameSiteLaxMode,
	}
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

func TestGetSessionStore_CookieOptions_Development(t *testing.T) {
	// Reset global store so getSessionStore() re-creates it
	store = nil
	viper.Set("ENVIRONMENT", "development")
	defer viper.Set("ENVIRONMENT", "")

	s := getSessionStore()
	assert.Equal(t, "/", s.Options.Path)
	assert.Equal(t, 86400*7, s.Options.MaxAge)
	assert.True(t, s.Options.HttpOnly)
	assert.False(t, s.Options.Secure, "Secure should be false in development")
	assert.Equal(t, http.SameSiteLaxMode, s.Options.SameSite)

	// Reset for other tests
	store = nil
}

func TestGetSessionStore_CookieOptions_Production(t *testing.T) {
	store = nil
	viper.Set("ENVIRONMENT", "production")
	defer viper.Set("ENVIRONMENT", "")

	s := getSessionStore()
	assert.True(t, s.Options.Secure, "Secure should be true in production")
	assert.Equal(t, http.SameSiteLaxMode, s.Options.SameSite)

	// Reset for other tests
	store = nil
}

func TestApiAuthMiddleware_DevApiKey_ValidKey(t *testing.T) {
	setupTestSession()
	viper.Set("DEV_API_KEY", "test-dev-key")
	defer viper.Set("DEV_API_KEY", "")

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := apiAuthMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/servers", nil)
	req.Header.Set("Authorization", "Bearer test-dev-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, nextHandlerCalled)
}

func TestApiAuthMiddleware_DevApiKey_InvalidKey(t *testing.T) {
	setupTestSession()
	viper.Set("DEV_API_KEY", "test-dev-key")
	defer viper.Set("DEV_API_KEY", "")

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := apiAuthMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/servers", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, nextHandlerCalled)
}

func TestGetUserEmail_WithSession(t *testing.T) {
	testStore := setupTestSession()

	// Create a session with email
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupW := httptest.NewRecorder()
	session, _ := testStore.Get(setupReq, sessionName)
	session.Values[userKey] = "test-user-id"
	session.Values[emailKey] = "user@example.com"
	session.Save(setupReq, setupW)

	// Create request with the session cookie
	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	for _, cookie := range setupW.Result().Cookies() {
		req.AddCookie(cookie)
	}

	email := getUserEmail(req)
	assert.Equal(t, "user@example.com", email)
}

func TestGetUserEmail_WithoutSession(t *testing.T) {
	setupTestSession()

	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	email := getUserEmail(req)
	assert.Empty(t, email)
}

func TestGetUserEmail_SessionWithoutEmail(t *testing.T) {
	testStore := setupTestSession()

	// Create a session with user but no email
	setupReq := httptest.NewRequest("GET", "/setup", nil)
	setupW := httptest.NewRecorder()
	session, _ := testStore.Get(setupReq, sessionName)
	session.Values[userKey] = "test-user-id"
	// Intentionally not setting emailKey
	session.Save(setupReq, setupW)

	req := httptest.NewRequest("GET", "/api/server/whitelist", nil)
	for _, cookie := range setupW.Result().Cookies() {
		req.AddCookie(cookie)
	}

	email := getUserEmail(req)
	assert.Empty(t, email)
}
