package servers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/nbyl/metio/internal/db"
	"github.com/nbyl/metio/internal/pulumi/programs"
)

type ProvisioningServiceInterface interface {
	CreateServer(ctx context.Context, serverID string, config *programs.ServerConfig) error
	UpdateServer(ctx context.Context, serverID string, config *programs.ServerConfig, updateType int) error
	DestroyServer(ctx context.Context, serverID string) error
	GetProvisioningStatus(ctx context.Context, serverID string) (*db.ProvisioningStatus, error)
	RevertServerConfig(ctx context.Context, serverID string) error
}

type ShutdownScheduleInput struct {
	Enabled  bool   `json:"enabled"`
	Time     string `json:"time,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type CreateServerRequest struct {
	Name             string                 `json:"name"`
	Region           string                 `json:"region"`
	Zone             string                 `json:"zone"`
	MachineType      string                 `json:"machineType"`
	MinecraftVersion string                 `json:"minecraftVersion"`
	DiskSizeGB       int                    `json:"diskSizeGB,omitempty"`
	ShutdownSchedule *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
	ExistingAddress  string                 `json:"existingAddress,omitempty"`
}

type UpdateServerRequest struct {
	Name             *string                `json:"name,omitempty"`
	Region           *string                `json:"region,omitempty"`
	Zone             *string                `json:"zone,omitempty"`
	MachineType      *string                `json:"machineType,omitempty"`
	MinecraftVersion *string                `json:"minecraftVersion,omitempty"`
	DiskSizeGB       *int                   `json:"diskSizeGB,omitempty"`
	ShutdownSchedule *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
}

type ServerConfigJSON struct {
	Name                        string                 `json:"name"`
	Region                      string                 `json:"region"`
	Zone                        string                 `json:"zone"`
	MachineType                 string                 `json:"machineType"`
	MinecraftVersion            string                 `json:"minecraftVersion"`
	DiskSizeGB                  int                    `json:"diskSizeGB"`
	InfraVersion                int                    `json:"infraVersion,omitempty"`
	DeployedByControllerVersion string                 `json:"deployedByControllerVersion,omitempty"`
	MachineAgentImage           string                 `json:"machineAgentImage,omitempty"`
	ShutdownSchedule            *ShutdownScheduleInput `json:"shutdownSchedule,omitempty"`
	CreatedAt                   string                 `json:"createdAt"`
	UpdatedAt                   string                 `json:"updatedAt"`
}

type StatusResponse struct {
	Players           PlayersJSON `json:"players"`
	Timestamp         string      `json:"timestamp"`
	Uptime            string      `json:"uptime"`
	ServerState       string      `json:"serverState"`
	InstanceIP        string      `json:"instanceIP"`
	Version           string      `json:"version"`
	WhitelistEnabled  bool        `json:"whitelistEnabled"`
	ScheduledShutdown *string     `json:"scheduledShutdown,omitempty"`
	AgentVersion      string      `json:"agentVersion,omitempty"`
}

type PlayersJSON struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

type ServerResponse struct {
	ID                   string           `json:"id"`
	Config               ServerConfigJSON `json:"config"`
	Status               *StatusResponse  `json:"status,omitempty"`
	CurrentInfraVersion  int              `json:"currentInfraVersion"`
	Outdated             bool             `json:"outdated"`
	OutdatedMachineAgent bool             `json:"outdatedMachineAgent"`
	ControllerVersion    string           `json:"controllerVersion,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type ServerActionResponse struct {
	Success bool           `json:"success"`
	State   db.ServerState `json:"state"`
}

type ScheduleShutdownRequest struct {
	ShutdownTime string `json:"shutdownTime"`
}

type ScheduleShutdownResponse struct {
	Success           bool    `json:"success"`
	ScheduledShutdown *string `json:"scheduledShutdown,omitempty"`
}

type WhitelistResponse struct {
	Enabled bool              `json:"enabled"`
	Players []WhitelistPlayer `json:"players"`
}

type WhitelistPlayer struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	AddedAt  string `json:"addedAt"`
	AddedBy  string `json:"addedBy"`
}

