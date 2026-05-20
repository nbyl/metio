package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// MojangProfile represents a Minecraft player profile from the Mojang API
type MojangProfile struct {
	ID   string `json:"id"`   // UUID without dashes
	Name string `json:"name"` // Actual username (case-corrected)
}

// playerDBResponse represents the response from the PlayerDB API
type playerDBResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Player struct {
			ID       string `json:"id"`       // UUID with dashes
			Username string `json:"username"` // Canonical username
			RawID    string `json:"raw_id"`   // UUID without dashes
		} `json:"player"`
	} `json:"data"`
	Success bool `json:"success"`
}

// MinecraftUserLookup defines the interface for looking up Minecraft users
type MinecraftUserLookup interface {
	LookupUser(ctx context.Context, username string) (*MojangProfile, error)
}

// MojangClient handles communication with the Mojang API with a PlayerDB fallback
type MojangClient struct {
	httpClient      *http.Client
	baseURL         string
	playerDBBaseURL string
}

// NewMojangClient creates a new Mojang API client with PlayerDB fallback
func NewMojangClient() *MojangClient {
	return &MojangClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:         "https://api.mojang.com",
		playerDBBaseURL: "https://playerdb.co",
	}
}

// LookupUser looks up a Minecraft user by username.
// It first tries the Mojang API, then falls back to PlayerDB if the user is not found.
// Returns the profile if found, nil if not found, or an error on API failure.
func (c *MojangClient) LookupUser(ctx context.Context, username string) (*MojangProfile, error) {
	// Try Mojang API first
	profile, err := c.lookupMojang(ctx, username)
	if err != nil {
		// On hard errors (network, rate limit, etc.), don't fallback
		return nil, err
	}
	if profile != nil {
		return profile, nil
	}

	// Mojang returned "not found" — try PlayerDB as fallback
	log.Printf("Mojang API did not find user %q, trying PlayerDB fallback", username)
	profile, err = c.lookupPlayerDB(ctx, username)
	if err != nil {
		log.Printf("PlayerDB fallback also failed for user %q: %v", username, err)
		// Return nil (not found) rather than a fallback error, since Mojang already said "not found"
		return nil, nil
	}
	if profile != nil {
		log.Printf("PlayerDB fallback found user %q as %q (UUID: %s)", username, profile.Name, profile.ID)
	}
	return profile, nil
}

// lookupMojang queries the Mojang API for a Minecraft user profile
func (c *MojangClient) lookupMojang(ctx context.Context, username string) (*MojangProfile, error) {
	url := fmt.Sprintf("%s/users/profiles/minecraft/%s", c.baseURL, username)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var profile MojangProfile
		if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &profile, nil
	case http.StatusNotFound, http.StatusNoContent:
		// User not found via Mojang
		return nil, nil
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("rate limited by Mojang API")
	default:
		return nil, fmt.Errorf("unexpected status code from Mojang API: %d", resp.StatusCode)
	}
}

// lookupPlayerDB queries the PlayerDB API as a fallback for Minecraft user lookups.
// PlayerDB is more reliable for some Microsoft-migrated accounts that the Mojang API
// returns 204 No Content for.
func (c *MojangClient) lookupPlayerDB(ctx context.Context, username string) (*MojangProfile, error) {
	url := fmt.Sprintf("%s/api/player/minecraft/%s", c.playerDBBaseURL, username)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "metio/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to PlayerDB: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("unexpected status code from PlayerDB: %d", resp.StatusCode)
	}

	var pdbResp playerDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&pdbResp); err != nil {
		return nil, fmt.Errorf("failed to decode PlayerDB response: %w", err)
	}

	if !pdbResp.Success {
		return nil, nil
	}

	// Convert PlayerDB response to MojangProfile format
	rawID := pdbResp.Data.Player.RawID
	if rawID == "" {
		// Fall back to stripping dashes from the UUID with dashes
		rawID = strings.ReplaceAll(pdbResp.Data.Player.ID, "-", "")
	}

	return &MojangProfile{
		ID:   rawID,
		Name: pdbResp.Data.Player.Username,
	}, nil
}

// FormatUUID formats a UUID without dashes to include dashes
// e.g., "069a79f444e94726a5befca90e38aaf5" -> "069a79f4-44e9-4726-a5be-fca90e38aaf5"
func FormatUUID(uuid string) string {
	if len(uuid) != 32 {
		return uuid
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		uuid[0:8],
		uuid[8:12],
		uuid[12:16],
		uuid[16:20],
		uuid[20:32],
	)
}

// Default client for convenience
var defaultMojangClient = NewMojangClient()

// LookupMinecraftUser looks up a Minecraft user using the default client
var LookupMinecraftUser = func(ctx context.Context, username string) (*MojangProfile, error) {
	return defaultMojangClient.LookupUser(ctx, username)
}
