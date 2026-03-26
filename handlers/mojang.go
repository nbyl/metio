package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MojangProfile represents a Minecraft player profile from the Mojang API
type MojangProfile struct {
	ID   string `json:"id"`   // UUID without dashes
	Name string `json:"name"` // Actual username (case-corrected)
}

// MojangClient handles communication with the Mojang API
type MojangClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewMojangClient creates a new Mojang API client
func NewMojangClient() *MojangClient {
	return &MojangClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://api.mojang.com",
	}
}

// LookupUser looks up a Minecraft user by username
// Returns the profile if found, nil if not found, or an error on API failure
func (c *MojangClient) LookupUser(ctx context.Context, username string) (*MojangProfile, error) {
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
		// User not found
		return nil, nil
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("rate limited by Mojang API")
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
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
func LookupMinecraftUser(ctx context.Context, username string) (*MojangProfile, error) {
	return defaultMojangClient.LookupUser(ctx, username)
}
