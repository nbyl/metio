package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMojangClient creates a MojangClient pointing at test servers
func newTestMojangClient(mojangURL, playerDBURL string) *MojangClient {
	return &MojangClient{
		httpClient:      http.DefaultClient,
		baseURL:         mojangURL,
		playerDBBaseURL: playerDBURL,
	}
}

func TestLookupUser_MojangReturnsProfile(t *testing.T) {
	// Mojang returns 200 with profile — no fallback needed
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/users/profiles/minecraft/Notch", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MojangProfile{
			ID:   "069a79f444e94726a5befca90e38aaf5",
			Name: "Notch",
		})
	}))
	defer mojangServer.Close()

	playerDBCalled := false
	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerDBCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "069a79f444e94726a5befca90e38aaf5", profile.ID)
	assert.Equal(t, "Notch", profile.Name)
	assert.False(t, playerDBCalled, "PlayerDB should not be called when Mojang succeeds")
}

func TestLookupUser_MojangReturns204_PlayerDBFallback(t *testing.T) {
	// Mojang returns 204 (No Content) — should fall back to PlayerDB
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/player/minecraft/boboGHG", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "c8d57769-3fe2-4010-9872-ccd44ba40903",
					Username: "boboGHG",
					RawID:    "c8d577693fe240109872ccd44ba40903",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "boboGHG")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "c8d577693fe240109872ccd44ba40903", profile.ID)
	assert.Equal(t, "boboGHG", profile.Name)
}

func TestLookupUser_MojangReturns404_PlayerDBFallback(t *testing.T) {
	// Mojang returns 404 — should fall back to PlayerDB
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "c8d57769-3fe2-4010-9872-ccd44ba40903",
					Username: "boboGHG",
					RawID:    "c8d577693fe240109872ccd44ba40903",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "boboGHG")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "c8d577693fe240109872ccd44ba40903", profile.ID)
	assert.Equal(t, "boboGHG", profile.Name)
}

func TestLookupUser_MojangReturns404_PlayerDBReturns400(t *testing.T) {
	// Mojang returns 404 — should fall back to PlayerDB, which now returns 400 (not 404)
	// for unknown/invalid usernames (e.g. "minecraft.invalid_username") — treat as not found
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "minecraft.invalid_username",
			Message: "Failed to get player data.",
			Success: false,
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "nonexistentuser12345")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_MojangReturns204_PlayerDBReturns400(t *testing.T) {
	// Mojang returns 204 (migrated account) — should fall back to PlayerDB, which now
	// returns 400 (not 404) for not-found — treat as not found
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "minecraft.invalid_username",
			Message: "Failed to get player data.",
			Success: false,
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "nonexistentuser12345")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_MojangReturns204_PlayerDBReturns400_ValidPlayer(t *testing.T) {
	// The regression: PlayerDB now returns 400 (not 404) for a migrated account like
	// boboGHG. Prior behavior treated 400 as a hard error, which LookupUser swallowed
	// into "not found" — rejecting valid players. PlayerDB may return 200 + success:true
	// for the same username when resolved; this test verifies the 400 not-found path
	// is handled without producing a hard error, and the resolved path still works.
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBRequests := 0
	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerDBRequests++
		if playerDBRequests == 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(playerDBResponse{
				Code:    "minecraft.invalid_username",
				Message: "Failed to get player data.",
				Success: false,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "c8d57769-3fe2-4010-9872-ccd44ba40903",
					Username: "boboGHG",
					RawID:    "c8d577693fe240109872ccd44ba40903",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)

	// First lookup: 400 + success:false → not found, no hard error
	profile, err := client.LookupUser(context.Background(), "boboGHG")
	require.NoError(t, err)
	assert.Nil(t, profile)

	// Second lookup: PlayerDB resolves the player
	profile, err = client.LookupUser(context.Background(), "boboGHG")
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "c8d577693fe240109872ccd44ba40903", profile.ID)
	assert.Equal(t, "boboGHG", profile.Name)
}

func TestLookupUser_BothAPIsReturnNotFound(t *testing.T) {
	// Both Mojang and PlayerDB return not found — should return nil
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "nonexistentuser12345")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_MojangRateLimited_PlayerDBFallback(t *testing.T) {
	// Mojang returns 429 — should fall back to PlayerDB
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "069a79f4-44e9-4726-a5be-fca90e38aaf5",
					Username: "Notch",
					RawID:    "069a79f444e94726a5befca90e38aaf5",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "069a79f444e94726a5befca90e38aaf5", profile.ID)
	assert.Equal(t, "Notch", profile.Name)
}

