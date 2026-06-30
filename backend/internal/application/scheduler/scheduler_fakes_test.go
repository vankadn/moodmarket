// application/scheduler/scheduler_fakes_test.go
//
// Test doubles for the scheduler package. These exist because runForUser and
// runCycle were refactored to depend on the narrow interfaces in deps.go plus
// the domain ports, so they can now be driven without constructing the real
// services and their heavy dependency graphs.
package scheduler

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ── Recommender / Investor (deps.go interfaces) ─────────────────────────────

type fakeRecommender struct {
	rec     *models.Recommendation
	err     error
	gotReq  models.InvestmentRequest
	calls   int
}

func (r *fakeRecommender) GetDailyRecommendation(_ context.Context, _ string, req models.InvestmentRequest) (*models.Recommendation, error) {
	r.calls++
	r.gotReq = req
	if r.err != nil {
		return nil, r.err
	}
	return r.rec, nil
}

type fakeInvestor struct {
	receipts   []models.TradeReceipt
	decisionID string
	err        error

	called      bool
	gotAllocs   []models.Allocation
	gotTotal    float64
	gotConfigID string
}

func (i *fakeInvestor) Execute(
	_ context.Context,
	_ string,
	allocations []models.Allocation,
	totalAmount float64,
	_, _, _ string,
	_ map[string]string,
	configID string,
) ([]models.TradeReceipt, string, error) {
	i.called = true
	i.gotAllocs = allocations
	i.gotTotal = totalAmount
	i.gotConfigID = configID
	if i.err != nil {
		return nil, "", i.err
	}
	return i.receipts, i.decisionID, nil
}

// ── NotificationProvider ────────────────────────────────────────────────────

type fakeNotifications struct {
	summaryCalls int
	failureCalls int
	skipCalls    int
	skipReason   string

	lastSummaryReceipts []models.TradeReceipt
	lastSummaryTotal    float64
}

func (n *fakeNotifications) SendInvestmentSummary(_ context.Context, _ ports.NotificationTarget, receipts []models.TradeReceipt, totalInvested float64, _ string) error {
	n.summaryCalls++
	n.lastSummaryReceipts = receipts
	n.lastSummaryTotal = totalInvested
	return nil
}
func (n *fakeNotifications) SendInvestmentFailure(_ context.Context, _ ports.NotificationTarget, _ string) error {
	n.failureCalls++
	return nil
}
func (n *fakeNotifications) SendMarketClosed(_ context.Context, _ ports.NotificationTarget, _ string) error {
	return nil
}
func (n *fakeNotifications) SendSkipSummary(_ context.Context, _ ports.NotificationTarget, _, reason string) error {
	n.skipCalls++
	n.skipReason = reason
	return nil
}
func (n *fakeNotifications) SendRebalancingAlert(_ context.Context, _ ports.NotificationTarget, _ []models.TickerDrift) error {
	return nil
}
func (n *fakeNotifications) SendRebalanceDigest(_ context.Context, _ ports.NotificationTarget, _ *models.RebalanceAnalysis) error {
	return nil
}

// ── DecisionRepository ──────────────────────────────────────────────────────

type fakeDecisionRepo struct {
	spentToday    float64
	spentTodayErr error

	hasSkipToday    bool
	hasSkipTodayErr error

	saved []*models.InvestmentDecision
}

func (r *fakeDecisionRepo) Save(_ context.Context, d *models.InvestmentDecision) error {
	r.saved = append(r.saved, d)
	return nil
}
func (r *fakeDecisionRepo) SumInvestedToday(_ context.Context, _, _, _ string) (float64, error) {
	return r.spentToday, r.spentTodayErr
}
func (r *fakeDecisionRepo) SumAllTimeByConfig(_ context.Context, _, _ string) (float64, error) {
	return 0, nil
}
func (r *fakeDecisionRepo) HasSkipToday(_ context.Context, _, _, _, _ string) (bool, error) {
	return r.hasSkipToday, r.hasSkipTodayErr
}
func (r *fakeDecisionRepo) ListByUser(_ context.Context, _ string, _ int) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) ListByUserSince(_ context.Context, _ string, _ *time.Time) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) ActivityByStrategy(_ context.Context, _ string) ([]models.StrategyActivity, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) CostBasisByStrategy(_ context.Context, _ string) (map[string]map[string]float64, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) StampVerdict(_ context.Context, _ string, _ *models.DecisionVerdict) error {
	return nil
}
func (r *fakeDecisionRepo) ListUnverdicted(_ context.Context, _ string, _ time.Duration) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) GetUsersWithPendingVerdicts(_ context.Context, _ time.Duration) ([]string, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) GetEvalSummary(_ context.Context, _ string) (*models.EvalSummary, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) ListDecisions(_ context.Context, _ string, _, _ int) ([]models.InvestmentDecision, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) WinRateTrend(_ context.Context, _ string, _ int) ([]models.WinRateTrendPoint, error) {
	return nil, nil
}
func (r *fakeDecisionRepo) AssetClassBreakdown(_ context.Context, _ string) ([]models.AssetClassBreakdownItem, error) {
	return nil, nil
}

