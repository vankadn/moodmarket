// application/services/recommendation_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type rebalanceRequestBuilder interface {
	BuildRequest(ctx context.Context, userID string) (*models.RebalanceRequest, error)
}

type RecommendationService struct {
	advisor              ports.InvestmentAdvisor
	profileRepo          ports.ProfileRepository
	marketData           ports.MarketDataProvider
	decisionRepo         ports.DecisionRepository
	financialData        ports.FinancialDataProvider
	brokerageFactory     ports.BrokerageProviderFactory
	documentRepo         ports.DocumentRepository
	portfolioAggregator  ports.PortfolioAggregator
	rebalanceRepo        ports.RebalanceRepository
	rebalanceAggregator  rebalanceRequestBuilder
	rebalanceAdvisor     ports.RebalanceAdvisor
	critic               ports.RecommendationCritic
	notifications        ports.NotificationProvider
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
	brokerageFactory ports.BrokerageProviderFactory,
	documentRepo ports.DocumentRepository,
	portfolioAggregator ports.PortfolioAggregator,
	rebalanceRepo ports.RebalanceRepository,
	rebalanceAggregator rebalanceRequestBuilder,
	rebalanceAdvisor ports.RebalanceAdvisor,
	critic ports.RecommendationCritic,
	notifications ports.NotificationProvider,
) *RecommendationService {
	return &RecommendationService{
		advisor:             advisor,
		profileRepo:         profileRepo,
		marketData:          marketData,
		decisionRepo:        decisionRepo,
		financialData:       financialData,
		brokerageFactory:    brokerageFactory,
		documentRepo:        documentRepo,
		portfolioAggregator: portfolioAggregator,
		rebalanceRepo:       rebalanceRepo,
		rebalanceAggregator: rebalanceAggregator,
		rebalanceAdvisor:    rebalanceAdvisor,
		critic:              critic,
		notifications:       notifications,
	}
}

