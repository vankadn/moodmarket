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
	ConfigID       string              `bson:"config_id,omitempty"`
	Timestamp      time.Time           `bson:"timestamp"`
	MarketSnapshot *marketSnapshotDoc  `bson:"market_snapshot,omitempty"`
	Allocations    []allocationDoc     `bson:"allocations"`
	Receipts       []tradeReceiptDoc   `bson:"receipts,omitempty"`
	TotalAmount    float64             `bson:"total_amount"`
	RiskLevel      string              `bson:"risk_level"`
	Summary        string              `bson:"summary"`
	Verdict        *decisionVerdictDoc `bson:"verdict,omitempty"`
}

type decisionVerdictDoc struct {
	StampedAt        time.Time          `bson:"stamped_at"`
	OverallReturnPct float64            `bson:"overall_return_pct"`
	SPYReturnPct     float64            `bson:"spy_return_pct"`
	BeatMarket       bool               `bson:"beat_market"`
	TickerVerdicts   []tickerVerdictDoc `bson:"ticker_verdicts"`
}

type tickerVerdictDoc struct {
	Ticker           string    `bson:"ticker"`
	EntryPrice       float64   `bson:"entry_price"`
	PrevDayPrice     float64   `bson:"prev_day_price"`
	PrevDayTimestamp time.Time `bson:"prev_day_timestamp"`
	CurrentPrice     float64   `bson:"current_price"`
	CurrentTimestamp time.Time `bson:"current_timestamp"`
	ReturnPct        float64   `bson:"return_pct"`
	TodayChangePct   float64   `bson:"today_change_pct"`
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
	SPYPrice          float64             `bson:"spy_price,omitempty"`
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
	coll := db.Collection("decisions")
	// Compound index covers per-strategy aggregation (user_id + config_id) and date-range
	// queries (user_id + timestamp). Idempotent on restart.
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "config_id", Value: 1},
			{Key: "timestamp", Value: -1},
		},
	})
	return &MongoDecisionRepository{collection: coll}
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

func (r *MongoDecisionRepository) ListByUserSince(ctx context.Context, userID string, since *time.Time) ([]models.InvestmentDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	if since != nil {
		filter["timestamp"] = bson.M{"$gte": *since}
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: list since: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []decisionDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo decision repo: decode since: %w", err)
	}

	decisions := make([]models.InvestmentDecision, len(docs))
	for i, doc := range docs {
		decisions[i] = toDecision(&doc)
	}
	return decisions, nil
}

func (r *MongoDecisionRepository) ActivityByStrategy(ctx context.Context, userID string) ([]models.StrategyActivity, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$config_id"},
			{Key: "total_invested", Value: bson.M{"$sum": "$total_amount"}},
			{Key: "decision_count", Value: bson.M{"$sum": 1}},
			{Key: "first_run_at", Value: bson.M{"$min": "$timestamp"}},
			{Key: "last_run_at", Value: bson.M{"$max": "$timestamp"}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "last_run_at", Value: -1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: activity by strategy: %w", err)
	}
	defer cursor.Close(ctx)

	type aggResult struct {
		ConfigID      string    `bson:"_id"`
		TotalInvested float64   `bson:"total_invested"`
		DecisionCount int       `bson:"decision_count"`
		FirstRunAt    time.Time `bson:"first_run_at"`
		LastRunAt     time.Time `bson:"last_run_at"`
	}

	var raw []aggResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("mongo decision repo: activity by strategy decode: %w", err)
	}

	activities := make([]models.StrategyActivity, len(raw))
	for i, a := range raw {
		activities[i] = models.StrategyActivity{
			ConfigID:      a.ConfigID,
			TotalInvested: a.TotalInvested,
			DecisionCount: a.DecisionCount,
			FirstRunAt:    a.FirstRunAt,
			LastRunAt:     a.LastRunAt,
		}
	}
	return activities, nil
}

