// infrastructure/db/mongo_decision_repository.go
package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// --- MongoDB document types (bson tags isolated here, never in domain models) ---

type decisionDocument struct {
	ID             primitive.ObjectID  `bson:"_id"`
	UserID         string              `bson:"user_id"`
	Timestamp      time.Time           `bson:"timestamp"`
	MarketSnapshot *marketSnapshotDoc  `bson:"market_snapshot,omitempty"`
	Allocations    []allocationDoc     `bson:"allocations"`
	Receipts       []tradeReceiptDoc   `bson:"receipts,omitempty"`
	TotalAmount    float64             `bson:"total_amount"`
	RiskLevel      string              `bson:"risk_level"`
	Summary        string              `bson:"summary"`
}

type tradeReceiptDoc struct {
	OrderID      string    `bson:"order_id"`
	Ticker       string    `bson:"ticker"`
	FilledAmount float64   `bson:"filled_amount"`
	FilledPrice  float64   `bson:"filled_price"`
	Status       string    `bson:"status"`
	Timestamp    time.Time `bson:"timestamp"`
}

type marketSnapshotDoc struct {
	Date              string              `bson:"date"`
	SPYChangePercent  float64             `bson:"spy_change_percent"`
	QQQChangePercent  float64             `bson:"qqq_change_percent"`
	SectorPerformance map[string]float64  `bson:"sector_performance"`
	MarketSentiment   string              `bson:"market_sentiment"`
	TopMovers         []tickerSnapshotDoc `bson:"top_movers"`
}

type tickerSnapshotDoc struct {
	Symbol        string  `bson:"symbol"`
	ChangePercent float64 `bson:"change_percent"`
	Price         float64 `bson:"price"`
}

type allocationDoc struct {
	Ticker     string  `bson:"ticker"`
	Name       string  `bson:"name"`
	Type       string  `bson:"type"`
	Amount     float64 `bson:"amount"`
	Percentage float64 `bson:"percentage"`
	Rationale  string  `bson:"rationale"`
}

// --- Repository ---

type MongoDecisionRepository struct {
	collection *mongo.Collection
}

func NewMongoDecisionRepository(db *mongo.Database) *MongoDecisionRepository {
	return &MongoDecisionRepository{collection: db.Collection("decisions")}
}

func (r *MongoDecisionRepository) Save(ctx context.Context, decision *models.InvestmentDecision) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	doc := fromDecision(decision)
	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("mongo decision repo: save: %w", err)
	}
	decision.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return nil
}

func (r *MongoDecisionRepository) ListByUser(ctx context.Context, userID string, limit int) ([]models.InvestmentDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: list: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []decisionDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo decision repo: decode: %w", err)
	}

	decisions := make([]models.InvestmentDecision, len(docs))
	for i, doc := range docs {
		decisions[i] = toDecision(&doc)
	}
	return decisions, nil
}

// --- Conversion helpers ---

func fromDecision(d *models.InvestmentDecision) *decisionDocument {
	doc := &decisionDocument{
		ID:          primitive.NewObjectID(),
		UserID:      d.UserID,
		Timestamp:   d.Timestamp,
		Allocations: fromAllocations(d.Allocations),
		Receipts:    fromReceipts(d.Receipts),
		TotalAmount: d.TotalAmount,
		RiskLevel:   d.RiskLevel,
		Summary:     d.Summary,
	}
	if d.MarketSnapshot != nil {
		doc.MarketSnapshot = fromMarketSnapshot(d.MarketSnapshot)
	}
	return doc
}

func toDecision(doc *decisionDocument) models.InvestmentDecision {
	d := models.InvestmentDecision{
		ID:          doc.ID.Hex(),
		UserID:      doc.UserID,
		Timestamp:   doc.Timestamp,
		Allocations: toAllocations(doc.Allocations),
		Receipts:    toReceipts(doc.Receipts),
		TotalAmount: doc.TotalAmount,
		RiskLevel:   doc.RiskLevel,
		Summary:     doc.Summary,
	}
	if doc.MarketSnapshot != nil {
		d.MarketSnapshot = toMarketSnapshot(doc.MarketSnapshot)
	}
	return d
}

func fromMarketSnapshot(s *models.MarketSnapshot) *marketSnapshotDoc {
	movers := make([]tickerSnapshotDoc, len(s.TopMovers))
	for i, m := range s.TopMovers {
		movers[i] = tickerSnapshotDoc{Symbol: m.Symbol, ChangePercent: m.ChangePercent, Price: m.Price}
	}
	return &marketSnapshotDoc{
		Date:              s.Date,
		SPYChangePercent:  s.SPYChangePercent,
		QQQChangePercent:  s.QQQChangePercent,
		SectorPerformance: s.SectorPerformance,
		MarketSentiment:   s.MarketSentiment,
		TopMovers:         movers,
	}
}

func toMarketSnapshot(doc *marketSnapshotDoc) *models.MarketSnapshot {
	movers := make([]models.TickerSnapshot, len(doc.TopMovers))
	for i, m := range doc.TopMovers {
		movers[i] = models.TickerSnapshot{Symbol: m.Symbol, ChangePercent: m.ChangePercent, Price: m.Price}
	}
	return &models.MarketSnapshot{
		Date:              doc.Date,
		SPYChangePercent:  doc.SPYChangePercent,
		QQQChangePercent:  doc.QQQChangePercent,
		SectorPerformance: doc.SectorPerformance,
		MarketSentiment:   doc.MarketSentiment,
		TopMovers:         movers,
	}
}

func fromAllocations(allocations []models.Allocation) []allocationDoc {
	docs := make([]allocationDoc, len(allocations))
	for i, a := range allocations {
		docs[i] = allocationDoc{
			Ticker:     a.Ticker,
			Name:       a.Name,
			Type:       a.Type,
			Amount:     a.Amount,
			Percentage: a.Percentage,
			Rationale:  a.Rationale,
		}
	}
	return docs
}

func toAllocations(docs []allocationDoc) []models.Allocation {
	allocations := make([]models.Allocation, len(docs))
	for i, d := range docs {
		allocations[i] = models.Allocation{
			Ticker:     d.Ticker,
			Name:       d.Name,
			Type:       d.Type,
			Amount:     d.Amount,
			Percentage: d.Percentage,
			Rationale:  d.Rationale,
		}
	}
	return allocations
}

func fromReceipts(receipts []models.TradeReceipt) []tradeReceiptDoc {
	docs := make([]tradeReceiptDoc, len(receipts))
	for i, r := range receipts {
		docs[i] = tradeReceiptDoc{
			OrderID:      r.OrderID,
			Ticker:       r.Ticker,
			FilledAmount: r.FilledAmount,
			FilledPrice:  r.FilledPrice,
			Status:       r.Status,
			Timestamp:    r.Timestamp,
		}
	}
	return docs
}

func toReceipts(docs []tradeReceiptDoc) []models.TradeReceipt {
	receipts := make([]models.TradeReceipt, len(docs))
	for i, d := range docs {
		receipts[i] = models.TradeReceipt{
			OrderID:      d.OrderID,
			Ticker:       d.Ticker,
			FilledAmount: d.FilledAmount,
			FilledPrice:  d.FilledPrice,
			Status:       d.Status,
			Timestamp:    d.Timestamp,
		}
	}
	return receipts
}
