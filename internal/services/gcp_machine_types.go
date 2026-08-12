package services

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"

	"github.com/nbyl/metio/internal/db"
)

const (
	// machineTypeCacheTTL bounds how often the Compute API is contacted. GCP
	// adds or deprecates machine types infrequently, so a long TTL keeps
	// outbound traffic negligible.
	machineTypeCacheTTL = 6 * time.Hour

	// machineTypeFetchTimeout bounds a single aggregatedList call.
	machineTypeFetchTimeout = 10 * time.Second
)

// GCPMachineType is a machine type offered by the project, e.g. "e2-small"
// with 2 vCPUs and 2 GB of memory. It carries no pricing: the Compute API
// does not expose it.
type GCPMachineType struct {
	ID       string
	VCPUs    int
	MemoryGB int
}

// machineTypesCache holds the last successfully fetched machine type list.
// Like locationsCache it also exposes stale entries, so an upstream outage can
// be served from the previous result rather than falling all the way back to
// the hardcoded list.
type machineTypesCache struct {
	mu        sync.RWMutex
	types     []GCPMachineType
	expiresAt time.Time
	ttl       time.Duration
}

// get returns the cached machine types along with whether they are still
// fresh. A stale entry is still returned so callers can use it as a fallback.
func (c *machineTypesCache) get() ([]GCPMachineType, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.types == nil {
		return nil, false
	}
	return c.types, time.Now().Before(c.expiresAt)
}

func (c *machineTypesCache) set(types []GCPMachineType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.types = types
	c.expiresAt = time.Now().Add(c.ttl)
}

// machineTypesFetcher retrieves the machine types of a project from the
// Compute API. It is a function so tests can inject a fake instead of a live
// client.
type machineTypesFetcher func(ctx context.Context, projectID string) ([]*computepb.MachineType, error)

// MachineTypesService serves the machine types offered to users, backed by
// the Compute API and cached in memory.
type MachineTypesService struct {
	fetchTypes machineTypesFetcher

	cache *machineTypesCache

	// fetchMu serialises refreshes so concurrent callers do not stampede
	// the Compute API when the cache expires.
	fetchMu sync.Mutex

	// fallback is used when the Compute API cannot be reached and nothing has
	// ever been cached.
	fallback []GCPMachineType
}

// NewMachineTypesService creates a service backed by the Compute API.
func NewMachineTypesService() *MachineTypesService {
	return &MachineTypesService{
		fetchTypes: fetchMachineTypesFromCompute,
		cache:      &machineTypesCache{ttl: machineTypeCacheTTL},
		fallback:   fallbackMachineTypes(),
	}
}

// List returns the machine types to offer, sorted by ID.
//
// It never returns an error: callers such as GET /api/options must not fail
// because the Compute API is unavailable. On failure the last cached list is
// served if one exists, otherwise the built-in fallback list.
func (s *MachineTypesService) List(ctx context.Context, projectID string) []GCPMachineType {
	if types, fresh := s.cache.get(); fresh {
		return types
	}

	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()

	// Another goroutine may have refreshed the cache while we waited.
	if types, fresh := s.cache.get(); fresh {
		return types
	}

	types, err := s.fetch(ctx, projectID)
	if err != nil {
		if stale, _ := s.cache.get(); len(stale) > 0 {
			log.Printf("Failed to refresh GCP machine types from the Compute API (project %s), serving cached list: %v", projectID, err)
			return stale
		}
		log.Printf("Failed to fetch GCP machine types from the Compute API (project %s), using fallback list: %v", projectID, err)
		return s.fallback
	}

	log.Printf("Fetched %d GCP machine types from the Compute API", len(types))
	s.cache.set(types)
	return types
}

// fetch retrieves the project's machine types from the Compute API,
// deduplicated across zones.
func (s *MachineTypesService) fetch(ctx context.Context, projectID string) ([]GCPMachineType, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, machineTypeFetchTimeout)
	defer cancel()

	raw, err := s.fetchTypes(fetchCtx, projectID)
	if err != nil {
		return nil, err
	}

	types := dedupeMachineTypes(raw)
	if len(types) == 0 {
		return nil, fmt.Errorf("compute API returned no usable machine types for project %s", projectID)
	}
	return types, nil
}

// fetchMachineTypesFromCompute lists the machine types available in any zone
// of the project via the Compute API's aggregatedList.
func fetchMachineTypesFromCompute(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
	client, err := compute.NewMachineTypesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute machine types client: %w", err)
	}
	defer client.Close()

	it := client.AggregatedList(ctx, &computepb.AggregatedListMachineTypesRequest{Project: projectID})
	var types []*computepb.MachineType
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list compute machine types: %w", err)
		}
		types = append(types, pair.Value.GetMachineTypes()...)
	}
	return types, nil
}

// dedupeMachineTypes keeps one entry per machine type name (the same type
// appears in every zone), drops entries without a name, CPUs or memory, and
// returns the types sorted by ID.
func dedupeMachineTypes(types []*computepb.MachineType) []GCPMachineType {
	byID := make(map[string]GCPMachineType)
	for _, mt := range types {
		if mt == nil || mt.Name == nil || mt.GuestCpus == nil || mt.MemoryMb == nil {
			continue
		}
		byID[*mt.Name] = GCPMachineType{
			ID:       *mt.Name,
			VCPUs:    int(*mt.GuestCpus),
			MemoryGB: int(*mt.MemoryMb / 1024),
		}
	}

	result := make([]GCPMachineType, 0, len(byID))
	for _, t := range byID {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// fallbackMachineTypes builds the machine type list from the hardcoded map
// that was served before the Compute API integration. The price is not part
// of the API surface and is dropped here.
func fallbackMachineTypes() []GCPMachineType {
	ids := make([]string, 0, len(db.MachineTypes))
	for id := range db.MachineTypes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	types := make([]GCPMachineType, 0, len(ids))
	for _, id := range ids {
		spec := db.MachineTypes[id]
		types = append(types, GCPMachineType{
			ID:       id,
			VCPUs:    spec.VCPUs,
			MemoryGB: spec.MemoryGB,
		})
	}
	return types
}

// Default service for convenience.
var defaultMachineTypesService = NewMachineTypesService()

// ListGCPMachineTypes returns the machine types to offer, using the default
// service.
var ListGCPMachineTypes = func(ctx context.Context, projectID string) []GCPMachineType {
	return defaultMachineTypesService.List(ctx, projectID)
}
