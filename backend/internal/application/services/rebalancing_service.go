// application/services/rebalancing_service.go
package services

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// RebalancingService compares a user's current brokerage positions against the
// target weights from their most recent InvestmentDecision and returns any tickers
// that have drifted past the configured threshold.
type RebalancingService struct {
	decisionRepo     ports.DecisionRepository
	profileRepo      ports.ProfileRepository
	brokerageFactory ports.BrokerageProviderFactory
	threshold        float64 // percentage-point drift required to trigger an alert
}

func NewRebalancingService(
	decisionRepo ports.DecisionRepository,
	profileRepo ports.ProfileRepository,
	brokerageFactory ports.BrokerageProviderFactory,
) *RebalancingService {
	threshold := 10.0
	if raw := os.Getenv("REBALANCE_DRIFT_THRESHOLD"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			threshold = v
		}
	}
	return &RebalancingService{
		decisionRepo:     decisionRepo,
		profileRepo:      profileRepo,
		brokerageFactory: brokerageFactory,
		threshold:        threshold,
	}
}

// CheckDrift returns tickers whose actual portfolio weight has drifted past the threshold
// compared to the target weights in the user's most recent InvestmentDecision.
// Returns nil, nil when there is no prior decision or no brokerage connection.
func (s *RebalancingService) CheckDrift(ctx context.Context, userID string) ([]models.TickerDrift, error) {
	decisions, err := s.decisionRepo.ListByUser(ctx, userID, 1)
	if err != nil {
		return nil, fmt.Errorf("rebalancing: list decisions: %w", err)
	}
	if len(decisions) == 0 || len(decisions[0].Allocations) == 0 {
		return nil, nil
	}
	last := decisions[0]

	conns, err := s.profileRepo.GetBrokerageConnections(ctx, userID)
	if err != nil || len(conns) == 0 {
		return nil, nil
	}

	provider, err := s.brokerageFactory.ForUser(&conns[0])
	if err != nil {
		return nil, fmt.Errorf("rebalancing: build provider: %w", err)
	}

	positions, err := provider.GetPositions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("rebalancing: get positions: %w", err)
	}
	if len(positions) == 0 {
		return nil, nil
	}

	// Build target weights. If Percentage is unset (legacy decisions), derive from Amount/Total.
	var totalAllocated float64
	for _, a := range last.Allocations {
		totalAllocated += a.Amount
	}
	targetPct := make(map[string]float64, len(last.Allocations))
	for _, a := range last.Allocations {
		pct := a.Percentage
		if pct == 0 && totalAllocated > 0 {
			pct = (a.Amount / totalAllocated) * 100
		}
		targetPct[a.Ticker] = pct
	}

	// Compute actual weights from live market values.
	var totalValue float64
	for _, p := range positions {
		totalValue += p.MarketValue
	}
	if totalValue <= 0 {
		return nil, nil
	}
	actualPct := make(map[string]float64, len(positions))
	for _, p := range positions {
		actualPct[p.Ticker] = (p.MarketValue / totalValue) * 100
	}

	// Return tickers whose drift exceeds the threshold.
	var drifts []models.TickerDrift
	for ticker, target := range targetPct {
		actual := actualPct[ticker]
		drift := actual - target
		if math.Abs(drift) >= s.threshold {
			drifts = append(drifts, models.TickerDrift{
				Ticker:    ticker,
				TargetPct: target,
				ActualPct: actual,
				DriftPct:  drift,
			})
		}
	}
	return drifts, nil
}
