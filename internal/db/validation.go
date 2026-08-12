package db

import (
	"fmt"
	"regexp"
)

var (
	nameRegex       = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z][a-z0-9]$|^[a-z]$`)
	timeRegex       = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]$`)
	regionRegex     = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	zoneRegex       = regexp.MustCompile(`^[a-z][a-z0-9-]*?-[a-z]$`)
	validGCPRegions = map[string]bool{
		"us-central1":             true,
		"us-east1":                true,
		"us-east4":                true,
		"us-west1":                true,
		"us-west2":                true,
		"us-west3":                true,
		"us-west4":                true,
		"northamerica-northeast1": true,
		"southamerica-east1":      true,
		"europe-west1":            true,
		"europe-west2":            true,
		"europe-west3":            true,
		"europe-west4":            true,
		"europe-west6":            true,
		"europe-central2":         true,
		"asia-east1":              true,
		"asia-east2":              true,
		"asia-northeast1":         true,
		"asia-northeast2":         true,
		"asia-northeast3":         true,
		"asia-south1":             true,
		"asia-southeast1":         true,
		"asia-southeast2":         true,
		"australia-southeast1":    true,
	}
	validGCPTZonesByRegion = map[string]map[string]bool{
		"us-central1":             {"us-central1-a": true, "us-central1-b": true, "us-central1-c": true, "us-central1-f": true},
		"us-east1":                {"us-east1-b": true, "us-east1-c": true, "us-east1-d": true},
		"us-east4":                {"us-east4-a": true, "us-east4-b": true, "us-east4-c": true},
		"us-west1":                {"us-west1-a": true, "us-west1-b": true, "us-west1-c": true},
		"us-west2":                {"us-west2-a": true, "us-west2-b": true, "us-west2-c": true},
		"us-west3":                {"us-west3-a": true, "us-west3-b": true, "us-west3-c": true},
		"us-west4":                {"us-west4-a": true, "us-west4-b": true, "us-west4-c": true},
		"northamerica-northeast1": {"northamerica-northeast1-a": true, "northamerica-northeast1-b": true, "northamerica-northeast1-c": true},
		"southamerica-east1":      {"southamerica-east1-a": true, "southamerica-east1-b": true, "southamerica-east1-c": true},
		"europe-west1":            {"europe-west1-b": true, "europe-west1-c": true, "europe-west1-d": true},
		"europe-west2":            {"europe-west2-a": true, "europe-west2-b": true, "europe-west2-c": true},
		"europe-west3":            {"europe-west3-a": true, "europe-west3-b": true, "europe-west3-c": true},
		"europe-west4":            {"europe-west4-a": true, "europe-west4-b": true, "europe-west4-c": true},
		"europe-west6":            {"europe-west6-a": true, "europe-west6-b": true, "europe-west6-c": true},
		"europe-central2":         {"europe-central2-a": true, "europe-central2-b": true, "europe-central2-c": true},
		"asia-east1":              {"asia-east1-a": true, "asia-east1-b": true, "asia-east1-c": true},
		"asia-east2":              {"asia-east2-a": true, "asia-east2-b": true, "asia-east2-c": true},
		"asia-northeast1":         {"asia-northeast1-a": true, "asia-northeast1-b": true, "asia-northeast1-c": true},
		"asia-northeast2":         {"asia-northeast2-a": true, "asia-northeast2-b": true, "asia-northeast2-c": true},
		"asia-northeast3":         {"asia-northeast3-a": true, "asia-northeast3-b": true, "asia-northeast3-c": true},
		"asia-south1":             {"asia-south1-a": true, "asia-south1-b": true, "asia-south1-c": true},
		"asia-southeast1":         {"asia-southeast1-a": true, "asia-southeast1-b": true, "asia-southeast1-c": true},
		"asia-southeast2":         {"asia-southeast2-a": true, "asia-southeast2-b": true, "asia-southeast2-c": true},
		"australia-southeast1":    {"australia-southeast1-a": true, "australia-southeast1-b": true, "australia-southeast1-c": true},
	}
)

func isValidTimeFormat(t string) bool {
	return timeRegex.MatchString(t)
}