// ── AutoInvestRepository ────────────────────────────────────────────────────

type fakeAutoInvestRepo struct {
	enabled    []models.AutoInvestConfig
	enabledErr error

	stamped map[string]time.Time // configID → time
}

func newFakeAutoInvestRepo() *fakeAutoInvestRepo {
	return &fakeAutoInvestRepo{stamped: map[string]time.Time{}}
}

func (r *fakeAutoInvestRepo) GetAllEnabled(_ context.Context) ([]models.AutoInvestConfig, error) {
	return r.enabled, r.enabledErr
}
func (r *fakeAutoInvestRepo) StampLastRunAt(_ context.Context, configID string, t time.Time) error {
	r.stamped[configID] = t
	return nil
}
func (r *fakeAutoInvestRepo) GetByUserID(_ context.Context, _ string) (*models.AutoInvestConfig, error) {
	return nil, nil
}
func (r *fakeAutoInvestRepo) Upsert(_ context.Context, _ *models.AutoInvestConfig) error { return nil }
func (r *fakeAutoInvestRepo) GetAllByUserID(_ context.Context, _ string) ([]models.AutoInvestConfig, error) {
	return nil, nil
}
func (r *fakeAutoInvestRepo) Create(_ context.Context, c *models.AutoInvestConfig) (*models.AutoInvestConfig, error) {
	return c, nil
}
func (r *fakeAutoInvestRepo) UpdateByID(_ context.Context, _, _ string, c *models.AutoInvestConfig) (*models.AutoInvestConfig, error) {
	return c, nil
}
func (r *fakeAutoInvestRepo) DeleteByID(_ context.Context, _, _ string) error { return nil }

// ── ProfileRepository ───────────────────────────────────────────────────────

type fakeProfileRepo struct {
	profile *models.UserProfile
	err     error
}

func (r *fakeProfileRepo) GetByUserID(_ context.Context, _ string) (*models.UserProfile, error) {
	return r.profile, r.err
}
func (r *fakeProfileRepo) Upsert(_ context.Context, _ *models.UserProfile) error { return nil }
func (r *fakeProfileRepo) SavePlaidConnection(_ context.Context, _ string, _ models.PlaidConnection) error {
	return nil
}
func (r *fakeProfileRepo) GetPlaidConnections(_ context.Context, _ string) ([]models.PlaidConnection, error) {
	return nil, nil
}
func (r *fakeProfileRepo) RemovePlaidConnection(_ context.Context, _, _ string) error { return nil }
func (r *fakeProfileRepo) GetBrokerageConnections(_ context.Context, _ string) ([]models.BrokerageConnection, error) {
	return nil, nil
}
func (r *fakeProfileRepo) UpsertBrokerageConnection(_ context.Context, _ string, _ models.BrokerageConnection) error {
	return nil
}
func (r *fakeProfileRepo) RemoveBrokerageConnection(_ context.Context, _, _ string) error { return nil }
func (r *fakeProfileRepo) SaveLegacySingleBrokerageConnection(_ context.Context, _ string, _ models.BrokerageConnection) error {
	return nil
}
func (r *fakeProfileRepo) ClearLegacySingleBrokerageConnection(_ context.Context, _ string) error {
	return nil
}
func (r *fakeProfileRepo) SavePortfolioConnection(_ context.Context, _ string, _ models.PortfolioConnection) error {
	return nil
}
func (r *fakeProfileRepo) GetPortfolioConnection(_ context.Context, _ string) (*models.PortfolioConnection, error) {
	return nil, nil
}
func (r *fakeProfileRepo) ClearPortfolioConnection(_ context.Context, _ string) error { return nil }

// ── SchedulerRepository / MarketCalendar ────────────────────────────────────

type fakeSchedulerRepo struct {
	runs []*models.SchedulerRun
}

func (r *fakeSchedulerRepo) Save(_ context.Context, run *models.SchedulerRun) error {
	r.runs = append(r.runs, run)
	return nil
}

type fakeCalendar struct{ tradingDay bool }

func (c *fakeCalendar) IsTradingDay(_ time.Time) bool { return c.tradingDay }
