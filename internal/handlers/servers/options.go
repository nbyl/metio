package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/services"
)

type MachineTypeOption struct {
	ID          string  `json:"id"`
	VCPUs       int     `json:"vcpus"`
	MemoryGB    int     `json:"memoryGB"`
	MonthlyCost float64 `json:"monthlyCost"`
}

type RegionOption struct {
	ID    string   `json:"id"`
	Zones []string `json:"zones"`
}

type OptionsResponse struct {
	MachineTypes     []MachineTypeOption `json:"machineTypes"`
	Regions          []RegionOption      `json:"regions"`
	MinecraftVersion []string            `json:"minecraftVersions"`
}

func ListOptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	machineTypes := make([]MachineTypeOption, 0, len(db.MachineTypes))
	for id, spec := range db.MachineTypes {
		machineTypes = append(machineTypes, MachineTypeOption{
			ID:          id,
			VCPUs:       spec.VCPUs,
			MemoryGB:    spec.MemoryGB,
			MonthlyCost: spec.MonthlyCost,
		})
	}
	sort.Slice(machineTypes, func(i, j int) bool {
		return machineTypes[i].ID < machineTypes[j].ID
	})

	regions := ListGCPRegions(ctx)

	available := ListMinecraftVersions(ctx)
	versions := make([]string, len(available))
	copy(versions, available)

	resp := OptionsResponse{
		MachineTypes:     machineTypes,
		Regions:          regions,
		MinecraftVersion: versions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// isMinecraftVersionAvailable reports whether the given version is one of the
// versions currently offered. It deliberately resolves the list through
// ListMinecraftVersions, the same source GET /api/options serves, so the
// dropdown and this validation can never disagree.
func isMinecraftVersionAvailable(ctx context.Context, version string) bool {
	for _, v := range ListMinecraftVersions(ctx) {
		if v == version {
			return true
		}
	}
	return false
}

// LocationsToRegionOptions converts GCP region locations into the sorted API
// response shape. The service already returns sorted regions and zones; the
// sort here keeps the shape stable regardless of the source.
func LocationsToRegionOptions(locations []services.GCPLocation) []RegionOption {
	regions := make([]RegionOption, 0, len(locations))
	for _, loc := range locations {
		zones := make([]string, len(loc.Zones))
		copy(zones, loc.Zones)
		sort.Strings(zones)
		regions = append(regions, RegionOption{ID: loc.ID, Zones: zones})
	}
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].ID < regions[j].ID
	})
	return regions
}

// isRegionAvailable reports whether the given region is offered. It resolves
// the list through ListGCPRegions, the same source GET /api/options serves.
func isRegionAvailable(ctx context.Context, region string) bool {
	for _, r := range ListGCPRegions(ctx) {
		if r.ID == region {
			return true
		}
	}
	return false
}

// isZoneAvailable reports whether the given zone is offered within region,
// resolved from the same source GET /api/options serves.
func isZoneAvailable(ctx context.Context, region, zone string) bool {
	for _, r := range ListGCPRegions(ctx) {
		if r.ID == region {
			for _, z := range r.Zones {
				if z == zone {
					return true
				}
			}
			return false
		}
	}
	return false
}
