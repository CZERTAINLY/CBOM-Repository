package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockHealthChecker is a mock implementation of the StorageHealthChecker interface
type mockStorageHealthChecker struct {
	shouldFail bool
	err        error
}

func (m *mockStorageHealthChecker) HealthCheck(ctx context.Context) error {
	if m.shouldFail {
		return m.err
	}
	return nil
}

func TestStorageChecker(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{shouldFail: false}
		checker := NewStorageChecker(mockStore)
		
		result := checker.Check(context.Background())
		
		assert.Equal(t, StatusUp, result.Status)
		assert.NotNil(t, result.Details)
		assert.Contains(t, result.Details, "latencyMs")
	})

	t.Run("failure", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{
			shouldFail: true,
			err:        errors.New("connection failed"),
		}
		checker := NewStorageChecker(mockStore)
		
		result := checker.Check(context.Background())
		
		assert.Equal(t, StatusDown, result.Status)
		assert.NotNil(t, result.Details)
		assert.Contains(t, result.Details, "error")
		assert.Contains(t, result.Details, "latencyMs")
		assert.Equal(t, "connection failed", result.Details["error"])
	})

	t.Run("name", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{shouldFail: false}
		checker := NewStorageChecker(mockStore)
		
		assert.Equal(t, "storage", checker.Name())
	})
}

func TestHealthService(t *testing.T) {
	t.Run("check_health_all_up", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{shouldFail: false}
		checker := NewStorageChecker(mockStore)
		svc := NewService(checker)
		
		result := svc.CheckHealth(context.Background())
		
		assert.Equal(t, StatusUp, result.Status)
		assert.NotNil(t, result.Components)
		assert.Contains(t, result.Components, "liveness")
		assert.Contains(t, result.Components, "readiness")
		assert.Contains(t, result.Components, "storage")
		assert.Equal(t, StatusUp, result.Components["liveness"].Status)
		assert.Equal(t, StatusUp, result.Components["readiness"].Status)
		assert.Equal(t, StatusUp, result.Components["storage"].Status)
	})

	t.Run("check_health_storage_down", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{
			shouldFail: true,
			err:        errors.New("storage failed"),
		}
		checker := NewStorageChecker(mockStore)
		svc := NewService(checker)
		
		result := svc.CheckHealth(context.Background())
		
		// With storage down, overall status should be DEGRADED (not DOWN)
		// because liveness and readiness are still UP
		assert.Equal(t, StatusDegraded, result.Status)
		assert.NotNil(t, result.Components)
		assert.Equal(t, StatusUp, result.Components["liveness"].Status)
		assert.Equal(t, StatusUp, result.Components["readiness"].Status)
		assert.Equal(t, StatusDown, result.Components["storage"].Status)
	})

	t.Run("check_liveness", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{shouldFail: false}
		checker := NewStorageChecker(mockStore)
		svc := NewService(checker)
		
		result := svc.CheckLiveness(context.Background())
		
		assert.Equal(t, StatusUp, result.Status)
		assert.NotNil(t, result.Components)
		assert.Contains(t, result.Components, "liveness")
		assert.Equal(t, StatusUp, result.Components["liveness"].Status)
		// Liveness should only include liveness component
		assert.Len(t, result.Components, 1)
	})

	t.Run("check_readiness_all_up", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{shouldFail: false}
		checker := NewStorageChecker(mockStore)
		svc := NewService(checker)
		
		result := svc.CheckReadiness(context.Background())
		
		assert.Equal(t, StatusUp, result.Status)
		assert.NotNil(t, result.Components)
		assert.Contains(t, result.Components, "readiness")
		assert.Equal(t, StatusUp, result.Components["readiness"].Status)
	})

	t.Run("check_readiness_storage_down", func(t *testing.T) {
		mockStore := &mockStorageHealthChecker{
			shouldFail: true,
			err:        errors.New("storage failed"),
		}
		checker := NewStorageChecker(mockStore)
		svc := NewService(checker)
		
		result := svc.CheckReadiness(context.Background())
		
		assert.Equal(t, StatusOutOfService, result.Status)
		assert.NotNil(t, result.Components)
		assert.Contains(t, result.Components, "readiness")
		assert.Equal(t, StatusOutOfService, result.Components["readiness"].Status)
	})
}

func TestCalculateOverallStatus(t *testing.T) {
	tests := []struct {
		name       string
		components map[string]Component
		expected   Status
	}{
		{
			name: "all_up",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusUp},
			},
			expected: StatusUp,
		},
		{
			name: "liveness_down",
			components: map[string]Component{
				"liveness":  {Status: StatusDown},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusUp},
			},
			expected: StatusDown,
		},
		{
			name: "readiness_down",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusDown},
				"storage":   {Status: StatusUp},
			},
			expected: StatusDown,
		},
		{
			name: "storage_down",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusDown},
			},
			expected: StatusDegraded,
		},
		{
			name: "storage_unknown",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusUnknown},
			},
			expected: StatusUnknown,
		},
		{
			name: "storage_degraded",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusDegraded},
			},
			expected: StatusDegraded,
		},
		{
			name: "storage_out_of_service",
			components: map[string]Component{
				"liveness":  {Status: StatusUp},
				"readiness": {Status: StatusUp},
				"storage":   {Status: StatusOutOfService},
			},
			expected: StatusDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateOverallStatus(tt.components)
			assert.Equal(t, tt.expected, result)
		})
	}
}
