package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDB struct {
	mock.Mock
}

func (m *MockDB) UpdateStatus(ctx context.Context, instanceName string, status Status) error {
	args := m.Called(ctx, instanceName, status)
	return args.Error(0)
}

func TestUpdateStatus(t *testing.T) {
	mockDB := new(MockDB)
	status := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
	}

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", status).Return(nil)

	err := mockDB.UpdateStatus(context.Background(), "test-instance", status)
	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
}

func TestUpdateStatusError(t *testing.T) {
	mockDB := new(MockDB)
	status := Status{
		Players:   Players{Current: 0, Max: 10},
		Timestamp: time.Now(),
	}

	mockDB.On("UpdateStatus", mock.Anything, "test-instance", status).Return(assert.AnError)

	err := mockDB.UpdateStatus(context.Background(), "test-instance", status)
	assert.Error(t, err)
	mockDB.AssertExpectations(t)
}
