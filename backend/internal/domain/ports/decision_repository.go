// domain/ports/decision_repository.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// DecisionRepository is the port any persistence layer must implement for decision logging.
type DecisionRepository interface {
	Save(ctx context.Context, decision *models.InvestmentDecision) error
	ListByUser(ctx context.Context, userID string, limit int) ([]models.InvestmentDecision, error)
}
