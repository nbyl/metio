package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloudTasksExecutor_TaskURLIncludesOperationID(t *testing.T) {
	e := &cloudTasksExecutor{baseURL: "https://controller.example.com"}

	url := e.taskURL("server-1", "server-1-1700000000")

	assert.Equal(t, "https://controller.example.com/tasks/provision/server-1?opId=server-1-1700000000", url)
}
