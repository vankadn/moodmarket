// application/services/recommendation_service_test.go
//
// Phase 24 — Feedback Loop: service-layer tests.
//
// These tests pin the threshold logic that gates whether a user's aggregate
// verdict performance is injected into the InvestmentRequest before Claude is
// called. Reading the test names explains the requirement:
//
//  - When verdict count meets the minimum threshold (5), the summary is injected.
//  - When verdict count is below the threshold, the summary is withheld.
//  - When GetEvalSummary returns nil, the summary is withheld.
//  - When GetEvalSummary returns an error, the summary is withheld (non-fatal).
//
// The tests use a capturingAdvisor that records the InvestmentRequest it
// receives so we can assert on PerformanceSummary without parsing a prompt.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ── mock implementations ─────────────────────────────────────────────────────

// capturingAdvisor records the InvestmentRequest it receives so tests can
// assert on PerformanceSummary without inspecting string output.
type capturingAdvisor struct {
	capturedReq models.InvestmentRequest
}

func (a *capturingAdvisor) GetRecommendation(_ context.Context, req models.InvestmentRequest, _ *models.UserProfile, _ *models.MarketSnapshot) (*models.Recommendation, error) {
	a.capturedReq = req
	return &models.Recommendation{
		TotalBudget: req.BaseBudget,
		Allocations: []models.Allocation{{Ticker: "VTI", Percentage: 100, Amount: req.BaseBudget}},
		Summary:     "stub recommendation",
		RiskLevel:   "moderate",
	}, nil
}

// stubDecisionRepo is a no-op DecisionRepository; individual test cases
// replace evalSummary / evalErr to exercise the GetEvalSummary branch.
type stubDecisionRepo struct {
	evalSummary *models.EvalSummary
	evalErr     error
}

func (r *stubDecisionRepo) Save(_ context.Context, _ *models.InvestmentDecision) error { return nil }
func (r *stubDecisionRepo) ListByUser(_ context.Context, _ string, _ int) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *stubDecisionRepo) ListByUserSince(_ context.Context, _ string, _ *time.Time) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *stubDecisionRepo) ActivityByStrategy(_ context.Context, _ string) ([]models.StrategyActivity, error) {
	return nil, nil
}
func (r *stubDecisionRepo) CostBasisByStrategy(_ context.Context, _ string) (map[string]map[string]float64, error) {
	return nil, nil
}
func (r *stubDecisionRepo) StampVerdict(_ context.Context, _ string, _ *models.DecisionVerdict) error {
	return nil
}
func (r *stubDecisionRepo) ListUnverdicted(_ context.Context, _ string, _ time.Duration) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *stubDecisionRepo) GetUsersWithPendingVerdicts(_ context.Context, _ time.Duration) ([]string, error) {
	return nil, nil
}
func (r *stubDecisionRepo) GetEvalSummary(_ context.Context, _ string) (*models.EvalSummary, error) {
	return r.evalSummary, r.evalErr
}
func (r *stubDecisionRepo) ListDecisions(_ context.Context, _ string, _, _ int) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *stubDecisionRepo) SumInvestedToday(_ context.Context, _, _, _ string) (float64, error) {
	return 0, nil
}
func (r *stubDecisionRepo) SumAllTimeByConfig(_ context.Context, _, _ string) (float64, error) {
	return 0, nil
}
func (r *stubDecisionRepo) HasSkipToday(_ context.Context, _, _, _, _ string) (bool, error) {
	return false, nil
}
func (r *stubDecisionRepo) WinRateTrend(_ context.Context, _ string, _ int) ([]models.WinRateTrendPoint, error) {
	return nil, nil
}
func (r *stubDecisionRepo) AssetClassBreakdown(_ context.Context, _ string) ([]models.AssetClassBreakdownItem, error) {
	return nil, nil
}

// stubProfileRepo returns ErrProfileNotFound for all users (no profile on file).
type stubProfileRepo struct{}

