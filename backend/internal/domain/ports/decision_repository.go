// domain/ports/decision_repository.go
package ports

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// DecisionRepository is the port any persistence layer must implement for decision logging.
type DecisionRepository interface {
	Save(ctx context.Context, decision *models.InvestmentDecision) error
	ListByUser(ctx context.Context, userID string, limit int) ([]models.InvestmentDecision, error)
	// ListByUserSince returns all decisions for the user on or after since. Nil since means all time.
	ListByUserSince(ctx context.Context, userID string, since *time.Time) ([]models.InvestmentDecision, error)
}
