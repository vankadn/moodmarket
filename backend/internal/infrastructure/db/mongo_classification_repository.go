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

type classificationDoc struct {
	Ticker            string    `bson:"ticker"`
	AssetClass        string    `bson:"asset_class"`
	Approved          bool      `bson:"approved"`
	SuggestedByClaude bool      `bson:"suggested_by_claude"`
	FirstSeenAt       time.Time `bson:"first_seen_at"`
}

// MongoClassificationRepository persists ticker→asset-class mappings.
type MongoClassificationRepository struct {
	collection *mongo.Collection
}

func NewMongoClassificationRepository(db *mongo.Database) *MongoClassificationRepository {
	coll := db.Collection("ticker_classifications")
	// Unique index on ticker — enforces one record per symbol.
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "ticker", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &MongoClassificationRepository{collection: coll}
}

func (r *MongoClassificationRepository) Seed(ctx context.Context, entries []models.ClassificationEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	writeModels := make([]mongo.WriteModel, len(entries))
	for i, e := range entries {
		doc := bson.M{
			"ticker":              e.Ticker,
			"asset_class":         e.AssetClass,
			"approved":            e.Approved,
			"suggested_by_claude": e.SuggestedByClaude,
			"first_seen_at":       e.FirstSeenAt,
		}
		writeModels[i] = mongo.NewUpdateOneModel().
			SetFilter(bson.M{"ticker": e.Ticker}).
			SetUpdate(bson.M{"$setOnInsert": doc}).
			SetUpsert(true)
	}

	_, err := r.collection.BulkWrite(ctx, writeModels, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return fmt.Errorf("mongo classification repo: seed: %w", err)
	}
	return nil
}

func (r *MongoClassificationRepository) LoadApproved(ctx context.Context) ([]models.ClassificationEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"approved": true})
	if err != nil {
		return nil, fmt.Errorf("mongo classification repo: load approved: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []classificationDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo classification repo: decode: %w", err)
	}

	entries := make([]models.ClassificationEntry, len(docs))
	for i, d := range docs {
		entries[i] = models.ClassificationEntry{
			Ticker:            d.Ticker,
			AssetClass:        d.AssetClass,
			Approved:          d.Approved,
			SuggestedByClaude: d.SuggestedByClaude,
			FirstSeenAt:       d.FirstSeenAt,
		}
	}
	return entries, nil
}

// QueueUnknown records a Claude-suggested classification for manual review.
// Uses $setOnInsert so an already-approved ticker is never downgraded.
func (r *MongoClassificationRepository) QueueUnknown(ctx context.Context, ticker, suggestedClass string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"ticker": ticker},
		bson.M{"$setOnInsert": bson.M{
			"ticker":              ticker,
			"asset_class":         suggestedClass,
			"approved":            false,
			"suggested_by_claude": true,
			"first_seen_at":       time.Now(),
		}},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo classification repo: queue unknown %s: %w", ticker, err)
	}
	return nil
}
