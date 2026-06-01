package services

import (
	"context"

	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/serviceusage/v1"
)

type ServiceUsageClient interface {
	GetService(ctx context.Context, name string) (*serviceusage.GoogleApiServiceusageV1Service, error)
	Close() error
}

type ResourceManagerClient interface {
	TestIamPermissions(ctx context.Context, projectID string, permissions []string) ([]string, error)
	Close() error
}

type serviceUsageAdapter struct {
	svc *serviceusage.Service
}

func NewServiceUsageAdapter(svc *serviceusage.Service) ServiceUsageClient {
	return &serviceUsageAdapter{svc: svc}
}

func (a *serviceUsageAdapter) GetService(ctx context.Context, name string) (*serviceusage.GoogleApiServiceusageV1Service, error) {
	return a.svc.Services.Get(name).Context(ctx).Do()
}

func (a *serviceUsageAdapter) Close() error {
	return nil
}

type resourceManagerAdapter struct {
	svc *cloudresourcemanager.Service
}

func NewResourceManagerAdapter(svc *cloudresourcemanager.Service) ResourceManagerClient {
	return &resourceManagerAdapter{svc: svc}
}

func (a *resourceManagerAdapter) TestIamPermissions(ctx context.Context, projectID string, permissions []string) ([]string, error) {
	req := &cloudresourcemanager.TestIamPermissionsRequest{
		Permissions: permissions,
	}
	resp, err := a.svc.Projects.TestIamPermissions("projects/"+projectID, req).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return resp.Permissions, nil
}

func (a *resourceManagerAdapter) Close() error {
	return nil
}