type AddPlayerRequest struct {
	Username string `json:"username"`
}

type SetEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

type ProvisioningStatusResponse struct {
	ID          string            `json:"id"`
	Operation   string            `json:"operation"`
	State       string            `json:"state"`
	CurrentStep string            `json:"currentStep"`
	Progress    int               `json:"progress"`
	Steps       []StepResponse    `json:"steps"`
	Error       string            `json:"error,omitempty"`
	StartedAt   string            `json:"startedAt"`
	CompletedAt *string           `json:"completedAt,omitempty"`
	Outputs     map[string]string `json:"outputs,omitempty"`
}

type StepResponse struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
}

type ComputeClient interface {
	Start(ctx context.Context, req *computepb.StartInstanceRequest) error
	Stop(ctx context.Context, req *computepb.StopInstanceRequest) error
	Close() error
}

type gcpComputeClient struct {
	client *compute.InstancesClient
}

func (g *gcpComputeClient) Start(ctx context.Context, req *computepb.StartInstanceRequest) error {
	_, err := g.client.Start(ctx, req)
	return err
}

func (g *gcpComputeClient) Stop(ctx context.Context, req *computepb.StopInstanceRequest) error {
	_, err := g.client.Stop(ctx, req)
	return err
}

func (g *gcpComputeClient) Close() error {
	return g.client.Close()
}

var NewComputeClient = func(ctx context.Context) (ComputeClient, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcpComputeClient{client: c}, nil
}

func shutdownScheduleToInput(s *db.ShutdownSchedule) *ShutdownScheduleInput {
	if s == nil {
		return nil
	}
	return &ShutdownScheduleInput{
		Enabled:  s.Enabled,
		Time:     s.Time,
		Timezone: s.Timezone,
	}
}

func shutdownScheduleFromInput(s *ShutdownScheduleInput) *db.ShutdownSchedule {
	if s == nil {
		return nil
	}
	return &db.ShutdownSchedule{
		Enabled:  s.Enabled,
		Time:     s.Time,
		Timezone: s.Timezone,
	}
}

func serverConfigToJSON(cfg *db.ServerConfig) ServerConfigJSON {
	return ServerConfigJSON{
		Name:                        cfg.Name,
		Region:                      cfg.Region,
		Zone:                        cfg.Zone,
		MachineType:                 cfg.MachineType,
		MinecraftVersion:            cfg.MinecraftVersion,
		DiskSizeGB:                  cfg.DiskSizeGB,
		InfraVersion:                cfg.InfraVersion,
		DeployedByControllerVersion: cfg.DeployedByControllerVersion,
		MachineAgentImage:           cfg.MachineAgentImage,
		ShutdownSchedule:            shutdownScheduleToInput(cfg.ShutdownSchedule),
		CreatedAt:                   cfg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                   cfg.UpdatedAt.Format(time.RFC3339),
	}
}

func toProvisioningStatusResponse(status *db.ProvisioningStatus) ProvisioningStatusResponse {
	steps := make([]StepResponse, len(status.Steps))
	for i, step := range status.Steps {
		steps[i] = StepResponse{
			Name:      step.Name,
			Status:    step.Status.String(),
			Message:   step.Message,
			Timestamp: step.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	var completedAt *string
	if status.CompletedAt != nil {
		formatted := status.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		completedAt = &formatted
	}

	return ProvisioningStatusResponse{
		ID:          status.ID,
		Operation:   status.Operation.String(),
		State:       status.State.String(),
		CurrentStep: status.CurrentStep,
		Progress:    calculateProgress(status),
		Steps:       steps,
		Error:       status.Error,
		StartedAt:   status.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		CompletedAt: completedAt,
		Outputs:     status.Outputs,
	}
}

func calculateProgress(status *db.ProvisioningStatus) int {
	if len(status.Steps) == 0 {
		return 0
	}

	if status.State == db.ProvisioningStateCompleted {
		return 100
	}

	completed := 0
	for _, step := range status.Steps {
		if step.Status == db.ProvisioningStateCompleted {
			completed++
		}
	}

	return (completed * 100) / len(status.Steps)
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
