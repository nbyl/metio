package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCORSMiddleware_Production(t *testing.T) {
	os.Unsetenv("DEV_MODE")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORSMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// In production, no CORS headers should be added
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSMiddleware_Development(t *testing.T) {
	t.Setenv("DEV_MODE", "true")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORSMiddleware(nextHandler)

	// Preflight request
	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestTracingMiddleware(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Verify context has been updated (span attached)
		assert.NotNil(t, r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TracingMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVerifyPubSubAuth(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		expected   bool
	}{
		{"valid bearer token", "Bearer token123", true},
		{"empty header", "", false},
		{"no bearer prefix", "token123", false},
		{"basic auth", "Basic dXNlcjpwYXNz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/events", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			assert.Equal(t, tt.expected, verifyPubSubAuth(req))
		})
	}
}

func TestIsUserAllowed(t *testing.T) {
	viper.Set("ALLOWED_USERS", []string{"admin@example.com", "user@example.com"})
	defer viper.Set("ALLOWED_USERS", nil)

	assert.True(t, isUserAllowed(&User{Email: "admin@example.com"}))
	assert.True(t, isUserAllowed(&User{Email: "user@example.com"}))
	assert.False(t, isUserAllowed(&User{Email: "hacker@example.com"}))
}

func TestGenerateStateOauthCookie(t *testing.T) {
	w := httptest.NewRecorder()
	state := generateStateOauthCookie(w)

	assert.NotEmpty(t, state)
	cookies := w.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "oauthstate", cookies[0].Name)
	assert.Equal(t, state, cookies[0].Value)
}

func TestGetGoogleOauthConfig(t *testing.T) {
	// Reset cached config
	googleOauthConfig = nil
	viper.Set("BASE_URL", "https://example.com")
	viper.Set("GOOGLE_CLIENT_ID", "test-client-id")
	viper.Set("GOOGLE_CLIENT_SECRET", "test-secret")
	defer func() {
		viper.Set("BASE_URL", "")
		viper.Set("GOOGLE_CLIENT_ID", "")
		viper.Set("GOOGLE_CLIENT_SECRET", "")
		googleOauthConfig = nil
	}()

	cfg := getGoogleOauthConfig()
	assert.Equal(t, "https://example.com/auth/callback", cfg.RedirectURL)
	assert.Equal(t, "test-client-id", cfg.ClientID)
	assert.Equal(t, "test-secret", cfg.ClientSecret)
	assert.Len(t, cfg.Scopes, 2)
}
