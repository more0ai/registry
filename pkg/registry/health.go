package registry

import (
	"context"
	"time"

	"github.com/morezero/capabilities-registry/pkg/db"
)

// Health checks the registry service health.
func (r *Registry) Health(ctx context.Context) *HealthOutput {
	dbOk := true

	if r.repo == nil {
		dbOk = false
	} else {
		// Simple DB check
		_, _, err := r.repo.ListCapabilities(ctx, db.ListCapabilitiesParams{Limit: 1})
		if err != nil {
			dbOk = false
		}
	}

	status := "healthy"
	if !dbOk {
		status = "unhealthy"
	}

	return &HealthOutput{
		Status: status,
		Checks: HealthChecks{
			Database: dbOk,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
