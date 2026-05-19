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
	_, _ = coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "ticker", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &MongoClassificationRepository{collection: coll}
}

// LoadAll returns every ticker in the collection regardless of approved status.
func (r *MongoClassificationRepository) LoadAll(ctx context.Context) ([]models.ClassificationEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{})
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

// StoreClassification upserts a Claude-classified ticker as approved:true.
// $set updates the class and approval on every call; $setOnInsert preserves first_seen_at.
func (r *MongoClassificationRepository) StoreClassification(ctx context.Context, ticker, assetClass string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.collection.UpdateOne(ctx,
		bson.M{"ticker": ticker},
		bson.D{
			{Key: "$set", Value: bson.M{
				"asset_class":         assetClass,
				"approved":            true,
				"suggested_by_claude": true,
			}},
			{Key: "$setOnInsert", Value: bson.M{
				"ticker":        ticker,
				"first_seen_at": time.Now(),
			}},
		},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongo classification repo: store %s: %w", ticker, err)
	}
	return nil
}
