// application/services/investment_service.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// InvestmentService orchestrates the full invest loop:
// load user's brokerage credentials → place orders → build decision record → persist → return receipts.
// It does not generate recommendations — that is RecommendationService's job.
type InvestmentService struct {
	brokerageFactory ports.BrokerageProviderFactory
	profileRepo      ports.ProfileRepository
	decisionRepo     ports.DecisionRepository
	marketData       ports.MarketDataProvider
}

func NewInvestmentService(
	brokerageFactory ports.BrokerageProviderFactory,
	profileRepo ports.ProfileRepository,
	decisionRepo ports.DecisionRepository,
	marketData ports.MarketDataProvider,
) *InvestmentService {
	return &InvestmentService{
		brokerageFactory: brokerageFactory,
		profileRepo:      profileRepo,
		decisionRepo:     decisionRepo,
		marketData:       marketData,
	}
}

// Execute places one market order per allocation, routes each allocation to the appropriate
// brokerage connection, saves the full decision record, and returns all receipts that succeeded.
// Partial receipts are returned on individual order failures.
// Returns ErrBrokerageNotConnected (wrapped) when the user has no brokerage account.
// perAllocBrokerage: map of ticker → connectionID for manual per-allocation overrides;
// nil or missing ticker falls back to asset-category auto-routing.
// configID: the AutoInvestConfig.ID that triggered this execution; "manual" for user-initiated invest.
func (s *InvestmentService) Execute(
	ctx context.Context,
	userID string,
	allocations []models.Allocation,
	totalAmount float64,
	riskLevel, summary string,
	perAllocBrokerage map[string]string,
	configID string,
) ([]models.TradeReceipt, string, error) {

	connections, err := s.profileRepo.GetBrokerageConnections(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("invest: load connections: %w", err)
	}
	if len(connections) == 0 {
		return nil, "", fmt.Errorf("invest: %w", ports.ErrBrokerageNotConnected)
	}

	// Fetch market snapshot for the decision record (daily cache hit after first call).
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("investment service: market data unavailable (%v) — proceeding without it", err)
		snapshot = nil
	}

	// Group allocations by connection ID.
	type group struct {
		conn        models.BrokerageConnection
		allocations []models.Allocation
	}
	groups := make(map[string]*group)
	connOrder := []string{} // preserve insertion order for stable logging

	for _, alloc := range allocations {
		var conn *models.BrokerageConnection

		// Per-allocation manual override takes priority over auto-routing.
		if overrideID := perAllocBrokerage[alloc.Ticker]; overrideID != "" {
			for i := range connections {
				if connections[i].ID == overrideID {
					conn = &connections[i]
					break
				}
			}
			if conn == nil {
				log.Printf("investment service: per-alloc override id %q for %s not found — falling back to routing", overrideID, alloc.Ticker)
			}
		}
		if conn == nil {
			conn = RouteAllocation(alloc, connections)
		}
		if conn == nil {
			log.Printf("investment service: no connection matched for %s (%s) — skipped", alloc.Ticker, alloc.Type)
			continue
		}
		if _, ok := groups[conn.ID]; !ok {
			groups[conn.ID] = &group{conn: *conn}
			connOrder = append(connOrder, conn.ID)
		}
		groups[conn.ID].allocations = append(groups[conn.ID].allocations, alloc)
	}

	receipts := make([]models.TradeReceipt, 0, len(allocations))
	for _, id := range connOrder {
		g := groups[id]
		brokerage, err := s.brokerageFactory.ForUser(&g.conn)
		if err != nil {
			log.Printf("investment service: build provider for connection %s failed (%v) — group skipped", id, err)
			continue
		}
		for _, alloc := range g.allocations {
			if alloc.Amount < 1.00 {
				log.Printf("[invest] skipping %s — notional %.2f below Alpaca $1.00 minimum", alloc.Ticker, alloc.Amount)
				continue
			}
			tradeOrder := models.TradeOrder{
				UserID: userID,
				Ticker: alloc.Ticker,
				Amount: alloc.Amount,
			}
			receipt, err := brokerage.PlaceMarketOrder(ctx, tradeOrder)
			if err != nil {
				log.Printf("investment service: order %s on %s failed (skipped): %v", alloc.Ticker, id, err)
				continue
			}
			receipt.BrokerageID = g.conn.ID
			receipt.BrokerageName = g.conn.Name
			receipts = append(receipts, *receipt)
		}
	}

	decision := &models.InvestmentDecision{
		UserID:         userID,
		ConfigID:       configID,
		Timestamp:      time.Now(),
		MarketSnapshot: snapshot,
		Allocations:    allocations,
		Receipts:       receipts,
		TotalAmount:    totalAmount,
		RiskLevel:      riskLevel,
		Summary:        summary,
		DecisionType:   "invest",
	}
	if err := s.decisionRepo.Save(ctx, decision); err != nil {
		log.Printf("investment service: save decision: %v", err)
	}

	return receipts, decision.ID, nil
}
