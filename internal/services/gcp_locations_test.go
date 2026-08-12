package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/assert"
)

// fakeZone builds a compute Zone with the given name and region URL.
func fakeZone(name, region string) *computepb.Zone {
	regionURL := "https://www.googleapis.com/compute/v1/projects/p/regions/" + region
	return &computepb.Zone{Name: &name, Region: &regionURL}
}

// newTestLocationsService builds a service pointed at the given fetcher with
// the supplied TTL.
func newTestLocationsService(fn zonesFetcher, ttl time.Duration) *LocationsService {
	return &LocationsService{
		fetchZones: fn,
		cache:      &locationsCache{ttl: ttl},
		fallback: []GCPLocation{
			{ID: "europe-west1", Zones: []string{"europe-west1-b"}},
		},
	}
}

func TestLocationsService_GroupsZonesByRegionSorted(t *testing.T) {
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		return []*computepb.Zone{
			fakeZone("us-central1-c", "us-central1"),
			fakeZone("us-central1-a", "us-central1"),
			fakeZone("europe-west1-b", "europe-west1"),
		}, nil
	}, time.Hour)

	locations := s.List(context.Background(), "p")

	assert.Equal(t, []GCPLocation{
		{ID: "europe-west1", Zones: []string{"europe-west1-b"}},
		{ID: "us-central1", Zones: []string{"us-central1-a", "us-central1-c"}},
	}, locations)
}

func TestLocationsService_CachesBetweenCalls(t *testing.T) {
	var calls int32
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		atomic.AddInt32(&calls, 1)
		return []*computepb.Zone{fakeZone("europe-west1-b", "europe-west1")}, nil
	}, time.Hour)

	s.List(context.Background(), "p")
	s.List(context.Background(), "p")

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestLocationsService_RefetchesAfterTTL(t *testing.T) {
	var calls int32
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		atomic.AddInt32(&calls, 1)
		return []*computepb.Zone{fakeZone("europe-west1-b", "europe-west1")}, nil
	}, -time.Nanosecond)

	s.List(context.Background(), "p")
	s.List(context.Background(), "p")

	// The first call populated the cache; the negative TTL expires it, forcing
	// a refetch on the second call.
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestLocationsService_ConcurrentCallersSingleFetch(t *testing.T) {
	var calls int32
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(10 * time.Millisecond)
		return []*computepb.Zone{fakeZone("europe-west1-b", "europe-west1")}, nil
	}, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.List(context.Background(), "p")
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestLocationsService_FallbackOnError(t *testing.T) {
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		return nil, errors.New("upstream unavailable")
	}, time.Hour)

	assert.Equal(t, s.fallback, s.List(context.Background(), "p"))
}

func TestLocationsService_FallbackOnEmptyGrouping(t *testing.T) {
	// A response that yields no usable regions (e.g. zones without a region
	// URL) must fall back rather than offer an empty list.
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		return []*computepb.Zone{{Name: strPtr("europe-west1-b")}}, nil
	}, time.Hour)

	assert.Equal(t, s.fallback, s.List(context.Background(), "p"))
}

func TestLocationsService_ServesStaleAfterError(t *testing.T) {
	calls := 0
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		calls++
		if calls == 1 {
			return []*computepb.Zone{
				fakeZone("europe-west1-b", "europe-west1"),
				fakeZone("europe-west1-c", "europe-west1"),
			}, nil
		}
		return nil, errors.New("upstream unavailable")
	}, -time.Nanosecond)

	first := s.List(context.Background(), "p")
	assert.Len(t, first[0].Zones, 2)

	// The second call hits a stale cache entry rather than the hardcoded
	// fallback, so the previously fetched zones are still served.
	second := s.List(context.Background(), "p")
	assert.Equal(t, first, second)
}

func TestLocationsService_IgnoresZonesWithNoRegion(t *testing.T) {
	s := newTestLocationsService(func(ctx context.Context, projectID string) ([]*computepb.Zone, error) {
		return []*computepb.Zone{
			fakeZone("europe-west1-b", "europe-west1"),
			{Name: strPtr("orphan-zone")},
		}, nil
	}, time.Hour)

	assert.Equal(t, []GCPLocation{
		{ID: "europe-west1", Zones: []string{"europe-west1-b"}},
	}, s.List(context.Background(), "p"))
}

func strPtr(s string) *string { return &s }
