// infrastructure/db/mongo_profile_repository.go
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

// plaidConnectionDoc mirrors the MongoDB subdocument for a Plaid connection.
// AccessToken is stored encrypted; it must be decrypted by the repository before returning to callers.
type plaidConnectionDoc struct {
	Institution string `bson:"institution"`
	AccessToken string `bson:"access_token"` // encrypted at rest
	ItemID      string `bson:"item_id"`
}

// profileDocument is the MongoDB-specific representation.
// bson tags are intentionally isolated here and never appear in domain models.
// PlaidConnections uses omitempty so fromProfile (used in $set) never touches the plaid_connections array.
// AutoInvestEnabled uses omitempty (bool) so fromProfile's zero value is omitted — $set never resets the flag.
type profileDocument struct {
	UserID                    string               `bson:"user_id"`
	FullName                  string               `bson:"full_name"`
	Salary                    float64              `bson:"salary"`
	MonthlySavings            float64              `bson:"monthly_savings"`
	RetirementContributionPct float64              `bson:"retirement_contribution_percent"`
	ExistingPortfolioValue    float64              `bson:"existing_portfolio_value"`
	TimeHorizon               string               `bson:"time_horizon"`
	ImmigrationStatus         string               `bson:"immigration_status"`
	RiskTolerance             string               `bson:"risk_tolerance"`
	InvestmentGoal            string               `bson:"investment_goal"`
	HasEmergencyFund          bool                 `bson:"has_emergency_fund"`
	AutoInvestEnabled         bool                 `bson:"auto_invest_enabled,omitempty"`
	AutoInvestEnabledAt       time.Time            `bson:"auto_invest_enabled_at,omitempty"`
	PlaidConnections          []plaidConnectionDoc `bson:"plaid_connections,omitempty"`
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

	// fromProfile never sets PlaidConnections (omitempty, nil slice), so $set never touches plaid_connections.
	doc := fromProfile(profile)
	filter := bson.M{"user_id": profile.UserID}
	update := bson.M{"$set": doc}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: upsert: %w", err)
	}
	return nil
}

// SavePlaidConnection appends a new connection to the user document.
// AccessToken is encrypted before writing so it is never stored in plaintext.
func (r *MongoProfileRepository) SavePlaidConnection(ctx context.Context, userID string, connection models.PlaidConnection) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	encryptedToken, err := EncryptToken(connection.AccessToken)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt access token: %w", err)
	}

	doc := plaidConnectionDoc{
		Institution: connection.Institution,
		AccessToken: encryptedToken,
		ItemID:      connection.ItemID,
	}

	filter := bson.M{"user_id": userID}
	update := bson.M{"$push": bson.M{"plaid_connections": doc}}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: save plaid connection: %w", err)
	}
	return nil
}

// GetPlaidConnections returns all connections for the user with decrypted access tokens.
func (r *MongoProfileRepository) GetPlaidConnections(ctx context.Context, userID string) ([]models.PlaidConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var doc profileDocument
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil // no profile yet means no connections
	}
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: get plaid connections: %w", err)
	}

	connections := make([]models.PlaidConnection, 0, len(doc.PlaidConnections))
	for _, c := range doc.PlaidConnections {
		decrypted, err := DecryptToken(c.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("mongo profile repo: decrypt access token for %s: %w", c.Institution, err)
		}
		connections = append(connections, models.PlaidConnection{
			Institution: c.Institution,
			AccessToken: decrypted,
			ItemID:      c.ItemID,
		})
	}
	return connections, nil
}

// RemovePlaidConnection removes the connection matching itemID using $pull.
func (r *MongoProfileRepository) RemovePlaidConnection(ctx context.Context, userID string, itemID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$pull": bson.M{"plaid_connections": bson.M{"item_id": itemID}}}
	if _, err := r.collection.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("mongo profile repo: remove plaid connection: %w", err)
	}
	return nil
}

func toProfile(doc *profileDocument) *models.UserProfile {
	profile := &models.UserProfile{
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
		AutoInvestEnabled:         doc.AutoInvestEnabled,
		AutoInvestEnabledAt:       doc.AutoInvestEnabledAt,
	}

	// Populate ConnectedAccounts with institution + item_id only; access token is never exposed.
	if len(doc.PlaidConnections) > 0 {
		profile.ConnectedAccounts = make([]models.PlaidConnectionSummary, len(doc.PlaidConnections))
		for i, c := range doc.PlaidConnections {
			profile.ConnectedAccounts[i] = models.PlaidConnectionSummary{
				Institution: c.Institution,
				ItemID:      c.ItemID,
			}
		}
	}
	return profile
}

func fromProfile(p *models.UserProfile) *profileDocument {
	// PlaidConnections and AutoInvestEnabled are intentionally omitted —
	// $set must never overwrite those fields during a normal profile save.
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

// GetAutoInvestUsers returns all profiles where auto_invest_enabled is true.
func (r *MongoProfileRepository) GetAutoInvestUsers(ctx context.Context) ([]models.UserProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cursor, err := r.collection.Find(ctx, bson.M{"auto_invest_enabled": true})
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: get auto-invest users: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []profileDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("mongo profile repo: decode auto-invest users: %w", err)
	}

	profiles := make([]models.UserProfile, len(docs))
	for i, doc := range docs {
		profiles[i] = *toProfile(&doc)
	}
	return profiles, nil
}

// SetAutoInvest updates only the auto_invest_enabled flag and its consent timestamp.
// Uses a targeted $set so no other profile fields are touched.
func (r *MongoProfileRepository) SetAutoInvest(ctx context.Context, userID string, enabled bool, enabledAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{
		"auto_invest_enabled":    enabled,
		"auto_invest_enabled_at": enabledAt,
	}}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: set auto-invest: %w", err)
	}
	return nil
}
