package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nbyl/metio/internal/db"
)

const (
	// versionManifestURL is Mojang's authoritative list of published versions.
	versionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

	// versionCacheTTL bounds how often Mojang is contacted. Releases are
	// infrequent, so a long TTL keeps outbound traffic negligible.
	versionCacheTTL = 6 * time.Hour

	// releaseVersionType is the only manifest entry type offered to users.
	// Snapshots, old_alpha and old_beta are excluded.
	releaseVersionType = "release"
)

// versionManifest is the subset of Mojang's manifest that we consume.
type versionManifest struct {
	Versions []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"versions"`
}

// versionCache holds the last successfully fetched version list. Unlike a plain
// TTL cache it also exposes stale entries, so an upstream outage can be served
// from the previous result rather than falling all the way back to the
// hardcoded list.
type versionCache struct {
	mu        sync.RWMutex
	versions  []string
	expiresAt time.Time
	ttl       time.Duration
}

// get returns the cached versions along with whether they are still fresh.
// A stale entry is still returned so callers can use it as a fallback.
func (c *versionCache) get() ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.versions == nil {
		return nil, false
	}
	return c.versions, time.Now().Before(c.expiresAt)
}

func (c *versionCache) set(versions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.versions = versions
	c.expiresAt = time.Now().Add(c.ttl)
}

// MinecraftVersionService serves the list of Minecraft versions offered to
// users, backed by Mojang's version manifest and cached in memory.
type MinecraftVersionService struct {
	httpClient  *http.Client
	manifestURL string

	cache *versionCache

	// fetchMu serialises refreshes so concurrent callers do not stampede
	// Mojang when the cache expires.
	fetchMu sync.Mutex

	// fallback is used when Mojang cannot be reached and nothing has ever
	// been cached.
	fallback []string
}

// NewMinecraftVersionService creates a service using Mojang's manifest.
func NewMinecraftVersionService() *MinecraftVersionService {
	return &MinecraftVersionService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		manifestURL: versionManifestURL,
		cache:       &versionCache{ttl: versionCacheTTL},
		fallback:    db.MinecraftVersions,
	}
}

// List returns the Minecraft versions to offer, newest first.
//
// It never returns an error: callers such as GET /api/options must not fail
// because Mojang is unavailable. On failure the last cached list is served if
// one exists, otherwise the built-in fallback list.
func (s *MinecraftVersionService) List(ctx context.Context) []string {
	if versions, fresh := s.cache.get(); fresh {
		return versions
	}

	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	// Another goroutine may have refreshed the cache while we waited.
	if versions, fresh := s.cache.get(); fresh {
		return versions
	}

	versions, err := s.fetch(ctx)
	if err != nil {
		if stale, _ := s.cache.get(); len(stale) > 0 {
			log.Printf("Failed to refresh Minecraft versions from Mojang (%s), serving cached list: %v", s.manifestURL, err)
			return stale
		}
		log.Printf("Failed to fetch Minecraft versions from Mojang (%s), using fallback list: %v", s.manifestURL, err)
		return s.fallback
	}

	log.Printf("Fetched %d Minecraft versions from Mojang", len(versions))
	s.cache.set(versions)
	return versions
}

// fetch retrieves the manifest and returns the release versions in the order
// Mojang lists them, which is newest first.
func (s *MinecraftVersionService) fetch(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", s.manifestURL, err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", s.manifestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from Mojang manifest: %d (url %s)", resp.StatusCode, s.manifestURL)
	}

	var manifest versionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest from %s: %w", s.manifestURL, err)
	}

	versions := make([]string, 0, len(manifest.Versions))
	for _, v := range manifest.Versions {
		if v.Type == releaseVersionType {
			versions = append(versions, v.ID)
		}
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("manifest contained no release versions")
	}

	return versions, nil
}

// Default service for convenience.
var defaultMinecraftVersionService = NewMinecraftVersionService()

// ListMinecraftVersions returns the Minecraft versions to offer, using the
// default service.
var ListMinecraftVersions = func(ctx context.Context) []string {
	return defaultMinecraftVersionService.List(ctx)
}