func (r *stubProfileRepo) GetByUserID(_ context.Context, _ string) (*models.UserProfile, error) {
	return nil, ports.ErrProfileNotFound
}
func (r *stubProfileRepo) Upsert(_ context.Context, _ *models.UserProfile) error { return nil }
func (r *stubProfileRepo) SavePlaidConnection(_ context.Context, _ string, _ models.PlaidConnection) error {
	return nil
}
func (r *stubProfileRepo) GetPlaidConnections(_ context.Context, _ string) ([]models.PlaidConnection, error) {
	return nil, nil
}
func (r *stubProfileRepo) RemovePlaidConnection(_ context.Context, _, _ string) error { return nil }
func (r *stubProfileRepo) GetBrokerageConnections(_ context.Context, _ string) ([]models.BrokerageConnection, error) {
	return nil, nil
}
func (r *stubProfileRepo) UpsertBrokerageConnection(_ context.Context, _ string, _ models.BrokerageConnection) error {
	return nil
}
func (r *stubProfileRepo) RemoveBrokerageConnection(_ context.Context, _, _ string) error {
	return nil
}
func (r *stubProfileRepo) SaveLegacySingleBrokerageConnection(_ context.Context, _ string, _ models.BrokerageConnection) error {
	return nil
}
func (r *stubProfileRepo) ClearLegacySingleBrokerageConnection(_ context.Context, _ string) error {
	return nil
}
func (r *stubProfileRepo) SavePortfolioConnection(_ context.Context, _ string, _ models.PortfolioConnection) error {
	return nil
}
func (r *stubProfileRepo) GetPortfolioConnection(_ context.Context, _ string) (*models.PortfolioConnection, error) {
	return nil, nil
}
func (r *stubProfileRepo) ClearPortfolioConnection(_ context.Context, _ string) error { return nil }

// stubMarketData returns an error so the snapshot is skipped (non-fatal path).
type stubMarketData struct{}

func (m *stubMarketData) GetDailySnapshot(_ context.Context) (*models.MarketSnapshot, error) {
	return nil, errors.New("market data stub: unavailable")
}
func (m *stubMarketData) GetPrice(_ context.Context, _ string) (float64, error) {
	return 0, errors.New("market data stub: unavailable")
}

// stubFinancialData is never reached because stubProfileRepo returns no Plaid connections.
type stubFinancialData struct{}

