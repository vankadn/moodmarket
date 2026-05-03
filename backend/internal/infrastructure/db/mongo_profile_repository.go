package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// profileDocument is the MongoDB-specific representation.
// bson tags are intentionally isolated here and never appear in domain models.
type profileDocument struct {
	UserID                    string `bson:"user_id"`
	FullName                  string `bson:"full_name"`
	Salary                    float64 `bson:"salary"`
	MonthlySavings            float64 `bson:"monthly_savings"`
	RetirementContributionPct float64 `bson:"retirement_contribution_percent"`
	ExistingPortfolioValue    float64 `bson:"existing_portfolio_value"`
	TimeHorizon               string  `bson:"time_horizon"`
	ImmigrationStatus         string  `bson:"immigration_status"`
	RiskTolerance             string  `bson:"risk_tolerance"`
	InvestmentGoal            string  `bson:"investment_goal"`
	HasEmergencyFund          bool    `bson:"has_emergency_fund"`
}

type MongoProfileRepository struct {
	collection *mongo.Collection
}

func NewMongoProfileRepository(db *mongo.Database) *MongoProfileRepository {
	return &MongoProfileRepository{collection: db.Collection("profiles")}
}

func (r *MongoProfileRepository) GetByUserID(ctx context.Context, userID string) (*models.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var doc profileDocument
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ports.ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: get: %w", err)
	}
	return toProfile(&doc), nil
}

func (r *MongoProfileRepository) Upsert(ctx context.Context, profile *models.UserProfile) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	doc := fromProfile(profile)
	filter := bson.M{"user_id": profile.UserID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: upsert: %w", err)
	}
	return nil
}

func toProfile(doc *profileDocument) *models.UserProfile {
	return &models.UserProfile{
		UserID:                    doc.UserID,
		FullName:                  doc.FullName,
		Salary:                    doc.Salary,
		MonthlySavings:            doc.MonthlySavings,
		RetirementContributionPct: doc.RetirementContributionPct,
		ExistingPortfolioValue:    doc.ExistingPortfolioValue,
		TimeHorizon:               models.TimeHorizon(doc.TimeHorizon),
		ImmigrationStatus:         models.ImmigrationStatus(doc.ImmigrationStatus),
		RiskTolerance:             models.RiskTolerance(doc.RiskTolerance),
		InvestmentGoal:            models.InvestmentGoal(doc.InvestmentGoal),
		HasEmergencyFund:          doc.HasEmergencyFund,
	}
}

func fromProfile(p *models.UserProfile) *profileDocument {
	return &profileDocument{
		UserID:                    p.UserID,
		FullName:                  p.FullName,
		Salary:                    p.Salary,
		MonthlySavings:            p.MonthlySavings,
		RetirementContributionPct: p.RetirementContributionPct,
		ExistingPortfolioValue:    p.ExistingPortfolioValue,
		TimeHorizon:               string(p.TimeHorizon),
		ImmigrationStatus:         string(p.ImmigrationStatus),
		RiskTolerance:             string(p.RiskTolerance),
		InvestmentGoal:            string(p.InvestmentGoal),
		HasEmergencyFund:          p.HasEmergencyFund,
	}
}