// GetDailyRecommendation fetches the user's profile, today's market snapshot, and live Plaid
// balances (if accounts are connected), then asks the advisor for an allocation and logs the decision.
// Market data and Plaid failures are both non-fatal — the recommendation is returned regardless.
func (s *RecommendationService) GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error) {
	// Step 1: user profile
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ports.ErrProfileNotFound) {
		return nil, fmt.Errorf("recommendation service: fetch profile: %w", err)
	}

	// Step 2: market snapshot
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("[recommend] market data unavailable (%v) — skipped", err)
		snapshot = nil
	} else if snapshot != nil {
		log.Printf("[recommend] market snapshot: SPY %+.2f%% QQQ %+.2f%% sentiment=%s", snapshot.SPYChangePercent, snapshot.QQQChangePercent, snapshot.MarketSentiment)
	}

	// Step 3: Plaid bank balances
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("[recommend] plaid connections fetch failed (%v) — skipped", err)
	} else if len(connections) > 0 {
		summary, err := s.financialData.GetBalanceSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] balance fetch failed (%v) — skipped", err)
		} else {
			req.BalanceSummary = &summary
			log.Printf("[recommend] balances loaded (%d accounts)", summary.AccountCount)
		}

		// Step 4: Plaid transaction history — Claude can flag large pending charges
		txSummary, err := s.financialData.GetTransactionSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] transaction fetch failed (%v) — skipped", err)
		} else {
			req.TransactionSummary = &txSummary
			log.Printf("[recommend] transactions loaded")
		}
	}

	// Steps 5 & 5b: current positions — Alpaca connections first, then external aggregator.
	// seen deduplicates by ticker across both sources (first-seen wins).
	seen := make(map[string]bool)

	brokerageConns, _ := s.profileRepo.GetBrokerageConnections(ctx, userID)
	for i := range brokerageConns {
		provider, err := s.brokerageFactory.ForUser(&brokerageConns[i])
		if err != nil {
			log.Printf("[recommend] build provider for %s failed (%v) — skipped", brokerageConns[i].ID, err)
			continue
		}
		positions, err := provider.GetPositions(ctx, userID)
		if err != nil {
			log.Printf("[recommend] positions fetch from %s failed (%v) — skipped", brokerageConns[i].ID, err)
			continue
		}
		for _, p := range positions {
			if seen[p.Ticker] {
				continue
			}
			seen[p.Ticker] = true
			req.Positions = append(req.Positions, p)
		}
	}
	if len(brokerageConns) > 0 {
		log.Printf("[recommend] %d position(s) loaded across %d brokerage connection(s)", len(req.Positions), len(brokerageConns))
	}

	// Step 5b: external portfolio holdings merged with Alpaca positions; ticker deduplication continues.
	portfolioConn, _ := s.profileRepo.GetPortfolioConnection(ctx, userID)
	if portfolioConn != nil {
		extPositions, err := s.portfolioAggregator.GetHoldings(ctx, portfolioConn.ProviderUserID, portfolioConn.ProviderUserSecret)
		if err != nil {
			log.Printf("[recommend] external holdings fetch failed (%v) — skipped", err)
		} else {
			added := 0
			for _, p := range extPositions {
				if seen[p.Ticker] {
					continue
				}
				seen[p.Ticker] = true
				req.Positions = append(req.Positions, p)
				added++
			}
			log.Printf("[recommend] %d external position(s) merged (provider=%s), total=%d", added, portfolioConn.Provider, len(req.Positions))
		}
	}

	// Step 6: recent decision history — Claude avoids repeating the same allocation daily
	recentDecisions, err := s.decisionRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		log.Printf("[recommend] recent decisions fetch failed (%v) — skipped", err)
	} else {
		req.RecentDecisions = recentDecisions
		log.Printf("[recommend] %d recent decision(s) loaded", len(recentDecisions))
		var recentBlocked []models.InvestmentDecision
		for _, d := range recentDecisions {
			if d.DecisionType == "blocked" {
				recentBlocked = append(recentBlocked, d)
				if len(recentBlocked) == 3 {
					break
				}
			}
		}
		if len(recentBlocked) > 0 {
			req.RecentBlockedDecisions = recentBlocked
			log.Printf("[recommend] %d recent blocked decision(s) injected into prompt", len(recentBlocked))
		}
	}

	// Step 6a: load latest rebalance analysis with freshness check.
	// If fresh (< 24h), inject it. If stale/missing, use stale as fallback and
	// kick off a background refresh so the next recommendation gets fresh context.
	const rebalanceCacheTTL = 24 * time.Hour
	ra, raErr := s.rebalanceRepo.GetLatestAnalysis(ctx, userID)
	if raErr != nil {
		log.Printf("[recommend] rebalance analysis fetch failed (%v) — skipped", raErr)
	} else if ra != nil && time.Since(ra.GeneratedAt) < rebalanceCacheTTL {
		req.RebalanceAnalysis = ra
		log.Printf("[recommend] rebalance analysis loaded (%d insights, generated %s)", len(ra.Insights), ra.GeneratedAt.Format("Jan 2"))
	} else {
		if ra != nil {
			req.RebalanceAnalysis = ra // use stale as fallback for this request
		}
		go s.refreshRebalanceAnalysis(context.WithoutCancel(ctx), userID, profile)
		log.Printf("[recommend] rebalance analysis stale/missing — background refresh started")
	}

	// Step 6b: compute per-ticker position context from already-loaded decision history.
	// Derived entirely from recentDecisions — zero new DB calls.
	// Receipts with FilledPrice==0 are skipped (Alpaca async orders not yet settled).
	if len(recentDecisions) > 0 {
		type accumulator struct {
			priceSum      float64
			totalInvested float64
			count         int
			earliest      time.Time
		}
		accs := make(map[string]*accumulator)
		for _, d := range recentDecisions {
			for _, receipt := range d.Receipts {
				if receipt.FilledPrice == 0 {
					continue
				}
				acc, ok := accs[receipt.Ticker]
				if !ok {
					acc = &accumulator{earliest: receipt.Timestamp}
					accs[receipt.Ticker] = acc
				}
				acc.priceSum += receipt.FilledPrice
				acc.totalInvested += receipt.FilledAmount
				acc.count++
				if receipt.Timestamp.Before(acc.earliest) {
					acc.earliest = receipt.Timestamp
				}
			}
		}
		if len(accs) > 0 {
			posCtx := make(map[string]models.TickerContext, len(accs))
			now := time.Now()
			for ticker, acc := range accs {
				months := int(now.Sub(acc.earliest).Hours() / 24 / 30.44)
				posCtx[ticker] = models.TickerContext{
					AverageCostBasis: acc.priceSum / float64(acc.count),
					TotalInvested:    acc.totalInvested,
					PurchaseCount:    acc.count,
					FirstPurchasedAt: acc.earliest,
					MonthsHeld:       months,
				}
			}
			req.PositionContext = posCtx
			log.Printf("[recommend] position context built for %d ticker(s)", len(posCtx))
		}
	}

	// Step 6.5: stamp verdicts for any decisions older than 24h that haven't been evaluated yet.
	// Runs concurrently with step 7 so tax doc fetch and Polygon calls happen in parallel.
	// Must complete before step 8 so GetEvalSummary reflects freshly-stamped verdicts.
	stampDone := make(chan struct{})
	go func() {
		defer close(stampDone)
		stampVerdicts(ctx, userID, 24*time.Hour,
			s.decisionRepo, s.profileRepo, s.brokerageFactory, s.marketData)
	}()

	// Step 7: tax documents — gives Claude income, withholding, and housing context
	taxDocs, err := s.documentRepo.GetByUserID(ctx, userID)
	if err != nil {
		log.Printf("[recommend] tax documents fetch failed (%v) — skipped", err)
	} else {
		req.TaxDocuments = taxDocs
		log.Printf("[recommend] %d tax document(s) loaded", len(taxDocs))
	}

	<-stampDone

	// Step 8: performance summary — gives Claude feedback on its own track record for this user.
	// Non-fatal; omitted silently if below the 5-verdict threshold or on any error.
	const minVerdictsForFeedback = 5
	if ps, psErr := s.decisionRepo.GetEvalSummary(ctx, userID); psErr != nil {
		log.Printf("[recommend] performance summary fetch failed (%v) — skipped", psErr)
	} else if ps != nil && ps.VerdictedDecisions >= minVerdictsForFeedback {
		req.PerformanceSummary = ps
		log.Printf("[recommend] performance summary loaded (%d verdicts)", ps.VerdictedDecisions)
	}

	// Step 9: Claude generates allocation (fetches market news itself via get_market_news tool)
	log.Printf("[recommend] sending to Claude (profile=%v market=%v plaid=%v positions=%d decisions=%d taxdocs=%d perf=%v)",
		profile != nil, snapshot != nil, req.BalanceSummary != nil, len(req.Positions), len(req.RecentDecisions), len(req.TaxDocuments), req.PerformanceSummary != nil)
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		if errors.Is(err, ports.ErrAdvisorOverloaded) {
			cached, fallbackErr := s.cachedRecommendation(ctx, userID, req.BaseBudget)
			if fallbackErr == nil {
				log.Printf("[recommend] advisor overloaded — returning cached recommendation")
				return cached, nil
			}
			log.Printf("[recommend] advisor overloaded and no cached recommendation available: %v", fallbackErr)
		}
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] Claude returned %d allocation(s) (risk=%s)", len(rec.Allocations), rec.RiskLevel)

	// Step 9.5: adversarial critic pass — reviews the recommendation before execution.
	// Non-fatal if the critic itself errors: we log and proceed with the recommendation.
	criticReview, criticErr := s.critic.ReviewRecommendation(ctx, req, profile, rec)
	if criticErr != nil {
		log.Printf("[recommend] critic review failed (%v) — proceeding without critic", criticErr)
	} else if criticReview.Verdict == "block" {
		log.Printf("[recommend] critic BLOCKED recommendation (risk=%s): %s", criticReview.RiskLevel, criticReview.Reasoning)
		blocked := &models.InvestmentDecision{
			UserID:         userID,
			Timestamp:      time.Now(),
			MarketSnapshot: snapshot,
			Allocations:    rec.Allocations,
			TotalAmount:    rec.TotalBudget,
			RiskLevel:      rec.RiskLevel,
			Summary:        rec.Summary,
			DecisionType:   "blocked",
			BlockedReason:  criticReview.Reasoning,
			CriticReview:   criticReview,
		}
		go func(saveCtx context.Context, d *models.InvestmentDecision) {
			if err := s.decisionRepo.Save(saveCtx, d); err != nil {
				log.Printf("[recommend] persist blocked decision failed: %v", err)
			} else {
				log.Printf("[recommend] persist blocked decision saved to MongoDB")
			}
		}(context.WithoutCancel(ctx), blocked)

		if profile != nil && (profile.NotificationEmail != "" || profile.Phone != "") {
			target := ports.NotificationTarget{
				UserID: userID,
				Email:  profile.NotificationEmail,
				Phone:  profile.Phone,
				Source: "manual",
			}
			_ = s.notifications.SendInvestmentFailure(ctx, target, "Recommendation blocked by risk review: "+criticReview.Reasoning)
		}

		// Return $0 recommendation — callers check WasBlocked to avoid writing a duplicate skip doc.
		return &models.Recommendation{
			TotalBudget: 0,
			WasBlocked:  true,
			SkipReason:  "Blocked by risk review: " + criticReview.Reasoning,
			Summary:     rec.Summary,
			RiskLevel:   rec.RiskLevel,
		}, nil
	}

	return rec, nil
}