func (r *MongoDecisionRepository) CostBasisByStrategy(ctx context.Context, userID string) (map[string]map[string]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		{{Key: "$unwind", Value: "$allocations"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "config_id", Value: "$config_id"},
				{Key: "ticker", Value: "$allocations.ticker"},
			}},
			{Key: "amount", Value: bson.M{"$sum": "$allocations.amount"}},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: cost basis by strategy: %w", err)
	}
	defer cursor.Close(ctx)

	type aggResult struct {
		ID struct {
			ConfigID string `bson:"config_id"`
			Ticker   string `bson:"ticker"`
		} `bson:"_id"`
		Amount float64 `bson:"amount"`
	}

	var raw []aggResult
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("mongo decision repo: cost basis by strategy decode: %w", err)
	}

	result := make(map[string]map[string]float64)
	for _, a := range raw {
		if result[a.ID.ConfigID] == nil {
			result[a.ID.ConfigID] = make(map[string]float64)
		}
		result[a.ID.ConfigID][a.ID.Ticker] = a.Amount
	}
	return result, nil
}

func (r *MongoDecisionRepository) StampVerdict(ctx context.Context, decisionID string, verdict *models.DecisionVerdict) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(decisionID)
	if err != nil {
		return fmt.Errorf("mongo decision repo: invalid decision id %q: %w", decisionID, err)
	}

	// Write if: no verdict, empty ticker data, abnormal return (Inf/NaN), or has ticker data but overall=0 (fill-amount bug).
	noVerdictFilter := bson.M{"_id": oid, "$or": bson.A{
		bson.M{"verdict": bson.M{"$exists": false}},
		bson.M{"verdict.ticker_verdicts": bson.M{"$size": 0}},
		bson.M{"verdict.overall_return_pct": bson.M{"$gt": 1e15}},
		bson.M{"verdict.overall_return_pct": bson.M{"$lt": -1e15}},
		bson.M{"verdict.overall_return_pct": 0, "verdict.ticker_verdicts.0": bson.M{"$exists": true}},
	}}
	update := bson.M{"$set": bson.M{"verdict": fromVerdict(verdict)}}
	_, err = r.collection.UpdateOne(ctx, noVerdictFilter, update)
	if err != nil {
		return fmt.Errorf("mongo decision repo: stamp verdict %s: %w", decisionID, err)
	}
	return nil
}

func (r *MongoDecisionRepository) ListUnverdicted(ctx context.Context, userID string, minAge time.Duration) ([]models.InvestmentDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// "No verdict yet" is age-gated — don't evaluate too quickly.
	// Bad-verdict conditions (empty tickers, Inf, zero-overall-with-data) are NOT age-gated —
	// they must be corrected regardless of how recent the decision is.
	badVerdict := bson.A{
		bson.M{"verdict.ticker_verdicts": bson.M{"$size": 0}},
		bson.M{"verdict.overall_return_pct": bson.M{"$gt": 1e15}},
		bson.M{"verdict.overall_return_pct": bson.M{"$lt": -1e15}},
		bson.M{"verdict.overall_return_pct": 0, "verdict.ticker_verdicts.0": bson.M{"$exists": true}},
	}
	noVerdictYet := bson.M{"verdict": bson.M{"$exists": false}}
	if minAge > 0 {
		cutoff := time.Now().Add(-minAge)
		noVerdictYet["timestamp"] = bson.M{"$lt": cutoff}
	}
	filter := bson.M{
		"user_id": userID,
		"$or":     append(bson.A{noVerdictYet}, badVerdict...),
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: list unverdicted: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []decisionDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo decision repo: decode unverdicted: %w", err)
	}

	decisions := make([]models.InvestmentDecision, len(docs))
	for i, doc := range docs {
		decisions[i] = toDecision(&doc)
	}
	return decisions, nil
}

func (r *MongoDecisionRepository) GetUsersWithPendingVerdicts(ctx context.Context, minAge time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	badVerdictConds := bson.A{
		bson.M{"verdict.ticker_verdicts": bson.M{"$size": 0}},
		bson.M{"verdict.overall_return_pct": bson.M{"$gt": 1e15}},
		bson.M{"verdict.overall_return_pct": bson.M{"$lt": -1e15}},
		bson.M{"verdict.overall_return_pct": 0, "verdict.ticker_verdicts.0": bson.M{"$exists": true}},
	}
	noVerdictCond := bson.M{"verdict": bson.M{"$exists": false}}
	if minAge > 0 {
		noVerdictCond["timestamp"] = bson.M{"$lt": time.Now().Add(-minAge)}
	}
	filter := bson.M{"$or": append(bson.A{noVerdictCond}, badVerdictConds...)}
	raw, err := r.collection.Distinct(ctx, "user_id", filter)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: distinct users with pending verdicts: %w", err)
	}

	userIDs := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			userIDs = append(userIDs, s)
		}
	}
	return userIDs, nil
}

