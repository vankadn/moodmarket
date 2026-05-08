// infrastructure/db/mongo_scheduler_repository.go
package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type schedulerRunDoc struct {
	ID             primitive.ObjectID `bson:"_id"`
	RunID          string             `bson:"run_id"`
	StartedAt      time.Time          `bson:"started_at"`
	CompletedAt    time.Time          `bson:"completed_at"`
	UsersProcessed int                `bson:"users_processed"`
	TotalInvested  float64            `bson:"total_invested"`
	Errors         []string           `bson:"errors,omitempty"`
}

type MongoSchedulerRepository struct {
	collection *mongo.Collection
}

func NewMongoSchedulerRepository(db *mongo.Database) *MongoSchedulerRepository {
	return &MongoSchedulerRepository{collection: db.Collection("scheduler_runs")}
}

func (r *MongoSchedulerRepository) Save(ctx context.Context, run *models.SchedulerRun) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	doc := schedulerRunDoc{
		ID:             primitive.NewObjectID(),
		RunID:          run.RunID,
		StartedAt:      run.StartedAt,
		CompletedAt:    run.CompletedAt,
		UsersProcessed: run.UsersProcessed,
		TotalInvested:  run.TotalInvested,
		Errors:         run.Errors,
	}
	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("mongo scheduler repo: save: %w", err)
	}
	return nil
}