// refreshRebalanceAnalysis builds a fresh portfolio analysis in the background and persists it.
// Called as a goroutine when the cached analysis is stale or missing — never blocks a recommendation.
func (s *RecommendationService) refreshRebalanceAnalysis(ctx context.Context, userID string, profile *models.UserProfile) {
	req, err := s.rebalanceAggregator.BuildRequest(ctx, userID)
	if err != nil {
		log.Printf("[recommend] background rebalance: build request failed (%v)", err)
		return
	}
	if len(req.Positions) == 0 {
		return // nothing to analyze
	}
	analysis, err := s.rebalanceAdvisor.AnalyzePortfolio(ctx, *req, profile)
	if err != nil {
		log.Printf("[recommend] background rebalance: analyze failed (%v)", err)
		return
	}
	analysis.UserID = userID
	if err := s.rebalanceRepo.SaveAnalysis(ctx, analysis); err != nil {
		log.Printf("[recommend] background rebalance: save failed (%v)", err)
		return
	}
	log.Printf("[recommend] background rebalance refresh complete for user=%s", userID)
}

// GetCashContext returns a pre-computed spending insight for the given user.
// Called by the frontend before the user taps invest so the UI can surface a nudge.
// Returns has_data=false when no Plaid connections exist or data is unavailable —
// the frontend renders nothing in that case.
func (s *RecommendationService) GetCashContext(ctx context.Context, userID string) (models.CashContext, error) {
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil || len(connections) == 0 {
		return models.CashContext{HasData: false}, nil
	}

	balance, err := s.financialData.GetBalanceSummary(ctx, connections)
	if err != nil {
		return models.CashContext{HasData: false}, nil
	}

	txSummary, err := s.financialData.GetTransactionSummary(ctx, connections)
	if err != nil {
		return models.CashContext{HasData: false}, nil
	}

	// Need spend data to be meaningful; zero spend means no transactions came through
	if txSummary.SpendLast30Days == 0 {
		return models.CashContext{HasData: false}, nil
	}

	dailySpend := txSummary.SpendLast30Days / 30
	runwayDays := int(balance.TotalCash / dailySpend)

	label, message := runwayLabelAndMessage(runwayDays)

	return models.CashContext{
		HasData:              true,
		RunwayDays:           runwayDays,
		RunwayLabel:          label,
		SpendLast7D:          txSummary.SpendLast7Days,
		SpendLast30D:         txSummary.SpendLast30Days,
		LargestPendingAmount: txSummary.LargestPendingAmount,
		LargestPendingName:   txSummary.LargestPendingName,
		Message:              message,
	}, nil
}

