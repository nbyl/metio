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

// searchJSON is a trimmed Modrinth search response with two modpack hits.
const searchJSON = `{
  "hits": [
    {
      "project_id": "e0oMrxjp",
      "project_type": "modpack",
      "slug": "skyblock-enhanced",
      "author": "Kd_Gaming1",
      "title": "SkyBlock Enhanced",
      "categories": ["adventure", "fabric", "multiplayer", "optimization"],
      "display_categories": ["adventure", "multiplayer", "optimization"],
      "versions": ["1.21.5", "26.2"],
      "downloads": 1788159,
      "icon_url": "https://cdn.modrinth.com/data/e0oMrxjp/icon.png"
    },
    {
      "project_id": "KmiWHzQ4",
      "project_type": "modpack",
      "slug": "skyblocker-modpack",
      "author": "Wohlhabend",
      "title": "Skyblocker Modpack",
      "categories": ["fabric", "lightweight", "multiplayer"],
      "versions": ["1.20.1", "1.21.5"],
      "downloads": 1083453,
      "icon_url": ""
    }
  ],
  "offset": 0,
  "limit": 2,
  "total_hits": 2
}`

// versionsJSON is a trimmed Modrinth version response for project e0oMrxjp.
const versionsJSON = `[
  {
    "id": "gXxKZ5tA",
    "project_id": "e0oMrxjp",
    "name": "SkyBlock Enhanced 1.0.0",
    "version_number": "1.0.0",
    "date_published": "2025-06-07T04:30:53.734Z",
    "version_type": "release",
    "loaders": ["fabric"],
    "game_versions": ["1.21.5", "26.2"],
    "featured": true,
    "status": "listed",
    "downloads": 1000,
    "files": []
  },
  {
    "id": "nZzYj6bP",
    "project_id": "e0oMrxjp",
    "name": "SkyBlock Enhanced 0.9.0",
    "version_number": "0.9.0",
    "date_published": "2025-04-01T04:30:53.734Z",
    "version_type": "release",
    "loaders": ["fabric"],
    "game_versions": ["1.21.5"],
    "featured": false,
    "status": "listed",
    "downloads": 500,
    "files": []
  }
]`

// newTestModrinthService builds a service pointed at the given URL with the
// supplied TTL.
func newTestModrinthService(url string, ttl time.Duration) *ModrinthService {
	return &ModrinthService{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    url,
		cache:      &modrinthSearchCache{ttl: ttl},
	}
}

func TestModrinthService_SearchNormalizesHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)
	packs := s.Search(context.Background(), "skyblock", "")

	require.Len(t, packs, 2)

	first := packs[0]
	assert.Equal(t, "e0oMrxjp", first.ID)
	assert.Equal(t, "skyblock-enhanced", first.Slug)
	assert.Equal(t, "SkyBlock Enhanced", first.Name)
	assert.Equal(t, "Kd_Gaming1", first.Author)
	assert.Equal(t, "https://cdn.modrinth.com/data/e0oMrxjp/icon.png", first.IconURL)
	assert.Equal(t, int64(1788159), first.Downloads)
	assert.Equal(t, []string{"1.21.5", "26.2"}, first.GameVersions)
	assert.Equal(t, "fabric", first.Loader, "loader must be derived from the hit's categories")

	second := packs[1]
	assert.Equal(t, "Skyblocker Modpack", second.Name)
	assert.Equal(t, "fabric", second.Loader)
}

func TestModrinthService_SearchFiltersByMinecraftVersion(t *testing.T) {
	var gotQuery, gotFacets string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		gotFacets = r.URL.Query().Get("facets")
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)
	s.Search(context.Background(), "skyblock", "1.21.5")

	assert.Equal(t, "skyblock", gotQuery)
	assert.Equal(t, `[["project_type:modpack"],["versions:1.21.5"]]`, gotFacets)
}

func TestModrinthService_SearchOmitsVersionFacetWhenUnfiltered(t *testing.T) {
	var gotFacets string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFacets = r.URL.Query().Get("facets")
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)
	s.Search(context.Background(), "skyblock", "")

	assert.Equal(t, `[["project_type:modpack"]]`, gotFacets)
}

func TestModrinthService_CachesBetweenCalls(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	first := s.Search(context.Background(), "skyblock", "")
	second := s.Search(context.Background(), "skyblock", "")

	assert.Equal(t, first, second)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "identical query must be served from cache")
}

func TestModrinthService_DifferentQueryRefetches(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	s.Search(context.Background(), "skyblock", "")
	s.Search(context.Background(), "fabric", "")

	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "a different query must not reuse cached results")
}

func TestModrinthService_RefetchesAfterTTL(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	// A TTL that has already elapsed by the time the second call happens.
	s := newTestModrinthService(srv.URL, time.Millisecond)

	s.Search(context.Background(), "skyblock", "")
	time.Sleep(5 * time.Millisecond)
	s.Search(context.Background(), "skyblock", "")

	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "expired cache must trigger a refetch")
}

func TestModrinthService_ConcurrentCallsDoNotStampede(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Search(context.Background(), "skyblock", "")
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "concurrent callers must share a single fetch")
}

func TestModrinthService_FallsBackOnUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	assert.Equal(t, []ModrinthPack{}, s.Search(context.Background(), "skyblock", ""))
}

func TestModrinthService_FallsBackOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	assert.Equal(t, []ModrinthPack{}, s.Search(context.Background(), "skyblock", ""))
}

func TestModrinthService_ServesStaleWhenUpstreamFailsAfterSuccess(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(searchJSON))
	}))
	defer srv.Close()

	// Short TTL so the second call is forced to refetch.
	s := newTestModrinthService(srv.URL, time.Millisecond)

	first := s.Search(context.Background(), "skyblock", "")
	require.Len(t, first, 2)

	fail.Store(true)
	time.Sleep(5 * time.Millisecond)

	assert.Equal(t, first, s.Search(context.Background(), "skyblock", ""))
}

func TestModrinthService_VersionsNormalizesVersions(t *testing.T) {
	var gotPath, gotChangelog string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotChangelog = r.URL.Query().Get("include_changelog")
		w.Write([]byte(versionsJSON))
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)
	versions := s.Versions(context.Background(), "e0oMrxjp")

	require.Len(t, versions, 2)
	assert.Equal(t, "/project/e0oMrxjp/version", gotPath)
	assert.Equal(t, "false", gotChangelog, "changelogs are not needed for the listing")

	latest := versions[0]
	assert.Equal(t, "gXxKZ5tA", latest.ID)
	assert.Equal(t, "SkyBlock Enhanced 1.0.0", latest.Name)
	assert.Equal(t, "1.0.0", latest.VersionNumber)
	assert.Equal(t, []string{"fabric"}, latest.Loaders)
	assert.Equal(t, []string{"1.21.5", "26.2"}, latest.GameVersions)
	assert.True(t, latest.Latest)

	assert.False(t, versions[1].Latest)
}

func TestModrinthService_VersionsFallsBackOnUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newTestModrinthService(srv.URL, time.Hour)

	assert.Equal(t, []ModrinthVersion{}, s.Versions(context.Background(), "e0oMrxjp"))
}

func TestNewModrinthService_UsesModrinthBaseURLAndTimeout(t *testing.T) {
	s := NewModrinthService()

	assert.Equal(t, modrinthBaseURL, s.baseURL)
	assert.Equal(t, modrinthSearchCacheTTL, s.cache.ttl)
	assert.NotNil(t, s.httpClient)
	assert.Equal(t, 10*time.Second, s.httpClient.Timeout)
}
