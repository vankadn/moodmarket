// infrastructure/db/mongo_rebalance_repository.go
package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// --- BSON document types (never imported by domain or application layers) ---

type rebalanceAnalysisDoc struct {
	UserID                 string               `bson:"user_id"`
	GeneratedAt            time.Time            `bson:"generated_at"`
	PortfolioHealthSummary string               `bson:"portfolio_health_summary"`
	Insights               []positionInsightDoc `bson:"insights"`
}

type positionInsightDoc struct {
	Ticker            string  `bson:"ticker"`
	Name              string  `bson:"name"`
	Source            string  `bson:"source"`
	AccountName       string  `bson:"account_name"`
	CurrentValue      float64 `bson:"current_value"`
	UnrealizedPL      float64 `bson:"unrealized_pl"`
	UnrealizedPLPct   float64 `bson:"unrealized_pl_pct"`
	OriginalBuyThesis string  `bson:"original_buy_thesis,omitempty"`
	ClaudeAssessment  string  `bson:"claude_assessment"`
	SuggestedAction   string  `bson:"suggested_action"`
	TaxFlag           string  `bson:"tax_flag"`
}

// --- Repository ---

type MongoRebalanceRepository struct {
	collection *mongo.Collection
}

func NewMongoRebalanceRepository(db *mongo.Database) *MongoRebalanceRepository {
	coll := db.Collection("rebalance_analyses")
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "generated_at", Value: -1},
		},
	})
	return &MongoRebalanceRepository{collection: coll}
}

// SaveAnalysis upserts the analysis for the user — one document per user, always the latest.
func (r *MongoRebalanceRepository) SaveAnalysis(ctx context.Context, analysis *models.RebalanceAnalysis) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	insights := make([]positionInsightDoc, len(analysis.Insights))
	for i, ins := range analysis.Insights {
		insights[i] = positionInsightDoc{
			Ticker:            ins.Ticker,
			Name:              ins.Name,
			Source:            ins.Source,
			AccountName:       ins.AccountName,
			CurrentValue:      ins.CurrentValue,
			UnrealizedPL:      ins.UnrealizedPL,
			UnrealizedPLPct:   ins.UnrealizedPLPct,
			OriginalBuyThesis: ins.OriginalBuyThesis,
			ClaudeAssessment:  ins.ClaudeAssessment,
			SuggestedAction:   string(ins.SuggestedAction),
			TaxFlag:           string(ins.TaxFlag),
		}
	}

	doc := rebalanceAnalysisDoc{
		UserID:                 analysis.UserID,
		GeneratedAt:            analysis.GeneratedAt,
		PortfolioHealthSummary: analysis.PortfolioHealthSummary,
		Insights:               insights,
	}

	filter := bson.D{{Key: "user_id", Value: analysis.UserID}}
	_, err := r.collection.ReplaceOne(ctx, filter, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongo rebalance repo: save: %w", err)
	}
	return nil
}

// GetLatestAnalysis returns the most recent analysis for the user, or nil, nil if none exists.
func (r *MongoRebalanceRepository) GetLatestAnalysis(ctx context.Context, userID string) (*models.RebalanceAnalysis, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "user_id", Value: userID}}
	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var doc rebalanceAnalysisDoc
	if err := r.collection.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("mongo rebalance repo: get latest: %w", err)
	}

	insights := make([]models.PositionInsight, len(doc.Insights))
	for i, ins := range doc.Insights {
		insights[i] = models.PositionInsight{
			Ticker:            ins.Ticker,
			Name:              ins.Name,
			Source:            ins.Source,
			AccountName:       ins.AccountName,
			CurrentValue:      ins.CurrentValue,
			UnrealizedPL:      ins.UnrealizedPL,
			UnrealizedPLPct:   ins.UnrealizedPLPct,
			OriginalBuyThesis: ins.OriginalBuyThesis,
			ClaudeAssessment:  ins.ClaudeAssessment,
			SuggestedAction:   models.SuggestedAction(ins.SuggestedAction),
			TaxFlag:           models.TaxFlag(ins.TaxFlag),
		}
	}

	return &models.RebalanceAnalysis{
		UserID:                 doc.UserID,
		GeneratedAt:            doc.GeneratedAt,
		PortfolioHealthSummary: doc.PortfolioHealthSummary,
		Insights:               insights,
	}, nil
}
