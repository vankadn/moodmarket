// application/services/service_fakes_test.go
//
// Configurable test doubles for the P2 service-layer tests. They embed the
// fixed-value stubs already defined in recommendation_service_test.go and
// override only the methods a given test needs to drive, so each fake stays
// small and the interface surface is satisfied for free.
package services

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ── configurable decision repo ──────────────────────────────────────────────

type fakeDecisionRepo struct {
	*stubDecisionRepo
	decisions []models.InvestmentDecision
	listErr   error

	saved   *models.InvestmentDecision
	saveErr error
}

func newFakeDecisionRepo() *fakeDecisionRepo {
	return &fakeDecisionRepo{stubDecisionRepo: &stubDecisionRepo{}}
}

func (r *fakeDecisionRepo) ListByUser(_ context.Context, _ string, _ int) ([]models.InvestmentDecision, error) {
	return r.decisions, r.listErr
}

func (r *fakeDecisionRepo) Save(_ context.Context, d *models.InvestmentDecision) error {
	r.saved = d
	return r.saveErr
}

// ── configurable profile repo ───────────────────────────────────────────────

type fakeProfileRepo struct {
	*stubProfileRepo
	connections   []models.BrokerageConnection
	connErr       error
	profile       *models.UserProfile
	profileErr    error
	portfolioConn *models.PortfolioConnection
}

func newFakeProfileRepo() *fakeProfileRepo {
	return &fakeProfileRepo{stubProfileRepo: &stubProfileRepo{}}
}

func (r *fakeProfileRepo) GetBrokerageConnections(_ context.Context, _ string) ([]models.BrokerageConnection, error) {
	return r.connections, r.connErr
}

func (r *fakeProfileRepo) GetByUserID(_ context.Context, _ string) (*models.UserProfile, error) {
	if r.profileErr != nil {
		return nil, r.profileErr
	}
	return r.profile, nil
}

func (r *fakeProfileRepo) GetPortfolioConnection(_ context.Context, _ string) (*models.PortfolioConnection, error) {
	return r.portfolioConn, nil
}

// ── configurable brokerage provider + factory ───────────────────────────────

type fakeBrokerageProvider struct {
	positions    []models.Position
	positionsErr error

	// placeFn lets a test decide per-order whether the order succeeds.
	placeFn      func(models.TradeOrder) (*models.TradeReceipt, error)
	placedOrders []models.TradeOrder

	price    float64
	priceErr error
}

func (p *fakeBrokerageProvider) PlaceMarketOrder(_ context.Context, order models.TradeOrder) (*models.TradeReceipt, error) {
	p.placedOrders = append(p.placedOrders, order)
	if p.placeFn != nil {
		return p.placeFn(order)
	}
	return &models.TradeReceipt{OrderID: "ord-" + order.Ticker, Ticker: order.Ticker, FilledAmount: order.Amount, FilledPrice: 100, Status: "filled"}, nil
}

func (p *fakeBrokerageProvider) GetPositions(_ context.Context, _ string) ([]models.Position, error) {
	return p.positions, p.positionsErr
}

func (p *fakeBrokerageProvider) GetOrder(_ context.Context, _ string) (*models.TradeReceipt, error) {
	return nil, nil
}

func (p *fakeBrokerageProvider) GetPortfolioHistory(_ context.Context, _, _, _ string) ([]models.HistoryPoint, error) {
	return nil, nil
}

func (p *fakeBrokerageProvider) GetCurrentPrice(_ context.Context, _ string) (float64, error) {
	return p.price, p.priceErr
}

type fakeBrokerageFactory struct {
	provider ports.BrokerageProvider
	err      error
}

func (f *fakeBrokerageFactory) ForUser(_ *models.BrokerageConnection) (ports.BrokerageProvider, error) {
	return f.provider, f.err
}

// ── configurable market data ────────────────────────────────────────────────

type fakeMarketData struct {
	snapshot    *models.MarketSnapshot
	snapshotErr error
}

func (m *fakeMarketData) GetDailySnapshot(_ context.Context) (*models.MarketSnapshot, error) {
	return m.snapshot, m.snapshotErr
}

func (m *fakeMarketData) GetPrice(_ context.Context, _ string) (float64, error) {
	return 0, nil
}

// ── configurable portfolio aggregator ───────────────────────────────────────

type fakePortfolioAggregator struct {
	*stubPortfolioAggregator
	byAccount    map[string][]models.Position
	byAccountErr error
}

func newFakePortfolioAggregator() *fakePortfolioAggregator {
	return &fakePortfolioAggregator{stubPortfolioAggregator: &stubPortfolioAggregator{}}
}

func (p *fakePortfolioAggregator) GetHoldingsByAccount(_ context.Context, _, _ string) (map[string][]models.Position, error) {
	return p.byAccount, p.byAccountErr
}

// ── configurable document extractor + repo ──────────────────────────────────

type fakeExtractor struct {
	doc *models.TaxDocument
	err error

	gotBytes []byte
	gotType  string
}

func (e *fakeExtractor) ExtractTaxDocument(_ context.Context, fileBytes []byte, documentType string) (*models.TaxDocument, error) {
	e.gotBytes = fileBytes
	e.gotType = documentType
	return e.doc, e.err
}

type fakeDocumentRepo struct {
	*stubDocumentRepo
	saved   *models.TaxDocument
	saveErr error

	docs    []*models.TaxDocument
	listErr error

	deleteErr   error
	deletedUser string
	deletedID   string
}

func newFakeDocumentRepo() *fakeDocumentRepo {
	return &fakeDocumentRepo{stubDocumentRepo: &stubDocumentRepo{}}
}

func (d *fakeDocumentRepo) Save(_ context.Context, doc *models.TaxDocument) error {
	d.saved = doc
	return d.saveErr
}

func (d *fakeDocumentRepo) GetByUserID(_ context.Context, _ string) ([]*models.TaxDocument, error) {
	return d.docs, d.listErr
}

func (d *fakeDocumentRepo) DeleteByID(_ context.Context, userID, docID string) error {
	d.deletedUser = userID
	d.deletedID = docID
	return d.deleteErr
}