func isValidTimezone(tz string) bool {
	commonTimezones := map[string]bool{
		"Africa/Cairo":                   true,
		"Africa/Johannesburg":            true,
		"America/Anchorage":              true,
		"America/Argentina/Buenos_Aires": true,
		"America/Bogota":                 true,
		"America/Chicago":                true,
		"America/Denver":                 true,
		"America/Halifax":                true,
		"America/Los_Angeles":            true,
		"America/Mexico_City":            true,
		"America/New_York":               true,
		"America/Phoenix":                true,
		"America/Santiago":               true,
		"America/Sao_Paulo":              true,
		"America/Toronto":                true,
		"America/Vancouver":              true,
		"Asia/Baghdad":                   true,
		"Asia/Bangkok":                   true,
		"Asia/Dubai":                     true,
		"Asia/Hong_Kong":                 true,
		"Asia/Jakarta":                   true,
		"Asia/Jerusalem":                 true,
		"Asia/Karachi":                   true,
		"Asia/Kolkata":                   true,
		"Asia/Kuala_Lumpur":              true,
		"Asia/Manila":                    true,
		"Asia/Seoul":                     true,
		"Asia/Shanghai":                  true,
		"Asia/Singapore":                 true,
		"Asia/Taipei":                    true,
		"Asia/Tokyo":                     true,
		"Australia/Melbourne":            true,
		"Australia/Perth":                true,
		"Australia/Sydney":               true,
		"Europe/Amsterdam":               true,
		"Europe/Athens":                  true,
		"Europe/Berlin":                  true,
		"Europe/Brussels":                true,
		"Europe/Budapest":                true,
		"Europe/Copenhagen":              true,
		"Europe/Dublin":                  true,
		"Europe/Helsinki":                true,
		"Europe/Istanbul":                true,
		"Europe/Lisbon":                  true,
		"Europe/London":                  true,
		"Europe/Madrid":                  true,
		"Europe/Moscow":                  true,
		"Europe/Oslo":                    true,
		"Europe/Paris":                   true,
		"Europe/Prague":                  true,
		"Europe/Rome":                    true,
		"Europe/Stockholm":               true,
		"Europe/Vienna":                  true,
		"Europe/Warsaw":                  true,
		"Europe/Zurich":                  true,
		"Pacific/Auckland":               true,
		"Pacific/Honolulu":               true,
		"US/Alaska":                      true,
		"US/Arizona":                     true,
		"US/Central":                     true,
		"US/Eastern":                     true,
		"US/Hawaii":                      true,
		"US/Mountain":                    true,
		"US/Pacific":                     true,
		"UTC":                            true,
	}
	return commonTimezones[tz]
}

func ValidateServerConfig(config *ServerConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if err := ValidateServerName(config.Name); err != nil {
		return err
	}

	if err := ValidateRegion(config.Region); err != nil {
		return err
	}

	if err := ValidateZone(config.Zone); err != nil {
		return err
	}

	if err := ValidateMachineType(config.MachineType); err != nil {
		return err
	}

	if err := ValidateDiskSize(config.DiskSizeGB); err != nil {
		return err
	}

	if config.ShutdownSchedule != nil {
		if !config.ShutdownSchedule.Enabled {
			return nil
		}
		if config.ShutdownSchedule.Time == "" || config.ShutdownSchedule.Timezone == "" {
			return fmt.Errorf("shutdown schedule time and timezone are required when enabled")
		}
		if !isValidTimeFormat(config.ShutdownSchedule.Time) {
			return fmt.Errorf("shutdown schedule time must be in HH:MM format")
		}
		if !isValidTimezone(config.ShutdownSchedule.Timezone) {
			return fmt.Errorf("invalid timezone: %s", config.ShutdownSchedule.Timezone)
		}
	}

	return nil
}

func ValidateServerName(name string) error {
	if len(name) < 3 || len(name) > 24 {
		return fmt.Errorf("name must be between 3 and 24 characters, got %d", len(name))
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("name must start with a lowercase letter, end with a lowercase letter or digit, and contain only lowercase letters, digits, and hyphens")
	}
	return nil
}

// ValidateRegion checks the region's format only. Membership against the
// currently offered set is enforced at the handler layer against the same
// dynamic source /api/options serves, so this validator stays stable while
// the offered set changes over time.
func ValidateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("region is required")
	}
	if !regionRegex.MatchString(region) {
		return fmt.Errorf("invalid GCP region format: %s", region)
	}
	return nil
}

// ValidateZone checks the zone's format only. Membership against the
// currently offered set is enforced at the handler layer against the same
// dynamic source /api/options serves.
func ValidateZone(zone string) error {
	if zone == "" {
		return fmt.Errorf("zone is required")
	}
	if !zoneRegex.MatchString(zone) {
		return fmt.Errorf("invalid GCP zone format: %s", zone)
	}
	return nil
}

func ValidateMachineType(machineType string) error {
	if machineType == "" {
		return fmt.Errorf("machine type is required")
	}
	if _, ok := MachineTypes[machineType]; !ok {
		return fmt.Errorf("invalid machine type: %s", machineType)
	}
	return nil
}

func ListRegions() []string {
	regions := make([]string, 0, len(validGCPRegions))
	for r := range validGCPRegions {
		regions = append(regions, r)
	}
	return regions
}

func ListZonesByRegion(region string) []string {
	zones, ok := validGCPTZonesByRegion[region]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(zones))
	for z := range zones {
		result = append(result, z)
	}
	return result
}

func ListMachineTypes() map[string]MachineTypeSpec {
	result := make(map[string]MachineTypeSpec, len(MachineTypes))
	for k, v := range MachineTypes {
		result[k] = v
	}
	return result
}

func ValidateDiskSize(diskSizeGB int) error {
	if diskSizeGB < 10 {
		return fmt.Errorf("disk size must be at least 10 GB, got %d", diskSizeGB)
	}
	if diskSizeGB > 1000 {
		return fmt.Errorf("disk size must be at most 1000 GB, got %d", diskSizeGB)
	}
	return nil
}
