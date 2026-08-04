package db

import "time"

// PulumiSettings stores the auto-provisioned Pulumi state bucket configuration.
type PulumiSettings struct {
	StateBucket   string    `json:"stateBucket"`
	Initialized   bool      `json:"initialized"`
	InitializedAt time.Time `json:"initializedAt"`
	InitializedBy string    `json:"initializedBy"`
}
