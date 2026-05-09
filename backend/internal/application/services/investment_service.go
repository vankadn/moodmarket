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

// Execute places one market order per allocation, saves the full decision record,
// and returns all receipts that succeeded. If individual orders fail, execution
// continues for remaining tickers — partial receipts are returned, never a hard error.
// Returns ErrBrokerageNotConnected (wrapped) when the user has no brokerage account.
func (s *InvestmentService) Execute(
	ctx context.Context,
	userID string,
	allocations []models.Allocation,
	totalAmount float64,
	riskLevel, summary string,
) ([]models.TradeReceipt, string, error) {

	conn, _ := s.profileRepo.GetBrokerageConnection(ctx, userID)
	brokerage, err := s.brokerageFactory.ForUser(conn)
	if err != nil {
		return nil, "", fmt.Errorf("invest: %w", err) // wraps ErrBrokerageNotConnected
	}

	// Fetch market snapshot for the decision record (daily cache hit after first call).
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("investment service: market data unavailable (%v) — proceeding without it", err)
		snapshot = nil
	}

	receipts := make([]models.TradeReceipt, 0, len(allocations))
	for _, alloc := range allocations {
		order := models.TradeOrder{
			UserID: userID,
			Ticker: alloc.Ticker,
			Amount: alloc.Amount,
		}
		receipt, err := brokerage.PlaceMarketOrder(ctx, order)
		if err != nil {
			log.Printf("investment service: order %s failed (skipped): %v", alloc.Ticker, err)
			continue
		}
		receipts = append(receipts, *receipt)
	}

	decision := &models.InvestmentDecision{
		UserID:         userID,
		Timestamp:      time.Now(),
		MarketSnapshot: snapshot,
		Allocations:    allocations,
		Receipts:       receipts,
		TotalAmount:    totalAmount,
		RiskLevel:      riskLevel,
		Summary:        summary,
	}
	if err := s.decisionRepo.Save(ctx, decision); err != nil {
		log.Printf("investment service: save decision: %v", err)
	}

	return receipts, decision.ID, nil
}
