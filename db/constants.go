package db

type MachineTypeSpec struct {
	VCPUs       int
	MemoryGB    int
	MonthlyCost float64
}

var MachineTypes = map[string]MachineTypeSpec{
	"e2-medium":      {2, 4, 48.73},
	"e2-small":       {2, 2, 24.37},
	"e2-standard-2":  {2, 8, 60.69},
	"e2-standard-4":  {4, 16, 121.38},
	"e2-standard-8":  {8, 32, 242.76},
	"e2-standard-16": {16, 64, 485.51},
	"e2-standard-32": {32, 128, 971.03},
	"n2-standard-2":  {2, 8, 73.19},
	"n2-standard-4":  {4, 16, 146.39},
	"n2-standard-8":  {8, 32, 292.77},
	"n2-standard-16": {16, 64, 585.54},
	"n2-standard-32": {32, 128, 1171.08},
	"n2-standard-48": {48, 192, 1756.62},
	"n2-standard-64": {64, 256, 2342.16},
	"n2-standard-80": {80, 320, 2927.70},
	"n2-highmem-2":   {2, 16, 95.62},
	"n2-highmem-4":   {4, 32, 191.24},
	"n2-highmem-8":   {8, 64, 382.48},
	"n2-highmem-16":  {16, 128, 764.96},
	"n2-highmem-32":  {32, 256, 1529.91},
	"n2-highmem-48":  {48, 384, 2294.87},
	"n2-highmem-64":  {64, 512, 3059.83},
	"n2-highcpu-2":   {2, 2, 60.40},
	"n2-highcpu-4":   {4, 4, 120.80},
	"n2-highcpu-8":   {8, 8, 241.60},
	"n2-highcpu-16":  {16, 16, 483.19},
	"n2-highcpu-32":  {32, 32, 966.38},
	"n2-highcpu-48":  {48, 48, 1449.57},
	"n2-highcpu-64":  {64, 64, 1932.76},
	"n2-highcpu-80":  {80, 80, 2415.96},
}

var MinecraftVersions = []string{
	"1.7.10",
	"1.8.9",
	"1.9.4",
	"1.10.2",
	"1.11.2",
	"1.12.2",
	"1.13.2",
	"1.14.4",
	"1.15.2",
	"1.16.5",
	"1.17.1",
	"1.18.2",
	"1.19.4",
	"1.20.1",
	"1.20.4",
	"1.21.1",
	"1.21.3",
	"1.21.10",
}

func IsValidMinecraftVersion(version string) bool {
	for _, v := range MinecraftVersions {
		if v == version {
			return true
		}
	}
	return false
}
