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

// CheckDrift returns positions/asset-classes whose actual portfolio weight has drifted
// past the threshold. Two checks are combined:
//  1. Ticker-level drift vs Claude's last recommendation targets.
//  2. Asset-class drift vs the user's AllocationPreferences (when set) — these check
//     hard limits the user defined, e.g. "Crypto max 20%".
//
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

	// Compute actual weights from live market values.
	var totalValue float64
	for _, p := range positions {
		totalValue += p.MarketValue
	}
	if totalValue <= 0 {
		return nil, nil
	}
	actualTickerPct := make(map[string]float64, len(positions))
	for _, p := range positions {
		actualTickerPct[p.Ticker] = (p.MarketValue / totalValue) * 100
	}

	// Build target weights from last decision. If Percentage unset, derive from Amount/Total.
	var totalAllocated float64
	for _, a := range last.Allocations {
		totalAllocated += a.Amount
	}
	targetPct := make(map[string]float64, len(last.Allocations))
	tickerAssetClass := make(map[string]string, len(last.Allocations))
	for _, a := range last.Allocations {
		pct := a.Percentage
		if pct == 0 && totalAllocated > 0 {
			pct = (a.Amount / totalAllocated) * 100
		}
		targetPct[a.Ticker] = pct
		tickerAssetClass[a.Ticker] = a.AssetClass
	}

	var drifts []models.TickerDrift

	// 1. Ticker-level drift vs Claude's last recommendation.
	for ticker, target := range targetPct {
		actual := actualTickerPct[ticker]
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

	// 2. Asset-class drift vs user's AllocationPreferences (when configured).
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err == nil && profile != nil && profile.AllocationPreferences != nil && len(profile.AllocationPreferences.AssetClassLimits) > 0 {
		// Aggregate actual weights by asset class using the last decision's asset_class mapping.
		actualByClass := make(map[string]float64)
		for ticker, pct := range actualTickerPct {
			if class, ok := tickerAssetClass[ticker]; ok && class != "" {
				actualByClass[class] += pct
			}
		}

		for _, limit := range profile.AllocationPreferences.AssetClassLimits {
			actual := actualByClass[limit.AssetClass]
			if limit.MaxPct > 0 && actual > limit.MaxPct {
				drifts = append(drifts, models.TickerDrift{
					Ticker:    limit.AssetClass + " (asset class)",
					TargetPct: limit.MaxPct,
					ActualPct: actual,
					DriftPct:  actual - limit.MaxPct,
				})
			} else if limit.MinPct > 0 && actual < limit.MinPct {
				drifts = append(drifts, models.TickerDrift{
					Ticker:    limit.AssetClass + " (asset class)",
					TargetPct: limit.MinPct,
					ActualPct: actual,
					DriftPct:  actual - limit.MinPct,
				})
			}
		}
	}

	return drifts, nil
}
