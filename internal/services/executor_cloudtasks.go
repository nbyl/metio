package services

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	cloudtaskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/nbyl/metio/internal/db"
	"google.golang.org/protobuf/types/known/durationpb"
)

type cloudTasksExecutor struct {
	client           *cloudtasks.Client
	queuePath        string
	baseURL          string
	serviceAccount   string
	db               db.DB
	operationTimeout time.Duration
}

func NewCloudTasksExecutor(client *cloudtasks.Client, projectID, location, queueName, baseURL, serviceAccount string, dbConn db.DB, timeout time.Duration) OperationExecutor {
	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", projectID, location, queueName)
	return &cloudTasksExecutor{
		client:           client,
		queuePath:        queuePath,
		baseURL:          baseURL,
		serviceAccount:   serviceAccount,
		db:               dbConn,
		operationTimeout: timeout,
	}
}

func (e *cloudTasksExecutor) taskURL(serverID, opID string) string {
	return fmt.Sprintf("%s/tasks/provision/%s?opId=%s", e.baseURL, serverID, url.QueryEscape(opID))
}

func (e *cloudTasksExecutor) StartOperation(ctx context.Context, serverID string, opType db.ProvisioningOperation, initialOutputs map[string]string, _ func(context.Context, *db.ProvisioningStatus) error) error {
	existingStatus, err := e.db.GetProvisioningStatus(ctx, serverID)
	if err == nil && existingStatus != nil && existingStatus.State == db.ProvisioningStateInProgress {
		return fmt.Errorf(errMsgOperationInProgress, serverID)
	}

	now := time.Now()
	status := &db.ProvisioningStatus{
		ID:          fmt.Sprintf("%s-%d", serverID, now.Unix()),
		Operation:   opType,
		State:       db.ProvisioningStateInProgress,
		StartedAt:   now,
		CurrentStep: "enqueuing",
		Steps:       []db.ProvisioningStep{},
		Outputs:     initialOutputs,
	}
	if err := e.db.UpdateProvisioningStatus(ctx, serverID, status); err != nil {
		return fmt.Errorf("failed to save provisioning status: %w", err)
	}

	task := &cloudtaskspb.CreateTaskRequest{
		Parent: e.queuePath,
		Task: &cloudtaskspb.Task{
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					HttpMethod: cloudtaskspb.HttpMethod_POST,
					Url:        e.taskURL(serverID, status.ID),
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							ServiceAccountEmail: e.serviceAccount,
							Audience:            e.baseURL,
						},
					},
				},
			},
			DispatchDeadline: durationpb.New(e.operationTimeout),
		},
	}

	_, err = e.client.CreateTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to create cloud task: %w", err)
	}

	log.Printf("[%s] Enqueued Cloud Task for operation %s", serverID, opType.String())
	return nil
}

func (e *cloudTasksExecutor) CancelOperation(ctx context.Context, serverID string) error {
	status, err := e.db.GetProvisioningStatus(ctx, serverID)
	if err != nil {
		return fmt.Errorf(errMsgNoOperationInProgress, serverID)
	}
	if status == nil || status.State != db.ProvisioningStateInProgress {
		return fmt.Errorf(errMsgNoOperationInProgress, serverID)
	}

	status.State = db.ProvisioningStateFailed
	status.Error = "cancelled by user"
	if err := e.db.UpdateProvisioningStatus(ctx, serverID, status); err != nil {
		return fmt.Errorf("failed to cancel operation: %w", err)
	}
	return nil
}

func (e *cloudTasksExecutor) OperationTimeout() time.Duration {
	return e.operationTimeout
}