func TestLookupUser_MojangServerError_PlayerDBFallback(t *testing.T) {
	// Mojang returns 500 — should fall back to PlayerDB
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "069a79f4-44e9-4726-a5be-fca90e38aaf5",
					Username: "Notch",
					RawID:    "069a79f444e94726a5befca90e38aaf5",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "069a79f444e94726a5befca90e38aaf5", profile.ID)
	assert.Equal(t, "Notch", profile.Name)
}

func TestLookupUser_MojangError_PlayerDBNotFound_ReturnsNil(t *testing.T) {
	// Mojang errors but PlayerDB gives a clean "not found" — that is authoritative,
	// so LookupUser returns nil (not found) rather than the Mojang error
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "minecraft.invalid_username",
			Message: "Failed to get player data.",
			Success: false,
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_MojangError_PlayerDBError_ReturnsMojangError(t *testing.T) {
	// Both Mojang and PlayerDB fail — return the original Mojang error so the caller
	// can distinguish an outage from a not-found
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	assert.Nil(t, profile)
}

func TestLookupUser_PlayerDBReturnsSuccessFalse(t *testing.T) {
	// Mojang returns 204, PlayerDB returns success: false — should return nil
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "minecraft.api_failure",
			Message: "Failed to get player data.",
			Success: false,
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "baduser")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_PlayerDBServerError_ReturnsNil(t *testing.T) {
	// Mojang returns 204, PlayerDB returns 500 — should return nil (not error)
	// because the primary lookup already said "not found", fallback errors are suppressed
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "someuser")

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func TestLookupUser_PlayerDBFallbackUsesUUIDWithDashes(t *testing.T) {
	// When PlayerDB returns only the dashed UUID (no raw_id), it should strip dashes
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mojangServer.Close()

	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "069a79f4-44e9-4726-a5be-fca90e38aaf5",
					Username: "Notch",
					RawID:    "", // empty raw_id — should fall back to stripping dashes from ID
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient(mojangServer.URL, playerDBServer.URL)
	profile, err := client.LookupUser(context.Background(), "Notch")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "069a79f444e94726a5befca90e38aaf5", profile.ID)
	assert.Equal(t, "Notch", profile.Name)
}

func TestFormatUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid 32-char UUID",
			input:    "069a79f444e94726a5befca90e38aaf5",
			expected: "069a79f4-44e9-4726-a5be-fca90e38aaf5",
		},
		{
			name:     "already formatted UUID (returned as-is)",
			input:    "069a79f4-44e9-4726-a5be-fca90e38aaf5",
			expected: "069a79f4-44e9-4726-a5be-fca90e38aaf5",
		},
		{
			name:     "short string (returned as-is)",
			input:    "abc",
			expected: "abc",
		},
		{
			name:     "empty string (returned as-is)",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatUUID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLookupMojang_DirectCall(t *testing.T) {
	// Test the internal lookupMojang method directly
	mojangServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MojangProfile{
			ID:   "069a79f444e94726a5befca90e38aaf5",
			Name: "Notch",
		})
	}))
	defer mojangServer.Close()

	client := newTestMojangClient(mojangServer.URL, "")
	profile, err := client.lookupMojang(context.Background(), "Notch")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "Notch", profile.Name)
}

func TestLookupPlayerDB_DirectCall(t *testing.T) {
	// Test the internal lookupPlayerDB method directly
	playerDBServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "metio/1.0", r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playerDBResponse{
			Code:    "player.found",
			Message: "Successfully found player by given ID.",
			Success: true,
			Data: struct {
				Player struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				} `json:"player"`
			}{
				Player: struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					RawID    string `json:"raw_id"`
				}{
					ID:       "c8d57769-3fe2-4010-9872-ccd44ba40903",
					Username: "boboGHG",
					RawID:    "c8d577693fe240109872ccd44ba40903",
				},
			},
		})
	}))
	defer playerDBServer.Close()

	client := newTestMojangClient("", playerDBServer.URL)
	profile, err := client.lookupPlayerDB(context.Background(), "boboGHG")

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "c8d577693fe240109872ccd44ba40903", profile.ID)
	assert.Equal(t, "boboGHG", profile.Name)
}
