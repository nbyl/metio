package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nbyl/metio/internal/dbtypes"
)

type AgentClient interface {
	GetStatus(ctx context.Context) (dbtypes.Status, error)
	UpdateStatus(ctx context.Context, status dbtypes.Status) error
	GetWhitelistEntries(ctx context.Context) ([]dbtypes.WhitelistEntry, error)
	GetWhitelistConfig(ctx context.Context) (dbtypes.WhitelistConfig, error)
	SetWhitelistConfig(ctx context.Context, cfg dbtypes.WhitelistConfig) error
	AddWhitelistEntry(ctx context.Context, entry dbtypes.WhitelistEntry) error
	StopInstance(ctx context.Context, project, zone string) error
}

type client struct {
	baseURL      string
	token        string
	instanceName string
	httpClient   *http.Client
}

func New(baseURL, token, instanceName string) AgentClient {
	return &client{
		baseURL:      baseURL,
		token:        token,
		instanceName: instanceName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *client) GetStatus(ctx context.Context) (dbtypes.Status, error) {
	var status dbtypes.Status
	err := c.get(ctx, fmt.Sprintf("/agent/%s/status", c.instanceName), &status)
	return status, err
}

func (c *client) UpdateStatus(ctx context.Context, status dbtypes.Status) error {
	return c.put(ctx, fmt.Sprintf("/agent/%s/status", c.instanceName), status)
}

func (c *client) GetWhitelistEntries(ctx context.Context) ([]dbtypes.WhitelistEntry, error) {
	var entries []dbtypes.WhitelistEntry
	err := c.get(ctx, fmt.Sprintf("/agent/%s/whitelist", c.instanceName), &entries)
	return entries, err
}

func (c *client) GetWhitelistConfig(ctx context.Context) (dbtypes.WhitelistConfig, error) {
	var cfg dbtypes.WhitelistConfig
	err := c.get(ctx, fmt.Sprintf("/agent/%s/whitelist/config", c.instanceName), &cfg)
	return cfg, err
}

func (c *client) SetWhitelistConfig(ctx context.Context, cfg dbtypes.WhitelistConfig) error {
	return c.put(ctx, fmt.Sprintf("/agent/%s/whitelist/config", c.instanceName), cfg)
}

func (c *client) AddWhitelistEntry(ctx context.Context, entry dbtypes.WhitelistEntry) error {
	return c.post(ctx, fmt.Sprintf("/agent/%s/whitelist", c.instanceName), entry)
}

func (c *client) StopInstance(ctx context.Context, project, zone string) error {
	body := map[string]string{
		"project": project,
		"zone":    zone,
	}
	return c.post(ctx, fmt.Sprintf("/agent/%s/stop", c.instanceName), body)
}

func (c *client) get(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeError(resp.Body)
	}
	if dest != nil {
		return json.NewDecoder(resp.Body).Decode(dest)
	}
	return nil
}

func (c *client) put(ctx context.Context, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeError(resp.Body)
	}
	return nil
}

func (c *client) post(ctx context.Context, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return decodeError(resp.Body)
	}
	return nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func decodeError(r io.Reader) error {
	var errResp errorResponse
	if err := json.NewDecoder(r).Decode(&errResp); err != nil {
		return fmt.Errorf("controller returned status %d", 0)
	}
	return fmt.Errorf("controller: %s", errResp.Error)
}
