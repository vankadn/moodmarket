// domain/ports/scheduler_repository.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// SchedulerRepository persists one audit record per autonomous cycle run.
type SchedulerRepository interface {
	Save(ctx context.Context, run *models.SchedulerRun) error
}
