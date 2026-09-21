package servers

import (
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/nbyl/metio/internal/services"
)

// SearchModpacks serves GET /api/modpacks/search. It responds 200 with the
// Modrinth results as a JSON array; the service never returns an error, so an
// unreachable Modrinth yields 200 with empty or stale results rather than a
// failure the browser could not recover from.
func SearchModpacks(w http.ResponseWriter, r *http.Request) {
	query, mcVersion := searchParams(r.URL)

	packs := []services.ModrinthPack{}
	if query != "" {
		packs = SearchModrinthPacks(r.Context(), query, mcVersion)
	}

	writeJSONResponse(w, http.StatusOK, packs)
}

// ListModpackVersions serves GET /api/modpacks/{id}/versions. It responds 200
// with the pack's versions as a JSON array, degraded to empty when Modrinth is
// unreachable.
func ListModpackVersions(w http.ResponseWriter, r *http.Request) {
	modpackID := mux.Vars(r)["id"]

	writeJSONResponse(w, http.StatusOK, ListModrinthPackVersions(r.Context(), modpackID))
}

// searchParams extracts the query and optional Minecraft version filter.
func searchParams(u *url.URL) (query, mcVersion string) {
	q := u.Query()
	return q.Get("query"), q.Get("mcVersion")
}
