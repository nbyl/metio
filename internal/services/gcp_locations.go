package services

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"

	"github.com/nbyl/metio/internal/db"
)

const (
	// locationsCacheTTL bounds how often the Compute API is contacted. GCP
	// adds or deprecates regions/zones infrequently, so a long TTL keeps
	// outbound traffic negligible.
	locationsCacheTTL = 6 * time.Hour

	// locationsFetchTimeout bounds a single zones.list call.
	locationsFetchTimeout = 10 * time.Second
)

// GCPLocation groups the zones of a single GCP region, e.g. region
// "us-central1" with zones ["us-central1-a", "us-central1-b", ...].
type GCPLocation struct {
	ID    string
	Zones []string
}

// locationsCache holds the last successfully fetched region→zones mapping.
// Like versionCache it also exposes stale entries, so an upstream outage can
// be served from the previous result rather than falling all the way back to
// the hardcoded list.
type locationsCache struct {
	mu        sync.RWMutex
	locations []GCPLocation
	expiresAt time.Time
	ttl       time.Duration
}

// get returns the cached locations along with whether they are still fresh.
// A stale entry is still returned so callers can use it as a fallback.
func (c *locationsCache) get() ([]GCPLocation, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.locations == nil {
		return nil, false
	}
	return c.locations, time.Now().Before(c.expiresAt)
}

func (c *locationsCache) set(locations []GCPLocation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.locations = locations
	c.expiresAt = time.Now().Add(c.ttl)
}

// zonesFetcher retrieves the zones of a project from the Compute API. It is a
// function so tests can inject a fake instead of a live client.
type zonesFetcher func(ctx context.Context, projectID string) ([]*computepb.Zone, error)

// LocationsService serves the region→zones mapping offered to users, backed
// by the Compute API and cached in memory.
type LocationsService struct {
	fetchZones zonesFetcher

	cache *locationsCache

	// fetchMu serialises refreshes so concurrent callers do not stampede
	// the Compute API when the cache expires.
	fetchMu sync.Mutex

	// fallback is used when the Compute API cannot be reached and nothing has
	// ever been cached.
	fallback []GCPLocation
}

// NewLocationsService creates a service backed by the Compute API.
func NewLocationsService() *LocationsService {
	return &LocationsService{
		fetchZones: fetchZonesFromCompute,
		cache:      &locationsCache{ttl: locationsCacheTTL},
		fallback:   fallbackLocations(),
	}
}

// List returns the regions and their zones to offer, sorted by region.
//
// It never returns an error: callers such as GET /api/options must not fail
// because the Compute API is unavailable. On failure the last cached mapping
// is served if one exists, otherwise the built-in fallback list.
func (s *LocationsService) List(ctx context.Context, projectID string) []GCPLocation {
	if locations, fresh := s.cache.get(); fresh {
		return locations
	}

	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	// Another goroutine may have refreshed the cache while we waited.
	if locations, fresh := s.cache.get(); fresh {
		return locations
	}

	locations, err := s.fetch(ctx, projectID)
	if err != nil {
		if stale, _ := s.cache.get(); len(stale) > 0 {
			log.Printf("Failed to refresh GCP zones and regions from the Compute API (project %s), serving cached list: %v", projectID, err)
			return stale
		}
		log.Printf("Failed to fetch GCP zones and regions from the Compute API (project %s), using fallback list: %v", projectID, err)
		return s.fallback
	}

	log.Printf("Fetched %d GCP regions from the Compute API", len(locations))
	s.cache.set(locations)
	return locations
}

// fetch retrieves the project's zones from the Compute API and groups them by
// region. A result with no usable zones is treated as an error so callers fall
// back instead of offering an empty list.
func (s *LocationsService) fetch(ctx context.Context, projectID string) ([]GCPLocation, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, locationsFetchTimeout)
	defer cancel()

	zones, err := s.fetchZones(fetchCtx, projectID)
	if err != nil {
		return nil, err
	}

	locations := groupZones(zones)
	if len(locations) == 0 {
		return nil, fmt.Errorf("compute API returned no usable zones for project %s", projectID)
	}
	return locations, nil
}

// fetchZonesFromCompute lists every zone in the project via the Compute API.
func fetchZonesFromCompute(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
	client, err := compute.NewZonesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute zones client: %w", err)
	}
	defer client.Close()

	it := client.List(ctx, &computepb.ListZonesRequest{Project: projectID})
	var zones []*computepb.Zone
	for {
		zone, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list compute zones: %w", err)
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

// groupZones groups zone names by the region they belong to, dropping zones
// whose region URL does not resolve to a region ID, and returns the regions
// and their zones sorted.
func groupZones(zones []*computepb.Zone) []GCPLocation {
	byRegion := make(map[string][]string)
	for _, zone := range zones {
		if zone == nil || zone.Region == nil || zone.Name == nil {
			continue
		}
		region := path.Base(*zone.Region)
		if region == "" || region == "." || region == "/" {
			continue
		}
		byRegion[region] = append(byRegion[region], *zone.Name)
	}

	locations := make([]GCPLocation, 0, len(byRegion))
	for region, zoneNames := range byRegion {
		sort.Strings(zoneNames)
		locations = append(locations, GCPLocation{ID: region, Zones: zoneNames})
	}
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].ID < locations[j].ID
	})
	return locations
}

// fallbackLocations builds the region→zones mapping from the hardcoded list
// that was served before the Compute API integration.
func fallbackLocations() []GCPLocation {
	regionIDs := db.ListRegions()
	locations := make([]GCPLocation, 0, len(regionIDs))
	for _, regionID := range regionIDs {
		zones := db.ListZonesByRegion(regionID)
		sort.Strings(zones)
		locations = append(locations, GCPLocation{ID: regionID, Zones: zones})
	}
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].ID < locations[j].ID
	})
	return locations
}

// Default service for convenience.
var defaultLocationsService = NewLocationsService()

// ListGCPLocations returns the regions and their zones to offer, using the
// default service.
var ListGCPLocations = func(ctx context.Context, projectID string) []GCPLocation {
	return defaultLocationsService.List(ctx, projectID)
}