// cachedRecommendation pulls the user's most recent investment decision from MongoDB and
// rescales the allocation amounts to budget. Returns an error when no prior decision exists.
func (s *RecommendationService) cachedRecommendation(ctx context.Context, userID string, budget float64) (*models.Recommendation, error) {
	decisions, err := s.decisionRepo.ListByUser(ctx, userID, 20)
	if err != nil {
		return nil, fmt.Errorf("no cached decision available")
	}
	var d *models.InvestmentDecision
	for i := range decisions {
		if decisions[i].DecisionType == "invest" {
			d = &decisions[i]
			break
		}
	}
	if d == nil {
		return nil, fmt.Errorf("no cached invest decision available")
	}
	if len(d.Allocations) == 0 {
		return nil, fmt.Errorf("cached decision has no allocations")
	}

	// Rescale amounts to the requested budget while preserving percentages.
	scale := budget / d.TotalAmount
	allocs := make([]models.Allocation, len(d.Allocations))
	for i, a := range d.Allocations {
		allocs[i] = a
		allocs[i].Amount = a.Amount * scale
	}

	return &models.Recommendation{
		TotalBudget: budget,
		Allocations: allocs,
		Summary:     d.Summary,
		RiskLevel:   d.RiskLevel,
		FromCache:   true,
	}, nil
}

func runwayLabelAndMessage(days int) (string, string) {
	switch {
	case days > 30:
		return "healthy", "Your cash position looks strong."
	case days >= 14:
		return "moderate", "Moderate cash runway. Proceed as planned or adjust."
	default:
		return "tight", fmt.Sprintf("Your cash covers about %d days of typical spending. You may want to invest a smaller amount today.", days)
	}
}
