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
	// StampVerdict writes a verdict onto an existing decision.
	// Overwrites if the existing verdict has no ticker data (bad/incomplete stamp).
	StampVerdict(ctx context.Context, decisionID string, verdict *models.DecisionVerdict) error
	// ListUnverdicted returns decisions for the user with no verdict (or an empty verdict)
	// that are older than minAge.
	// Set minAge=0 to include all decisions regardless of age (useful for local testing).
	ListUnverdicted(ctx context.Context, userID string, minAge time.Duration) ([]models.InvestmentDecision, error)
	// GetUsersWithPendingVerdicts returns distinct userIDs with at least one decision
	// older than minAge that has no verdict stamped (or has an empty/zero verdict).
	// Set minAge=0 to include all decisions regardless of age (useful for local testing).
	GetUsersWithPendingVerdicts(ctx context.Context, minAge time.Duration) ([]string, error)
	// GetEvalSummary returns the aggregate verdict performance summary for the user.
	GetEvalSummary(ctx context.Context, userID string) (*models.EvalSummary, error)
	// ListVerdictedDecisions returns paginated decisions that have a verdict, newest first.
	// page is 1-based; limit is the page size.
	ListVerdictedDecisions(ctx context.Context, userID string, page, limit int) ([]models.InvestmentDecision, error)
}
