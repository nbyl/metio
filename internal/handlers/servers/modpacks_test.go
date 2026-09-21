package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newModpackRouter wires the API routes the way base.go does, without auth, so
// tests exercise registration as well as the handlers themselves.
func newModpackRouter() *mux.Router {
	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api").Subrouter()
	RegisterRoutes(apiRouter)
	return r
}

func stubSearchModrinthPacks(fn func(ctx context.Context, query, mcVersion string) []services.ModrinthPack) func() {
	orig := SearchModrinthPacks
	SearchModrinthPacks = fn
	return func() { SearchModrinthPacks = orig }
}

func stubListModrinthPackVersions(fn func(ctx context.Context, modpackID string) []services.ModrinthVersion) func() {
	orig := ListModrinthPackVersions
	ListModrinthPackVersions = fn
	return func() { ListModrinthPackVersions = orig }
}

func TestSearchModpacks_PassesQueryAndVersionThrough(t *testing.T) {
	var gotQuery, gotVersion string
	restore := stubSearchModrinthPacks(func(ctx context.Context, query, mcVersion string) []services.ModrinthPack {
		gotQuery, gotVersion = query, mcVersion
		return []services.ModrinthPack{{
			ID:           "e0oMrxjp",
			Slug:         "skyblock-enhanced",
			Name:         "SkyBlock Enhanced",
			Author:       "Kd_Gaming1",
			IconURL:      "https://cdn.modrinth.com/icon.png",
			Downloads:    1788159,
			GameVersions: []string{"1.21.5", "26.2"},
			Loader:       "fabric",
		}}
	})
	defer restore()

	req := httptest.NewRequest("GET", "/api/modpacks/search?query=skyblock&mcVersion=1.21.5", nil)
	w := httptest.NewRecorder()
	newModpackRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "skyblock", gotQuery)
	assert.Equal(t, "1.21.5", gotVersion)
	assert.Contains(t, w.Body.String(), `"iconUrl"`, "normalised JSON tags must be relayed to the browser")
	assert.Contains(t, w.Body.String(), `"gameVersions"`)

	var packs []services.ModrinthPack
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &packs))
	require.Len(t, packs, 1)
	assert.Equal(t, "e0oMrxjp", packs[0].ID)
	assert.Equal(t, "fabric", packs[0].Loader)
}

func TestSearchModpacks_EmptyQueryReturnsEmptyArray(t *testing.T) {
	var called bool
	restore := stubSearchModrinthPacks(func(ctx context.Context, query, mcVersion string) []services.ModrinthPack {
		called = true
		return nil
	})
	defer restore()

	req := httptest.NewRequest("GET", "/api/modpacks/search", nil)
	w := httptest.NewRecorder()
	newModpackRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, called, "an empty query must not reach the service")
	assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
}

func TestSearchModpacks_DegradesWhenServiceEmpty(t *testing.T) {
	restore := stubSearchModrinthPacks(func(ctx context.Context, query, mcVersion string) []services.ModrinthPack {
		return []services.ModrinthPack{}
	})
	defer restore()

	req := httptest.NewRequest("GET", "/api/modpacks/search?query=skyblock", nil)
	w := httptest.NewRecorder()
	newModpackRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
}

func TestListModpackVersions_UsesIDFromPath(t *testing.T) {
	var gotID string
	restore := stubListModrinthPackVersions(func(ctx context.Context, modpackID string) []services.ModrinthVersion {
		gotID = modpackID
		return []services.ModrinthVersion{{
			ID:            "gXxKZ5tA",
			Name:          "SkyBlock Enhanced 1.0.0",
			VersionNumber: "1.0.0",
			GameVersions:  []string{"1.21.5", "26.2"},
			Loaders:       []string{"fabric"},
			Latest:        true,
		}}
	})
	defer restore()

	req := httptest.NewRequest("GET", "/api/modpacks/e0oMrxjp/versions", nil)
	w := httptest.NewRecorder()
	newModpackRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "e0oMrxjp", gotID)

	var versions []services.ModrinthVersion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &versions))
	require.Len(t, versions, 1)
	assert.Equal(t, "gXxKZ5tA", versions[0].ID)
	assert.True(t, versions[0].Latest)
}

func TestListModpackVersions_DegradesWhenServiceEmpty(t *testing.T) {
	restore := stubListModrinthPackVersions(func(ctx context.Context, modpackID string) []services.ModrinthVersion {
		return []services.ModrinthVersion{}
	})
	defer restore()

	req := httptest.NewRequest("GET", "/api/modpacks/e0oMrxjp/versions", nil)
	w := httptest.NewRecorder()
	newModpackRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
}

func TestIsModpackAvailable_ResolvesThroughPickerSource(t *testing.T) {
	restore := stubListModrinthPackVersions(func(ctx context.Context, modpackID string) []services.ModrinthVersion {
		return []services.ModrinthVersion{{ID: "gXxKZ5tA"}}
	})
	defer restore()

	assert.True(t, isModpackAvailable(context.Background(), "e0oMrxjp"))
}

func TestIsModpackAvailable_FalseForUnknownPack(t *testing.T) {
	restore := stubListModrinthPackVersions(func(ctx context.Context, modpackID string) []services.ModrinthVersion {
		return []services.ModrinthVersion{}
	})
	defer restore()

	assert.False(t, isModpackAvailable(context.Background(), "e0oMrxjp"))
}
