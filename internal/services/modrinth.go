package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	// modrinthBaseURL is Modrinth's v2 API. Unlike Mojang it requires no key,
	// so this service has no configuration of its own.
	modrinthBaseURL = "https://api.modrinth.com/v2"

	// modrinthSearchCacheTTL bounds how often Modrinth is contacted for a given
	// query. Search results are more volatile than the version manifest, so the
	// TTL is shorter than versionCacheTTL but still bounds outbound traffic.
	modrinthSearchCacheTTL = 15 * time.Minute

	// modrinthProjectType is the facet that restricts results to modpacks.
	modrinthProjectType = "project_type:modpack"
)

// modrinthLoaders is the set of loader categories recognised in search hits.
// Modrinth tags loads into a project's categories; the intersection is mapped
// onto the normalised Loader field.
var modrinthLoaders = map[string]bool{
	"fabric":   true,
	"forge":    true,
	"quilt":    true,
	"neoforge": true,
}

// searchHit is the subset of a Modrinth search hit that we consume.
type searchHit struct {
	ProjectID  string   `json:"project_id"`
	Slug       string   `json:"slug"`
	Author     string   `json:"author"`
	Title      string   `json:"title"`
	Categories []string `json:"categories"`
	Versions   []string `json:"versions"`
	Downloads  int64    `json:"downloads"`
	IconURL    string   `json:"icon_url"`
}

// searchResponse is the envelope Modrinth returns for GET /search.
type searchResponse struct {
	Hits []searchHit `json:"hits"`
}

// modrinthVersion is the subset of a project version that we consume.
type modrinthVersion struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	Loaders       []string `json:"loaders"`
	GameVersions  []string `json:"game_versions"`
	Featured      bool     `json:"featured"`
}

// ModrinthPack is a modpack as presented to Metio users, normalised from
// Modrinth's search hit shape.
type ModrinthPack struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Author       string   `json:"author"`
	IconURL      string   `json:"iconUrl"`
	Downloads    int64    `json:"downloads"`
	GameVersions []string `json:"gameVersions"`
	Loader       string   `json:"loader"`
}

