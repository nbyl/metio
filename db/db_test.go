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

func (m *MockDB) GetStatus(ctx context.Context, instanceName string) (Status, error) {
	args := m.Called(ctx, instanceName)
	return args.Get(0).(Status), args.Error(1)
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

func TestGetStatus(t *testing.T) {
	mockDB := new(MockDB)
	expectedStatus := Status{
		Players:   Players{Current: 5, Max: 20},
		Timestamp: time.Now(),
	}

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(expectedStatus, nil)

	status, err := mockDB.GetStatus(context.Background(), "test-instance")
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, status)
	mockDB.AssertExpectations(t)
}

func TestGetStatusError(t *testing.T) {
	mockDB := new(MockDB)

	mockDB.On("GetStatus", mock.Anything, "test-instance").Return(Status{}, assert.AnError)

	_, err := mockDB.GetStatus(context.Background(), "test-instance")
	assert.Error(t, err)
	mockDB.AssertExpectations(t)
}