func (r *MongoDecisionRepository) GetEvalSummary(ctx context.Context, userID string) (*models.EvalSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Total decisions (all, with or without verdict)
	total, err := r.collection.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: eval total count: %w", err)
	}

	verdictedFilter := bson.M{"user_id": userID, "verdict": bson.M{"$exists": true}}

	// Aggregate win rate + avg returns across all verdicted decisions
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: verdictedFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "count", Value: bson.M{"$sum": 1}},
			{Key: "win_count", Value: bson.M{"$sum": bson.M{"$cond": bson.A{"$verdict.beat_market", 1, 0}}}},
			{Key: "sum_return", Value: bson.M{"$sum": "$verdict.overall_return_pct"}},
			{Key: "sum_spy", Value: bson.M{"$sum": "$verdict.spy_return_pct"}},
		}}},
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: eval aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	type aggResult struct {
		Count     int     `bson:"count"`
		WinCount  int     `bson:"win_count"`
		SumReturn float64 `bson:"sum_return"`
		SumSPY    float64 `bson:"sum_spy"`
	}
	var agg []aggResult
	if err := cursor.All(ctx, &agg); err != nil {
		return nil, fmt.Errorf("mongo decision repo: eval aggregate decode: %w", err)
	}

	summary := &models.EvalSummary{TotalDecisions: int(total)}
	if len(agg) == 0 {
		return summary, nil // no verdicts yet
	}
	a := agg[0]
	summary.VerdictedDecisions = a.Count
	if a.Count > 0 {
		summary.WinRate = float64(a.WinCount) / float64(a.Count)
		summary.AvgReturnPct = a.SumReturn / float64(a.Count)
		summary.AvgSPYReturnPct = a.SumSPY / float64(a.Count)
	}

	// Best decision
	if best, err := r.findExtreme(ctx, verdictedFilter, -1); err == nil && best != nil {
		summary.BestDecision = best
	}
	// Worst decision
	if worst, err := r.findExtreme(ctx, verdictedFilter, 1); err == nil && worst != nil {
		summary.WorstDecision = worst
	}

	// Per-strategy breakdown
	// Normalize empty config_id ("") to "manual" so legacy and manual decisions merge into one group.
	byStratPipeline := mongo.Pipeline{
		{{Key: "$match", Value: verdictedFilter}},
		{{Key: "$addFields", Value: bson.M{
			"_norm_config": bson.M{"$cond": bson.A{
				bson.M{"$or": bson.A{
					bson.M{"$eq": bson.A{"$config_id", ""}},
					bson.M{"$eq": bson.A{"$config_id", nil}},
				}},
				"manual",
				"$config_id",
			}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_norm_config"},
			{Key: "count", Value: bson.M{"$sum": 1}},
			{Key: "win_count", Value: bson.M{"$sum": bson.M{"$cond": bson.A{"$verdict.beat_market", 1, 0}}}},
			{Key: "sum_return", Value: bson.M{"$sum": "$verdict.overall_return_pct"}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
	}
	sc, err := r.collection.Aggregate(ctx, byStratPipeline)
	if err == nil {
		defer sc.Close(ctx)
		type stratResult struct {
			ConfigID  string  `bson:"_id"`
			Count     int     `bson:"count"`
			WinCount  int     `bson:"win_count"`
			SumReturn float64 `bson:"sum_return"`
		}
		var stratRows []stratResult
		if sc.All(ctx, &stratRows) == nil {
			for _, s := range stratRows {
				se := models.StrategyEval{
					ConfigID:      s.ConfigID,
					DecisionCount: s.Count,
				}
				if s.Count > 0 {
					se.WinRate = float64(s.WinCount) / float64(s.Count)
					se.AvgReturnPct = s.SumReturn / float64(s.Count)
				}
				summary.ByStrategy = append(summary.ByStrategy, se)
			}
		}
	}

	return summary, nil
}

// findExtreme returns a lightweight ref to the decision with the max (sortDir=-1) or min (sortDir=1) return_pct.
func (r *MongoDecisionRepository) findExtreme(ctx context.Context, filter bson.M, sortDir int) (*models.EvalDecisionRef, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "verdict.overall_return_pct", Value: sortDir}})
	var doc decisionDocument
	if err := r.collection.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return nil, err
	}
	ref := &models.EvalDecisionRef{
		ID:     doc.ID.Hex(),
		Date:   doc.Timestamp,
		Amount: doc.TotalAmount,
	}
	if doc.Verdict != nil {
		ref.ReturnPct = doc.Verdict.OverallReturnPct
	}
	return ref, nil
}