// ModrinthVersion is a published version of a modpack, normalised from
// Modrinth's version shape.
type ModrinthVersion struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"versionNumber"`
	GameVersions  []string `json:"gameVersions"`
	Loaders       []string `json:"loaders"`
	Latest        bool     `json:"latest"`
}

// modrinthSearchCache holds the last successfully fetched results for a single
// query. Like versionCache it also exposes stale entries, so an upstream outage
// is served from the previous result rather than nothing at all.
type modrinthSearchCache struct {
	mu        sync.RWMutex
	key       string
	hits      []ModrinthPack
	expiresAt time.Time
	ttl       time.Duration
}

// get returns the cached hits for key along with whether they are still fresh.
// A hit for a different key counts as a miss, so a new query is never served
// another query's results. A stale entry is still returned so callers can use
// it as a fallback.
func (c *modrinthSearchCache) get(key string) ([]ModrinthPack, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.key != key || c.hits == nil {
		return nil, false
	}
	return c.hits, time.Now().Before(c.expiresAt)
}

func (c *modrinthSearchCache) set(key string, hits []ModrinthPack) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key = key
	c.hits = hits
	c.expiresAt = time.Now().Add(c.ttl)
}

// ModrinthService searches Modrinth's modpack catalogue and lists the versions
// of a pack, backed by Modrinth's v2 API and cached in memory.
type ModrinthService struct {
	httpClient *http.Client
	baseURL    string

	cache *modrinthSearchCache

	// fetchMu serialises refreshes so concurrent callers do not stampede
	// Modrinth when the cache expires.
	fetchMu sync.Mutex
}

// NewModrinthService creates a service using Modrinth's v2 API.
func NewModrinthService() *ModrinthService {
	return &ModrinthService{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: modrinthBaseURL,
		cache:   &modrinthSearchCache{ttl: modrinthSearchCacheTTL},
	}
}

// Search returns the modpacks matching query, optionally filtered to a single
// Minecraft version, most relevant first.
//
// It never returns an error: callers must not fail because Modrinth is
// unavailable. On failure the last cached results for this query are served if
// one exists, otherwise an empty list.
func (s *ModrinthService) Search(ctx context.Context, query, mcVersion string) []ModrinthPack {
	key := searchCacheKey(query, mcVersion)

	if hits, fresh := s.cache.get(key); fresh {
		return hits
	}

	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	// Another goroutine may have refreshed the cache while we waited.
	if hits, fresh := s.cache.get(key); fresh {
		return hits
	}

	hits, err := s.fetchSearch(ctx, query, mcVersion)
	if err != nil {
		if stale, _ := s.cache.get(key); stale != nil {
			log.Printf("Failed to refresh Modrinth modpack search (%s), serving cached results: %v", s.baseURL, err)
			return stale
		}
		log.Printf("Failed to search Modrinth modpacks (%s), returning no results: %v", s.baseURL, err)
		return []ModrinthPack{}
	}

	log.Printf("Fetched %d Modrinth modpacks for query %q", len(hits), query)
	s.cache.set(key, hits)
	return hits
}

// Versions returns the published versions of the pack identified by projectID.
//
// It never returns an error; on upstream failure an empty list is returned.
func (s *ModrinthService) Versions(ctx context.Context, projectID string) []ModrinthVersion {
	versions, err := s.fetchVersions(ctx, projectID)
	if err != nil {
		log.Printf("Failed to fetch Modrinth versions for project %s (%s), returning no results: %v", projectID, s.baseURL, err)
		return []ModrinthVersion{}
	}
	return versions
}

// searchCacheKey identifies a search result in the cache: the query plus the
// optional Minecraft version filter.
func searchCacheKey(query, mcVersion string) string {
	return query + "|" + mcVersion
}

// fetchSearch retrieves the modpack hits for query. When mcVersion is set it is
// added as a facet so filtering happens upstream rather than client-side.
func (s *ModrinthService) fetchSearch(ctx context.Context, query, mcVersion string) ([]ModrinthPack, error) {
	facets := [][]string{{modrinthProjectType}}
	if mcVersion != "" {
		facets = append(facets, []string{"versions:" + mcVersion})
	}
	facetBytes, err := json.Marshal(facets)
	if err != nil {
		return nil, fmt.Errorf("failed to build search facets: %w", err)
	}

	facetURL, err := url.Parse(s.baseURL + "/search")
	if err != nil {
		return nil, fmt.Errorf("failed to parse search URL: %w", err)
	}
	facetQuery := facetURL.Query()
	facetQuery.Set("query", query)
	facetQuery.Set("facets", string(facetBytes))
	facetURL.RawQuery = facetQuery.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", facetURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", facetURL, err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", facetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from Modrinth search: %d (url %s)", resp.StatusCode, facetURL)
	}

	var search searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&search); err != nil {
		return nil, fmt.Errorf("failed to decode search results from %s: %w", facetURL, err)
	}

	hits := make([]ModrinthPack, 0, len(search.Hits))
	for _, hit := range search.Hits {
		hits = append(hits, normalizeHit(hit))
	}

	return hits, nil
}

// normalizeHit maps a raw Modrinth search hit onto the normalised ModrinthPack
// shape. The loader is derived from the hit's categories.
func normalizeHit(hit searchHit) ModrinthPack {
	versions := hit.Versions
	if versions == nil {
		versions = []string{}
	}
	return ModrinthPack{
		ID:           hit.ProjectID,
		Slug:         hit.Slug,
		Name:         hit.Title,
		Author:       hit.Author,
		IconURL:      hit.IconURL,
		Downloads:    hit.Downloads,
		GameVersions: versions,
		Loader:       loaderOf(hit.Categories),
	}
}

// loaderOf returns the first recognised loader category, or "" if the project
// does not advertise one.
func loaderOf(categories []string) string {
	for _, c := range categories {
		if modrinthLoaders[c] {
			return c
		}
	}
	return ""
}

// fetchVersions retrieves the published versions of projectID. Changelogs are
// skipped because the listing does not need them.
func (s *ModrinthService) fetchVersions(ctx context.Context, projectID string) ([]ModrinthVersion, error) {
	versionURL := s.baseURL + "/project/" + url.PathEscape(projectID) + "/version"

	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", versionURL, err)
	}
	versionQuery := req.URL.Query()
	versionQuery.Set("include_changelog", "false")
	req.URL.RawQuery = versionQuery.Encode()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", versionURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from Modrinth versions: %d (url %s)", resp.StatusCode, versionURL)
	}

	var raw []modrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode versions from %s: %w", versionURL, err)
	}

	versions := make([]ModrinthVersion, 0, len(raw))
	for _, v := range raw {
		versions = append(versions, ModrinthVersion{
			ID:            v.ID,
			Name:          v.Name,
			VersionNumber: v.VersionNumber,
			GameVersions:  v.GameVersions,
			Loaders:       v.Loaders,
			Latest:        v.Featured,
		})
	}

	return versions, nil
}

// Default service for convenience.
var defaultModrinthService = NewModrinthService()

// SearchModrinthPacks returns the modpacks matching query, using the default
// service.
var SearchModrinthPacks = func(ctx context.Context, query, mcVersion string) []ModrinthPack {
	return defaultModrinthService.Search(ctx, query, mcVersion)
}

// ListModrinthPackVersions returns the published versions of projectID, using
// the default service.
var ListModrinthPackVersions = func(ctx context.Context, projectID string) []ModrinthVersion {
	return defaultModrinthService.Versions(ctx, projectID)
}
