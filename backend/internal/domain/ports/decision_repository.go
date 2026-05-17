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
	// ActivityByStrategy aggregates decisions for the user grouped by config_id.
	// Results are sorted by last_run_at descending.
	ActivityByStrategy(ctx context.Context, userID string) ([]models.StrategyActivity, error)
	// CostBasisByStrategy returns the total amount invested per ticker per config_id.
	// Outer key: config_id ("manual", "", or ObjectID hex). Inner key: ticker symbol.
	CostBasisByStrategy(ctx context.Context, userID string) (map[string]map[string]float64, error)
}