func (r *MongoDecisionRepository) ListVerdictedDecisions(ctx context.Context, userID string, page, limit int) ([]models.InvestmentDecision, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	filter := bson.M{"user_id": userID, "verdict": bson.M{"$exists": true}}
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo decision repo: list verdicted: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []decisionDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo decision repo: list verdicted decode: %w", err)
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
		ConfigID:    d.ConfigID,
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
	if d.Verdict != nil {
		doc.Verdict = fromVerdict(d.Verdict)
	}
	return doc
}

func toDecision(doc *decisionDocument) models.InvestmentDecision {
	d := models.InvestmentDecision{
		ID:          doc.ID.Hex(),
		UserID:      doc.UserID,
		ConfigID:    doc.ConfigID,
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
	if doc.Verdict != nil {
		d.Verdict = toVerdict(doc.Verdict)
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
		SPYPrice:          s.SPYPrice,
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
		SPYPrice:          doc.SPYPrice,
		SPYChangePercent:  doc.SPYChangePercent,
		QQQChangePercent:  doc.QQQChangePercent,
		SectorPerformance: doc.SectorPerformance,
		MarketSentiment:   doc.MarketSentiment,
		TopMovers:         movers,
	}
}

func fromVerdict(v *models.DecisionVerdict) *decisionVerdictDoc {
	tvs := make([]tickerVerdictDoc, len(v.TickerVerdicts))
	for i, t := range v.TickerVerdicts {
		tvs[i] = tickerVerdictDoc{
			Ticker:           t.Ticker,
			EntryPrice:       t.EntryPrice,
			PrevDayPrice:     t.PrevDayPrice,
			PrevDayTimestamp: t.PrevDayTimestamp,
			CurrentPrice:     t.CurrentPrice,
			CurrentTimestamp: t.CurrentTimestamp,
			ReturnPct:        t.ReturnPct,
			TodayChangePct:   t.TodayChangePct,
		}
	}
	return &decisionVerdictDoc{
		StampedAt:        v.StampedAt,
		OverallReturnPct: v.OverallReturnPct,
		SPYReturnPct:     v.SPYReturnPct,
		BeatMarket:       v.BeatMarket,
		TickerVerdicts:   tvs,
	}
}

func toVerdict(doc *decisionVerdictDoc) *models.DecisionVerdict {
	tvs := make([]models.TickerVerdict, len(doc.TickerVerdicts))
	for i, t := range doc.TickerVerdicts {
		tvs[i] = models.TickerVerdict{
			Ticker:           t.Ticker,
			EntryPrice:       t.EntryPrice,
			PrevDayPrice:     t.PrevDayPrice,
			PrevDayTimestamp: t.PrevDayTimestamp,
			CurrentPrice:     t.CurrentPrice,
			CurrentTimestamp: t.CurrentTimestamp,
			ReturnPct:        t.ReturnPct,
			TodayChangePct:   t.TodayChangePct,
		}
	}
	return &models.DecisionVerdict{
		StampedAt:        doc.StampedAt,
		OverallReturnPct: doc.OverallReturnPct,
		SPYReturnPct:     doc.SPYReturnPct,
		BeatMarket:       doc.BeatMarket,
		TickerVerdicts:   tvs,
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
