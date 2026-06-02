package servers

import (
	"encoding/json"
	"net/http"
	"sort"

	"gitlab.com/nbyl/metio/internal/db"
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

	regionIDs := db.ListRegions()
	sort.Strings(regionIDs)
	regions := make([]RegionOption, 0, len(regionIDs))
	for _, regionID := range regionIDs {
		zones := db.ListZonesByRegion(regionID)
		sort.Strings(zones)
		regions = append(regions, RegionOption{ID: regionID, Zones: zones})
	}

	versions := make([]string, len(db.MinecraftVersions))
	copy(versions, db.MinecraftVersions)

	resp := OptionsResponse{
		MachineTypes:     machineTypes,
		Regions:          regions,
		MinecraftVersion: versions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
