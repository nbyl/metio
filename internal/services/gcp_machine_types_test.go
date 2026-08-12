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

// fakeMachineType builds a compute MachineType with the given name, CPUs and
// memory in MB.
func fakeMachineType(name string, cpus, memoryMB int32) *computepb.MachineType {
	return &computepb.MachineType{
		Name:      &name,
		GuestCpus: &cpus,
		MemoryMb:  &memoryMB,
	}
}

// newTestMachineTypesService builds a service pointed at the given fetcher
// with the supplied TTL.
func newTestMachineTypesService(fn machineTypesFetcher, ttl time.Duration) *MachineTypesService {
	return &MachineTypesService{
		fetchTypes: fn,
		cache:      &machineTypesCache{ttl: ttl},
		fallback: []GCPMachineType{
			{ID: "e2-small", VCPUs: 2, MemoryGB: 2},
		},
	}
}

func TestMachineTypesService_DeduplicatesZonesAndSorts(t *testing.T) {
	// The same type appears in every zone; only one entry per name should be
	// returned, sorted by ID.
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		return []*computepb.MachineType{
			fakeMachineType("n2-standard-2", 2, 8192),
			fakeMachineType("e2-small", 2, 2048),
			fakeMachineType("e2-small", 2, 2048),
			fakeMachineType("n2-standard-2", 2, 8192),
		}, nil
	}, time.Hour)

	types := s.List(context.Background(), "p")

	assert.Equal(t, []GCPMachineType{
		{ID: "e2-small", VCPUs: 2, MemoryGB: 2},
		{ID: "n2-standard-2", VCPUs: 2, MemoryGB: 8},
	}, types)
}

func TestMachineTypesService_CachesBetweenCalls(t *testing.T) {
	var calls int32
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		atomic.AddInt32(&calls, 1)
		return []*computepb.MachineType{fakeMachineType("e2-small", 2, 2048)}, nil
	}, time.Hour)

	s.List(context.Background(), "p")
	s.List(context.Background(), "p")

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestMachineTypesService_RefetchesAfterTTL(t *testing.T) {
	var calls int32
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		atomic.AddInt32(&calls, 1)
		return []*computepb.MachineType{fakeMachineType("e2-small", 2, 2048)}, nil
	}, -time.Nanosecond)

	s.List(context.Background(), "p")
	s.List(context.Background(), "p")

	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestMachineTypesService_ConcurrentCallersSingleFetch(t *testing.T) {
	var calls int32
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(10 * time.Millisecond)
		return []*computepb.MachineType{fakeMachineType("e2-small", 2, 2048)}, nil
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

func TestMachineTypesService_FallbackOnError(t *testing.T) {
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		return nil, errors.New("upstream unavailable")
	}, time.Hour)

	assert.Equal(t, s.fallback, s.List(context.Background(), "p"))
}

func TestMachineTypesService_FallbackOnEmptyResult(t *testing.T) {
	// A response with no usable machine types must fall back rather than
	// offer an empty list.
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		return []*computepb.MachineType{}, nil
	}, time.Hour)

	assert.Equal(t, s.fallback, s.List(context.Background(), "p"))
}

func TestMachineTypesService_ServesStaleAfterError(t *testing.T) {
	calls := 0
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		calls++
		if calls == 1 {
			return []*computepb.MachineType{
				fakeMachineType("e2-small", 2, 2048),
				fakeMachineType("c3-highcpu-8", 8, 16384),
			}, nil
		}
		return nil, errors.New("upstream unavailable")
	}, -time.Nanosecond)

	first := s.List(context.Background(), "p")
	assert.Len(t, first, 2)

	// The second call hits a stale cache entry rather than the hardcoded
	// fallback, so the previously fetched types are still served.
	second := s.List(context.Background(), "p")
	assert.Equal(t, first, second)
}

func TestMachineTypesService_DropsEntriesWithoutFields(t *testing.T) {
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		return []*computepb.MachineType{
			fakeMachineType("e2-small", 2, 2048),
			{Name: strPtr("no-cpus")},
			{Name: strPtr("no-memory")},
			{},
		}, nil
	}, time.Hour)

	assert.Equal(t, []GCPMachineType{
		{ID: "e2-small", VCPUs: 2, MemoryGB: 2},
	}, s.List(context.Background(), "p"))
}

func TestMachineTypesService_ConvertsMemoryMBToGB(t *testing.T) {
	s := newTestMachineTypesService(func(ctx context.Context, projectID string) ([]*computepb.MachineType, error) {
		return []*computepb.MachineType{fakeMachineType("m3-megamem-64", 64, 1048576)}, nil
	}, time.Hour)

	assert.Equal(t, []GCPMachineType{
		{ID: "m3-megamem-64", VCPUs: 64, MemoryGB: 1024},
	}, s.List(context.Background(), "p"))
}
