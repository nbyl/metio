package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"gitlab.com/nbyl/metio/internal/config"
)

var requiredAPIs = []string{
	"compute.googleapis.com",
	"storage.googleapis.com",
	"firestore.googleapis.com",
	"iam.googleapis.com",
	"cloudresourcemanager.googleapis.com",
	"serviceusage.googleapis.com",
	"artifactregistry.googleapis.com",
	"secretmanager.googleapis.com",
	"run.googleapis.com",
}

var requiredPermissions = []string{
	"compute.instances.create",
	"compute.instances.delete",
	"compute.instances.get",
	"compute.instances.start",
	"compute.instances.stop",
	"compute.disks.create",
	"compute.disks.delete",
	"compute.addresses.create",
	"compute.addresses.delete",
	"compute.firewalls.create",
	"compute.firewalls.delete",
	"compute.networks.get",
	"compute.zoneOperations.get",
	"storage.buckets.create",
	"storage.buckets.get",
	"storage.buckets.delete",
	"storage.objects.create",
	"storage.objects.delete",
	"storage.objects.get",
	"iam.serviceAccounts.create",
	"iam.serviceAccounts.delete",
	"iam.serviceAccounts.actAs",
	"iam.serviceAccounts.signBlob",
	"datastore.entities.create",
	"datastore.entities.get",
	"datastore.entities.update",
	"datastore.entities.delete",
	"datastore.entities.list",
	"artifactregistry.repositories.downloadArtifacts",
}

var permissionToRole = map[string]string{
	"compute.instances.create":                        "roles/compute.admin",
	"compute.instances.delete":                        "roles/compute.admin",
	"compute.instances.get":                           "roles/compute.viewer",
	"compute.instances.start":                         "roles/compute.admin",
	"compute.instances.stop":                          "roles/compute.admin",
	"compute.disks.create":                            "roles/compute.admin",
	"compute.disks.delete":                            "roles/compute.admin",
	"compute.addresses.create":                        "roles/compute.admin",
	"compute.addresses.delete":                        "roles/compute.admin",
	"compute.firewalls.create":                        "roles/compute.admin",
	"compute.firewalls.delete":                        "roles/compute.admin",
	"compute.networks.get":                            "roles/compute.networkViewer",
	"compute.zoneOperations.get":                      "roles/compute.viewer",
	"storage.buckets.create":                          "roles/storage.admin",
	"storage.buckets.get":                             "roles/storage.objectViewer",
	"storage.buckets.delete":                          "roles/storage.admin",
	"storage.objects.create":                          "roles/storage.objectCreator",
	"storage.objects.delete":                          "roles/storage.objectAdmin",
	"storage.objects.get":                             "roles/storage.objectViewer",
	"iam.serviceAccounts.create":                      "roles/iam.serviceAccountAdmin",
	"iam.serviceAccounts.delete":                      "roles/iam.serviceAccountAdmin",
	"iam.serviceAccounts.actAs":                       "roles/iam.serviceAccountUser",
	"iam.serviceAccounts.signBlob":                    "roles/iam.serviceAccountTokenCreator",
	"datastore.entities.create":                       "roles/datastore.user",
	"datastore.entities.get":                          "roles/datastore.user",
	"datastore.entities.update":                       "roles/datastore.user",
	"datastore.entities.delete":                       "roles/datastore.user",
	"datastore.entities.list":                         "roles/datastore.user",
	"artifactregistry.repositories.downloadArtifacts": "roles/artifactregistry.reader",
}

const defaultRole = "roles/owner"

type APICheckResult struct {
	Enabled bool `json:"enabled"`
}

type PermissionCheckResult struct {
	Granted bool `json:"granted"`
}

type Fix struct {
	Type       string `json:"type"`
	API        string `json:"api,omitempty"`
	Role       string `json:"role,omitempty"`
	Permission string `json:"permission,omitempty"`
	ConsoleURL string `json:"consoleUrl"`
}

