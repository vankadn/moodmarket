// application/scheduler/deps.go
//
// Consumer-side interfaces for the services the schedulers orchestrate. The
// concrete *services.* types satisfy these structurally, so wiring in main.go
// is unchanged — but tests can substitute lightweight fakes, which the concrete
// types (with their own heavy dependency graphs) would otherwise prevent.
package scheduler

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// Recommender produces a daily recommendation for a user. Satisfied by
// *services.RecommendationService.
type Recommender interface {
	GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error)
}

// Investor places the trades for a recommendation. Satisfied by
// *services.InvestmentService.
type Investor interface {
	Execute(
		ctx context.Context,
		userID string,
		allocations []models.Allocation,
		totalAmount float64,
		riskLevel, summary, overallReasoning string,
		perAllocBrokerage map[string]string,
		configID string,
	) ([]models.TradeReceipt, string, error)
}

// DriftChecker reports tickers/asset-classes that have drifted past threshold.
// Satisfied by *services.RebalancingService.
type DriftChecker interface {
	CheckDrift(ctx context.Context, userID string) ([]models.TickerDrift, error)
}

// RebalanceRequestBuilder assembles the rebalance analysis input for a user.
// Satisfied by *services.RebalanceAggregationService.
type RebalanceRequestBuilder interface {
	BuildRequest(ctx context.Context, userID string) (*models.RebalanceRequest, error)
}
