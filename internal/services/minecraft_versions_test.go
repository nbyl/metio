package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestJSON is a trimmed manifest with releases and non-releases interleaved,
// in Mojang's newest-first order.
const manifestJSON = `{
  "latest": {"release": "26.2", "snapshot": "26.3-snapshot-7"},
  "versions": [
    {"id": "26.3-snapshot-7", "type": "snapshot"},
    {"id": "26.2", "type": "release"},
    {"id": "26.1", "type": "release"},
    {"id": "1.21.11", "type": "release"},
    {"id": "1.21.10", "type": "release"},
    {"id": "b1.7.3", "type": "old_beta"},
    {"id": "a1.0.4", "type": "old_alpha"}
  ]
}`

// newTestService builds a service pointed at the given URL with the supplied TTL.
func newTestVersionService(url string, ttl time.Duration) *MinecraftVersionService {
	return &MinecraftVersionService{
		httpClient:  &http.Client{Timeout: 5 * time.Second},
		manifestURL: url,
		cache:       &versionCache{ttl: ttl},
		fallback:    []string{"fallback-1", "fallback-2"},
	}
}

func TestMinecraftVersionService_FiltersToReleasesPreservingOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(manifestJSON))
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)
	versions := s.List(context.Background())

	// Releases only, in the order Mojang listed them (newest first).
	assert.Equal(t, []string{"26.2", "26.1", "1.21.11", "1.21.10"}, versions)
}

func TestMinecraftVersionService_CachesBetweenCalls(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(manifestJSON))
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)

	first := s.List(context.Background())
	second := s.List(context.Background())

	assert.Equal(t, first, second)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "second call must be served from cache")
}

func TestMinecraftVersionService_RefetchesAfterTTL(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(manifestJSON))
	}))
	defer srv.Close()

	// A TTL that has already elapsed by the time the second call happens.
	s := newTestVersionService(srv.URL, time.Millisecond)

	s.List(context.Background())
	time.Sleep(5 * time.Millisecond)
	s.List(context.Background())

	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "expired cache must trigger a refetch")
}

func TestMinecraftVersionService_ConcurrentCallsDoNotStampede(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(manifestJSON))
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.List(context.Background())
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "concurrent callers must share a single fetch")
}

func TestMinecraftVersionService_FallsBackOnUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)

	assert.Equal(t, []string{"fallback-1", "fallback-2"}, s.List(context.Background()))
}

func TestMinecraftVersionService_FallsBackOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)

	assert.Equal(t, []string{"fallback-1", "fallback-2"}, s.List(context.Background()))
}

func TestMinecraftVersionService_FallsBackWhenManifestHasNoReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"versions":[{"id":"26.3-snapshot-7","type":"snapshot"}]}`))
	}))
	defer srv.Close()

	s := newTestVersionService(srv.URL, time.Hour)

	assert.Equal(t, []string{"fallback-1", "fallback-2"}, s.List(context.Background()))
}

func TestMinecraftVersionService_ServesStaleWhenUpstreamFailsAfterSuccess(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(manifestJSON))
	}))
	defer srv.Close()

	// Short TTL so the second call is forced to refetch.
	s := newTestVersionService(srv.URL, time.Millisecond)

	first := s.List(context.Background())
	require.Equal(t, []string{"26.2", "26.1", "1.21.11", "1.21.10"}, first)

	fail.Store(true)
	time.Sleep(5 * time.Millisecond)

	// The previous result is preferred over the hardcoded fallback.
	assert.Equal(t, first, s.List(context.Background()))
}

func TestNewMinecraftVersionService_UsesMojangManifestAndBuiltinFallback(t *testing.T) {
	s := NewMinecraftVersionService()

	assert.Equal(t, versionManifestURL, s.manifestURL)
	assert.Equal(t, versionCacheTTL, s.cache.ttl)
	assert.NotEmpty(t, s.fallback, "fallback list must not be empty")
}