type ValidationResult struct {
	Valid       bool                             `json:"valid"`
	APIs        map[string]APICheckResult        `json:"apis"`
	Permissions map[string]PermissionCheckResult `json:"permissions"`
	Fixes       []Fix                            `json:"fixes"`
	CheckedAt   time.Time                        `json:"checkedAt"`
}

type ValidationService struct {
	projectID       string
	serviceUsage    ServiceUsageClient
	resourceManager ResourceManagerClient
	cache           *validationCache
}

func NewValidationService(cfg config.Config, su ServiceUsageClient, rm ResourceManagerClient) *ValidationService {
	return &ValidationService{
		projectID:       cfg.ProjectID,
		serviceUsage:    su,
		resourceManager: rm,
		cache:           newValidationCache(5 * time.Minute),
	}
}

func (v *ValidationService) Validate(ctx context.Context) (*ValidationResult, error) {
	if cached, ok := v.cache.Get(); ok {
		return cached, nil
	}

	result, err := v.runValidation(ctx)
	if err != nil {
		return nil, err
	}

	v.cache.Set(result)
	return result, nil
}

func (v *ValidationService) InvalidateCache() {
	v.cache.Invalidate()
}

func (v *ValidationService) runValidation(ctx context.Context) (*ValidationResult, error) {
	apis, apiErr := v.checkAPIs(ctx)
	perms, permErr := v.checkPermissions(ctx)

	if apiErr != nil {
		log.Printf("validation: API check failed: %v", apiErr)
	}
	if permErr != nil {
		return nil, fmt.Errorf("permission check failed: %w", permErr)
	}

	fixes := v.buildFixes(apis, perms)

	valid := len(fixes) == 0

	result := &ValidationResult{
		Valid:       valid,
		APIs:        apis,
		Permissions: perms,
		Fixes:       fixes,
		CheckedAt:   time.Now(),
	}

	return result, nil
}

func (v *ValidationService) checkAPIs(ctx context.Context) (map[string]APICheckResult, error) {
	results := make(map[string]APICheckResult)
	for _, api := range requiredAPIs {
		name := fmt.Sprintf("projects/%s/services/%s", v.projectID, api)
		svc, err := v.serviceUsage.GetService(ctx, name)
		if err != nil {
			log.Printf("validation: failed to check API %s: %v", api, err)
			results[api] = APICheckResult{Enabled: false}
			continue
		}
		results[api] = APICheckResult{Enabled: svc.State == "ENABLED"}
	}
	return results, nil
}

func (v *ValidationService) checkPermissions(ctx context.Context) (map[string]PermissionCheckResult, error) {
	granted, err := v.resourceManager.TestIamPermissions(ctx, v.projectID, requiredPermissions)
	if err != nil {
		return nil, err
	}

	grantedSet := make(map[string]bool, len(granted))
	for _, p := range granted {
		grantedSet[p] = true
	}

	results := make(map[string]PermissionCheckResult)
	for _, p := range requiredPermissions {
		results[p] = PermissionCheckResult{Granted: grantedSet[p]}
	}
	return results, nil
}

func (v *ValidationService) buildFixes(apis map[string]APICheckResult, perms map[string]PermissionCheckResult) []Fix {
	var fixes []Fix

	for api, result := range apis {
		if !result.Enabled {
			fixes = append(fixes, Fix{
				Type:       "enable_api",
				API:        api,
				ConsoleURL: fmt.Sprintf("https://console.cloud.google.com/apis/library/%s?project=%s", api, v.projectID),
			})
		}
	}

	for perm, result := range perms {
		if !result.Granted {
			role, ok := permissionToRole[perm]
			if !ok {
				role = defaultRole
			}
			fixes = append(fixes, Fix{
				Type:       "grant_role",
				Role:       role,
				Permission: perm,
				ConsoleURL: fmt.Sprintf("https://console.cloud.google.com/iam-admin/iam?project=%s", v.projectID),
			})
		}
	}

	return fixes
}
