package servers

import (
	"context"
	"net/http"

	"github.com/nbyl/metio/internal/config"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/services"
)

var GetDBConnection func(ctx context.Context) (db.DB, config.Config, error)

var ProvisioningService ProvisioningServiceInterface

var LookupMinecraftUser func(ctx context.Context, username string) (*services.MojangProfile, error)

// ListMinecraftVersions returns the Minecraft versions offered to users. It
// defaults to the built-in list so tests and any caller that has not wired the
// live service still get a usable set; base.go overrides it with the
// Mojang-backed service in production.
var ListMinecraftVersions = func(ctx context.Context) []string {
	return db.MinecraftVersions
}

// ListGCPRegions returns the regions and their zones offered to users. It
// defaults to the built-in list so tests and any caller that has not wired the
// live service still get a usable set; base.go overrides it with the
// Compute-backed service in production.
var ListGCPRegions = func(ctx context.Context) []RegionOption {
	regionIDs := db.ListRegions()
	locations := make([]services.GCPLocation, 0, len(regionIDs))
	for _, regionID := range regionIDs {
		locations = append(locations, services.GCPLocation{ID: regionID, Zones: db.ListZonesByRegion(regionID)})
	}
	return LocationsToRegionOptions(locations)
}

var GetUserEmail func(r *http.Request) string

var WriteJSONError func(w http.ResponseWriter, message string, statusCode int)

var ControllerVersion string
