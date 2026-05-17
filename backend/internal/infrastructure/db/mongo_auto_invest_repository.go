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
	ID           primitive.ObjectID `bson:"_id"`
	UserID       string             `bson:"user_id"`
	Name         string             `bson:"name,omitempty"`
	Enabled      bool               `bson:"enabled"`
	Amount       float64            `bson:"amount"`
	Risk         string             `bson:"risk"`
	Strategy     string             `bson:"strategy,omitempty"`
	IntervalDays int                `bson:"interval_days,omitempty"`
	EnabledAt    time.Time          `bson:"enabled_at,omitempty"`
	UpdatedAt    time.Time          `bson:"updated_at"`
	LastRunAt    *time.Time         `bson:"last_run_at,omitempty"`
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
		"user_id":      config.UserID,
		"enabled":      config.Enabled,
		"amount":       config.Amount,
		"risk":         string(config.Risk),
		"interval_days": config.IntervalDays,
		"updated_at":   config.UpdatedAt,
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
		ID:           doc.ID.Hex(),
		UserID:       doc.UserID,
		Name:         doc.Name,
		Enabled:      doc.Enabled,
		Amount:       doc.Amount,
		Risk:         models.RiskTolerance(doc.Risk),
		Strategy:     doc.Strategy,
		IntervalDays: doc.IntervalDays,
		EnabledAt:    doc.EnabledAt,
		UpdatedAt:    doc.UpdatedAt,
		LastRunAt:    doc.LastRunAt,
	}
}

func (r *MongoAutoInvestRepository) StampLastRunAt(ctx context.Context, configID string, t time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		return fmt.Errorf("mongo auto-invest repo: stamp: invalid id %q: %w", configID, err)
	}
	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"last_run_at": t}})
	if err != nil {
		return fmt.Errorf("mongo auto-invest repo: stamp last run: %w", err)
	}
	return nil
}

func (r *MongoAutoInvestRepository) GetAllByUserID(ctx context.Context, userID string) ([]models.AutoInvestConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: get all by user: %w", err)
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

func (r *MongoAutoInvestRepository) Create(ctx context.Context, config *models.AutoInvestConfig) (*models.AutoInvestConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	config.UpdatedAt = now
	if config.IntervalDays <= 0 {
		config.IntervalDays = 1
	}

	doc := autoInvestConfigDoc{
		ID:           primitive.NewObjectID(),
		UserID:       config.UserID,
		Name:         config.Name,
		Enabled:      config.Enabled,
		Amount:       config.Amount,
		Risk:         string(config.Risk),
		Strategy:     config.Strategy,
		IntervalDays: config.IntervalDays,
		EnabledAt:    config.EnabledAt,
		UpdatedAt:    config.UpdatedAt,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: create: %w", err)
	}
	config.ID = doc.ID.Hex()
	return config, nil
}

func (r *MongoAutoInvestRepository) UpdateByID(ctx context.Context, configID, userID string, config *models.AutoInvestConfig) (*models.AutoInvestConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: update: invalid id %q: %w", configID, err)
	}

	config.UpdatedAt = time.Now()

	setFields := bson.M{
		"name":          config.Name,
		"enabled":       config.Enabled,
		"amount":        config.Amount,
		"risk":          string(config.Risk),
		"strategy":      config.Strategy,
		"interval_days": config.IntervalDays,
		"updated_at":    config.UpdatedAt,
	}
	update := bson.M{"$set": setFields}
	if config.Enabled && !config.EnabledAt.IsZero() {
		setFields["enabled_at"] = config.EnabledAt
	} else if !config.Enabled {
		update["$unset"] = bson.M{"enabled_at": ""}
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": oid, "user_id": userID}, update)
	if err != nil {
		return nil, fmt.Errorf("mongo auto-invest repo: update: %w", err)
	}
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("mongo auto-invest repo: update: config not found")
	}
	config.ID = configID
	config.UserID = userID
	return config, nil
}

func (r *MongoAutoInvestRepository) DeleteByID(ctx context.Context, configID, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(configID)
	if err != nil {
		return fmt.Errorf("mongo auto-invest repo: delete: invalid id %q: %w", configID, err)
	}
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return fmt.Errorf("mongo auto-invest repo: delete: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("mongo auto-invest repo: delete: config not found")
	}
	return nil
}