func (f *stubFinancialData) CreateLinkToken(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (f *stubFinancialData) ExchangePublicToken(_ context.Context, _ string) (string, string, string, error) {
	return "", "", "", nil
}
func (f *stubFinancialData) GetAccounts(_ context.Context, _ string) ([]models.BankAccount, error) {
	return nil, nil
}
func (f *stubFinancialData) GetBalanceSummary(_ context.Context, _ []models.PlaidConnection) (models.BalanceSummary, error) {
	return models.BalanceSummary{}, nil
}
func (f *stubFinancialData) GetTransactionSummary(_ context.Context, _ []models.PlaidConnection) (models.TransactionSummary, error) {
	return models.TransactionSummary{}, nil
}
func (f *stubFinancialData) RevokeToken(_ context.Context, _ string) error { return nil }

// stubBrokerageFactory is never reached because stubProfileRepo returns no brokerage connections.
type stubBrokerageFactory struct{}

func (b *stubBrokerageFactory) ForUser(_ *models.BrokerageConnection) (ports.BrokerageProvider, error) {
	return nil, errors.New("brokerage factory stub: not configured")
}

// stubPortfolioAggregator is never reached because stubProfileRepo returns no portfolio connection.
type stubPortfolioAggregator struct{}

func (p *stubPortfolioAggregator) ListAccounts(_ context.Context, _, _ string) ([]models.LinkedAccount, error) {
	return nil, nil
}

func (p *stubPortfolioAggregator) GetHoldings(_ context.Context, _, _ string) ([]models.Position, error) {
	return nil, nil
}

func (p *stubPortfolioAggregator) GetHoldingsByAccount(_ context.Context, _, _ string) (map[string][]models.Position, error) {
	return nil, nil
}

// stubRebalanceRepo returns nil analysis for all users (no rebalance analysis on file).
type stubRebalanceRepo struct{}

func (r *stubRebalanceRepo) SaveAnalysis(_ context.Context, _ *models.RebalanceAnalysis) error {
	return nil
}
func (r *stubRebalanceRepo) GetLatestAnalysis(_ context.Context, _ string) (*models.RebalanceAnalysis, error) {
	return nil, nil
}

// stubRebalanceAggregator returns an empty request (no positions — background refresh exits early).
type stubRebalanceAggregator struct{}

func (s *stubRebalanceAggregator) BuildRequest(_ context.Context, _ string) (*models.RebalanceRequest, error) {
	return &models.RebalanceRequest{}, nil
}

// stubRebalanceAdvisor always fails — never reached when positions are empty.
type stubRebalanceAdvisor struct{}

func (s *stubRebalanceAdvisor) AnalyzePortfolio(_ context.Context, _ models.RebalanceRequest, _ *models.UserProfile) (*models.RebalanceAnalysis, error) {
	return nil, errors.New("stub: no analysis")
}

// stubDocumentRepo returns no tax documents.
type stubDocumentRepo struct{}

func (d *stubDocumentRepo) Save(_ context.Context, _ *models.TaxDocument) error { return nil }
func (d *stubDocumentRepo) GetByUserID(_ context.Context, _ string) ([]*models.TaxDocument, error) {
	return nil, nil
}
func (d *stubDocumentRepo) GetByID(_ context.Context, _ string) (*models.TaxDocument, error) {
	return nil, nil
}
func (d *stubDocumentRepo) DeleteByID(_ context.Context, _, _ string) error { return nil }

// stubRecommendationCritic always approves — never blocks in unit tests.
type stubRecommendationCritic struct{}

func (c *stubRecommendationCritic) ReviewRecommendation(_ context.Context, _ models.InvestmentRequest, _ *models.UserProfile, _ *models.Recommendation) (*models.CriticReview, error) {
	return &models.CriticReview{Verdict: "approve", Concerns: []string{}, RiskLevel: "low", Reasoning: "stub: approved"}, nil
}

// stubNotificationProvider is a no-op.
type stubNotificationProvider struct{}

func (n *stubNotificationProvider) SendInvestmentSummary(_ context.Context, _ ports.NotificationTarget, _ []models.TradeReceipt, _ float64, _ string) error {
	return nil
}
func (n *stubNotificationProvider) SendInvestmentFailure(_ context.Context, _ ports.NotificationTarget, _ string) error {
	return nil
}
func (n *stubNotificationProvider) SendMarketClosed(_ context.Context, _ ports.NotificationTarget, _ string) error {
	return nil
}
func (n *stubNotificationProvider) SendSkipSummary(_ context.Context, _ ports.NotificationTarget, _, _ string) error {
	return nil
}
func (n *stubNotificationProvider) SendRebalancingAlert(_ context.Context, _ ports.NotificationTarget, _ []models.TickerDrift) error {
	return nil
}
func (n *stubNotificationProvider) SendRebalanceDigest(_ context.Context, _ ports.NotificationTarget, _ *models.RebalanceAnalysis) error {
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newServiceWithDecisionRepo wires a RecommendationService with the given
// decision repo and a capturing advisor. Returns both so callers can inspect
// what the advisor received.
func newServiceWithDecisionRepo(decisionRepo ports.DecisionRepository) (*RecommendationService, *capturingAdvisor) {
	advisor := &capturingAdvisor{}
	svc := NewRecommendationService(
		advisor,
		&stubProfileRepo{},
		&stubMarketData{},
		decisionRepo,
		&stubFinancialData{},
		&stubBrokerageFactory{},
		&stubDocumentRepo{},
		&stubPortfolioAggregator{},
		&stubRebalanceRepo{},
		&stubRebalanceAggregator{},
		&stubRebalanceAdvisor{},
		&stubRecommendationCritic{},
		&stubNotificationProvider{},
	)
	return svc, advisor
}

func evalSummaryWithVerdicts(n int) *models.EvalSummary {
	return &models.EvalSummary{
		TotalDecisions:     n,
		VerdictedDecisions: n,
		WinRate:            0.71,
		AvgReturnPct:       2.3,
		AvgSPYReturnPct:    1.1,
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestPhase24_PerformanceSummaryInjection tests the step-8 threshold logic that
// decides whether aggregate verdict stats are passed to the advisor.
func TestPhase24_PerformanceSummaryInjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                     string
		evalSummary              *models.EvalSummary
		evalErr                  error
		wantPerformanceSummaryNil bool
	}{
		{
			// The advisor should receive the full performance context when the
			// user has enough evaluated decisions to make the stats meaningful.
			name:                     "performance_summary_is_injected_into_advisor_request_when_verdict_count_meets_threshold",
			evalSummary:              evalSummaryWithVerdicts(5),
			wantPerformanceSummaryNil: false,
		},
		{
			// One more than the boundary to confirm it's not an off-by-one.
			name:                     "performance_summary_is_injected_when_verdict_count_is_well_above_threshold",
			evalSummary:              evalSummaryWithVerdicts(14),
			wantPerformanceSummaryNil: false,
		},
		{
			// With only 4 verdicts the sample is too small to be meaningful.
			// Injecting it would let one bad week steer Claude away from solid tickers.
			name:                     "performance_summary_is_withheld_when_verdict_count_is_below_threshold",
			evalSummary:              evalSummaryWithVerdicts(4),
			wantPerformanceSummaryNil: true,
		},
		{
			// Zero verdicts — brand-new user; no feedback to give Claude.
			name:                     "performance_summary_is_withheld_for_new_user_with_no_verdicts",
			evalSummary:              evalSummaryWithVerdicts(0),
			wantPerformanceSummaryNil: true,
		},
		{
			// GetEvalSummary returning nil is a valid "no data yet" response.
			// The section must be absent to avoid the prompt changing for new users.
			name:                     "performance_summary_is_withheld_when_eval_summary_returns_nil",
			evalSummary:              nil,
			wantPerformanceSummaryNil: true,
		},
		{
			// A database failure must not block the recommendation — step 8 is non-fatal.
			// The advisor is still called; the PerformanceSummary field is just omitted.
			name:                     "performance_summary_is_withheld_and_recommendation_still_succeeds_when_eval_summary_fetch_fails",
			evalErr:                  errors.New("mongo timeout"),
			wantPerformanceSummaryNil: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &stubDecisionRepo{evalSummary: tt.evalSummary, evalErr: tt.evalErr}
			svc, advisor := newServiceWithDecisionRepo(repo)

			req := models.InvestmentRequest{BaseBudget: 100}
			_, err := svc.GetDailyRecommendation(context.Background(), "user-1", req)
			if err != nil {
				t.Fatalf("GetDailyRecommendation returned unexpected error: %v", err)
			}

			gotNil := advisor.capturedReq.PerformanceSummary == nil
			if gotNil != tt.wantPerformanceSummaryNil {
				if tt.wantPerformanceSummaryNil {
					t.Errorf("expected PerformanceSummary to be nil (withheld), got %+v", advisor.capturedReq.PerformanceSummary)
				} else {
					t.Error("expected PerformanceSummary to be non-nil (injected), got nil")
				}
			}

			// When injected, verify the summary is the exact object returned by the repo
			// (not a copy or zero-value) — we want Claude to see real verdict data.
			if !tt.wantPerformanceSummaryNil && tt.evalSummary != nil {
				got := advisor.capturedReq.PerformanceSummary
				if got.VerdictedDecisions != tt.evalSummary.VerdictedDecisions {
					t.Errorf("VerdictedDecisions: got %d, want %d", got.VerdictedDecisions, tt.evalSummary.VerdictedDecisions)
				}
				if got.WinRate != tt.evalSummary.WinRate {
					t.Errorf("WinRate: got %.2f, want %.2f", got.WinRate, tt.evalSummary.WinRate)
				}
			}
		})
	}
}
