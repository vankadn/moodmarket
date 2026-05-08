// infrastructure/db/mongo_auto_invest_repository.go
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type autoInvestConfigDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	UserID    string             `bson:"user_id"`
	Enabled   bool               `bson:"enabled"`
	Amount    float64            `bson:"amount"`
	Risk      string             `bson:"risk"`
	EnabledAt time.Time          `bson:"enabled_at,omitempty"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type MongoAutoInvestRepository struct {
	collection *mongo.Collection
}

func NewMongoAutoInvestRepository(db *mongo.Database) *MongoAutoInvestRepository {
	return &MongoAutoInvestRepository{collection: db.Collection("auto_invest_configs")}
}

// GetByUserID returns the stored config, or a safe default if none exists.
func (r *MongoAutoInvestRepository) GetByUserID(ctx context.Context, userID string) (*models.AutoInvestConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var doc autoInvestConfigDoc
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &models.AutoInvestConfig{
			UserID:  userID,
			Enabled: false,
			Amount:  100,
			Risk:    models.RiskModerate,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: get: %w", err)
	}
	return toAutoInvestConfig(&doc), nil
}

func (r *MongoAutoInvestRepository) Upsert(ctx context.Context, config *models.AutoInvestConfig) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	config.UpdatedAt = time.Now()

	doc := bson.M{
		"user_id":    config.UserID,
		"enabled":    config.Enabled,
		"amount":     config.Amount,
		"risk":       string(config.Risk),
		"updated_at": config.UpdatedAt,
	}
	if config.Enabled && !config.EnabledAt.IsZero() {
		doc["enabled_at"] = config.EnabledAt
	}

	filter := bson.M{"user_id": config.UserID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	result, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("mongo auto-invest repo: upsert: %w", err)
	}
	if result.UpsertedID != nil {
		config.ID = result.UpsertedID.(primitive.ObjectID).Hex()
	}
	return nil
}

func (r *MongoAutoInvestRepository) GetAllEnabled(ctx context.Context) ([]models.AutoInvestConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"enabled": true})
	if err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: get enabled: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []autoInvestConfigDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: decode: %w", err)
	}

	configs := make([]models.AutoInvestConfig, len(docs))
	for i, doc := range docs {
		configs[i] = *toAutoInvestConfig(&doc)
	}
	return configs, nil
}

func toAutoInvestConfig(doc *autoInvestConfigDoc) *models.AutoInvestConfig {
	return &models.AutoInvestConfig{
		ID:        doc.ID.Hex(),
		UserID:    doc.UserID,
		Enabled:   doc.Enabled,
		Amount:    doc.Amount,
		Risk:      models.RiskTolerance(doc.Risk),
		EnabledAt: doc.EnabledAt,
		UpdatedAt: doc.UpdatedAt,
	}
}
