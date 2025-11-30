package health

import (
	"context"
	"time"
)

// Status represents the health status of a component or the overall system
type Status string

const (
	StatusUp           Status = "UP"
	StatusDown         Status = "DOWN"
	StatusOutOfService Status = "OUT_OF_SERVICE"
	StatusUnknown      Status = "UNKNOWN"
	StatusDegraded     Status = "DEGRADED"
)

// Component represents the health status of a single component
type Component struct {
	Status  Status         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// Health represents the overall health response
type Health struct {
	Status     Status               `json:"status"`
	Components map[string]Component `json:"components"`
}

// Checker is an interface for checking the health of a component
type Checker interface {
	Check(ctx context.Context) Component
	Name() string
}

// Service aggregates health checks from multiple components
type Service struct {
	checkers []Checker
}

// NewService creates a new health service with the given checkers
func NewService(checkers ...Checker) Service {
	return Service{
		checkers: checkers,
	}
}

// CheckHealth performs all health checks and returns the overall health status
func (s Service) CheckHealth(ctx context.Context) Health {
	components := make(map[string]Component)

	// Always include liveness and readiness
	components["liveness"] = Component{Status: StatusUp}
	components["readiness"] = Component{Status: StatusUp}

	// Run all registered checkers
	for _, checker := range s.checkers {
		components[checker.Name()] = checker.Check(ctx)
	}

	// Calculate overall status
	overallStatus := calculateOverallStatus(components)

	return Health{
		Status:     overallStatus,
		Components: components,
	}
}

// CheckLiveness returns liveness probe status
func (s Service) CheckLiveness(ctx context.Context) Health {
	return Health{
		Status: StatusUp,
		Components: map[string]Component{
			"liveness": {Status: StatusUp},
		},
	}
}

// CheckReadiness returns readiness probe status
func (s Service) CheckReadiness(ctx context.Context) Health {
	components := make(map[string]Component)

	// Run all registered checkers
	for _, checker := range s.checkers {
		components[checker.Name()] = checker.Check(ctx)
	}

	// Check if any critical components are down
	ready := true
	for _, comp := range components {
		if comp.Status == StatusDown || comp.Status == StatusOutOfService {
			ready = false
			break
		}
	}

	status := StatusUp
	if !ready {
		status = StatusOutOfService
	}

	return Health{
		Status: status,
		Components: map[string]Component{
			"readiness": {Status: status},
		},
	}
}

// calculateOverallStatus determines the overall health status based on component statuses
// Severity order: UP < DEGRADED < UNKNOWN < OUT_OF_SERVICE < DOWN
func calculateOverallStatus(components map[string]Component) Status {
	liveness, hasLiveness := components["liveness"]
	readiness, hasReadiness := components["readiness"]

	// Rule 1: If liveness.status != UP → overall status = DOWN
	if hasLiveness && liveness.Status != StatusUp {
		return StatusDown
	}

	// Rule 2: If readiness.status != UP → overall status = DOWN
	if hasReadiness && readiness.Status != StatusUp {
		return StatusDown
	}

	// Rule 3: Compute the worst status among all remaining components
	worstStatus := StatusUp

	for name, comp := range components {
		// Skip liveness and readiness as they are already checked
		if name == "liveness" || name == "readiness" {
			continue
		}

		switch comp.Status {
		case StatusDown, StatusOutOfService:
			// Any DOWN/OUT_OF_SERVICE → overall DEGRADED
			if worstStatus == StatusUp {
				worstStatus = StatusDegraded
			}
		case StatusUnknown:
			if worstStatus == StatusUp {
				worstStatus = StatusUnknown
			}
		case StatusDegraded:
			if worstStatus == StatusUp {
				worstStatus = StatusDegraded
			}
		}
	}

	return worstStatus
}

// StorageChecker checks the health of the storage backend
type StorageChecker struct {
	store StorageHealthChecker
}

// StorageHealthChecker is an interface for checking storage health
type StorageHealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// NewStorageChecker creates a new storage health checker
func NewStorageChecker(store StorageHealthChecker) StorageChecker {
	return StorageChecker{store: store}
}

// Check performs the storage health check
func (c StorageChecker) Check(ctx context.Context) Component {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	startTime := time.Now()
	err := c.store.HealthCheck(ctx)
	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		return Component{
			Status: StatusDown,
			Details: map[string]any{
				"error":     err.Error(),
				"latencyMs": latency,
			},
		}
	}

	return Component{
		Status: StatusUp,
		Details: map[string]any{
			"latencyMs": latency,
		},
	}
}

// Name returns the name of this checker
func (c StorageChecker) Name() string {
	return "storage"
}
