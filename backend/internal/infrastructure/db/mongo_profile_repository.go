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

// brokerageConnectionDoc mirrors the MongoDB subdocument for an Alpaca connection.
// APIKey and SecretKey are stored encrypted; decrypted by the repository before returning to callers.
type brokerageConnectionDoc struct {
	ID              string    `bson:"id,omitempty"`
	Name            string    `bson:"name,omitempty"`
	AssetCategories []string  `bson:"asset_categories,omitempty"`
	APIKey          string    `bson:"api_key"`    // AES-256-GCM encrypted
	SecretKey       string    `bson:"secret_key"` // AES-256-GCM encrypted
	BaseURL         string    `bson:"base_url"`
	Connected       bool      `bson:"connected"`
	ConnectedAt     time.Time `bson:"connected_at"`
}

// portfolioConnectionDoc stores credentials for an external portfolio aggregator.
// Both fields are AES-256-GCM encrypted; ProviderUserSecret is a long-lived HMAC signing key — never log it.
type portfolioConnectionDoc struct {
	Provider           string    `bson:"provider"`
	ProviderUserID     string    `bson:"provider_user_id"`     // AES-256-GCM encrypted
	ProviderUserSecret string    `bson:"provider_user_secret"` // AES-256-GCM encrypted HMAC key; never log
	ConnectedAt        time.Time `bson:"connected_at"`
}

// profileDocument is the MongoDB-specific representation.
// bson tags are intentionally isolated here and never appear in domain models.
// PlaidConnections, BrokerageConn, BrokerageConns, and PortfolioConn use omitempty so fromProfile ($set) never touches them.
type profileDocument struct {
	UserID                    string                   `bson:"user_id"`
	FullName                  string                   `bson:"full_name"`
	Salary                    float64                  `bson:"salary"`
	MonthlySavings            float64                  `bson:"monthly_savings"`
	RetirementContributionPct float64                  `bson:"retirement_contribution_percent"`
	ExistingPortfolioValue    float64                  `bson:"existing_portfolio_value"`
	TimeHorizon               string                   `bson:"time_horizon"`
	ImmigrationStatus         string                   `bson:"immigration_status"`
	RiskTolerance             string                   `bson:"risk_tolerance"`
	InvestmentGoal            string                   `bson:"investment_goal"`
	HasEmergencyFund          bool                     `bson:"has_emergency_fund"`
	IncludeCashContext        bool                     `bson:"include_cash_context"`
	NotificationEmail         string                   `bson:"notification_email,omitempty"`
	Phone                     string                   `bson:"phone,omitempty"`
	PlaidConnections          []plaidConnectionDoc     `bson:"plaid_connections,omitempty"`
	BrokerageConn             *brokerageConnectionDoc  `bson:"brokerage_connection,omitempty"`  // legacy single-connection field
	BrokerageConns            []brokerageConnectionDoc `bson:"brokerage_connections,omitempty"` // multi-connection array
	PortfolioConn             *portfolioConnectionDoc  `bson:"portfolio_connection,omitempty"`
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

// GetBrokerageConnections returns all brokerage connections with decrypted credentials.
// If brokerage_connections array is empty, falls back to legacy brokerage_connection field
// and synthesizes a single entry with ID="default" — identical routing behavior to before.
func (r *MongoProfileRepository) GetBrokerageConnections(ctx context.Context, userID string) ([]models.BrokerageConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var doc profileDocument
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: get brokerage connections: %w", err)
	}

	// Use new multi-connection array if populated.
	if len(doc.BrokerageConns) > 0 {
		return r.decryptBrokerageConns(doc.BrokerageConns)
	}

	// Backward-compat: synthesize from legacy single-connection field on read (no auto-save).
	if doc.BrokerageConn != nil && doc.BrokerageConn.Connected {
		apiKey, err := DecryptToken(doc.BrokerageConn.APIKey)
		if err != nil {
			return nil, fmt.Errorf("mongo profile repo: decrypt legacy brokerage api key: %w", err)
		}
		secretKey, err := DecryptToken(doc.BrokerageConn.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("mongo profile repo: decrypt legacy brokerage secret key: %w", err)
		}
		return []models.BrokerageConnection{{
			ID:              "default",
			AssetCategories: []models.AssetCategory{models.AssetCategoryDefault},
			APIKey:          apiKey,
			SecretKey:       secretKey,
			BaseURL:         doc.BrokerageConn.BaseURL,
			Connected:       true,
			ConnectedAt:     doc.BrokerageConn.ConnectedAt,
		}}, nil
	}
	return nil, nil
}

func (r *MongoProfileRepository) decryptBrokerageConns(docs []brokerageConnectionDoc) ([]models.BrokerageConnection, error) {
	conns := make([]models.BrokerageConnection, 0, len(docs))
	for _, d := range docs {
		apiKey, err := DecryptToken(d.APIKey)
		if err != nil {
			return nil, fmt.Errorf("mongo profile repo: decrypt brokerage api key for %s: %w", d.ID, err)
		}
		secretKey, err := DecryptToken(d.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("mongo profile repo: decrypt brokerage secret key for %s: %w", d.ID, err)
		}
		cats := make([]models.AssetCategory, len(d.AssetCategories))
		for i, c := range d.AssetCategories {
			cats[i] = models.AssetCategory(c)
		}
		conns = append(conns, models.BrokerageConnection{
			ID:              d.ID,
			Name:            d.Name,
			AssetCategories: cats,
			APIKey:          apiKey,
			SecretKey:       secretKey,
			BaseURL:         d.BaseURL,
			Connected:       d.Connected,
			ConnectedAt:     d.ConnectedAt,
		})
	}
	return conns, nil
}

// UpsertBrokerageConnection adds or updates a connection in brokerage_connections by conn.ID.
// Uses arrayFilters to update in-place; falls back to $push when no match exists.
func (r *MongoProfileRepository) UpsertBrokerageConnection(ctx context.Context, userID string, conn models.BrokerageConnection) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	encAPIKey, err := EncryptToken(conn.APIKey)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt brokerage api key: %w", err)
	}
	encSecretKey, err := EncryptToken(conn.SecretKey)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt brokerage secret key: %w", err)
	}

	cats := make([]string, len(conn.AssetCategories))
	for i, c := range conn.AssetCategories {
		cats[i] = string(c)
	}

	doc := brokerageConnectionDoc{
		ID:              conn.ID,
		Name:            conn.Name,
		AssetCategories: cats,
		APIKey:          encAPIKey,
		SecretKey:       encSecretKey,
		BaseURL:         conn.BaseURL,
		Connected:       true,
		ConnectedAt:     conn.ConnectedAt,
	}

	filter := bson.M{"user_id": userID, "brokerage_connections.id": conn.ID}
	update := bson.M{"$set": bson.M{"brokerage_connections.$": doc}}
	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("mongo profile repo: upsert brokerage connection (update): %w", err)
	}
	if res.ModifiedCount > 0 {
		return nil
	}

	// No existing entry matched — append.
	filter2 := bson.M{"user_id": userID}
	push := bson.M{"$push": bson.M{"brokerage_connections": doc}}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter2, push, opts); err != nil {
		return fmt.Errorf("mongo profile repo: upsert brokerage connection (push): %w", err)
	}
	return nil
}

// RemoveBrokerageConnection removes the entry matching connectionID from brokerage_connections.
func (r *MongoProfileRepository) RemoveBrokerageConnection(ctx context.Context, userID string, connectionID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$pull": bson.M{"brokerage_connections": bson.M{"id": connectionID}}}
	if _, err := r.collection.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("mongo profile repo: remove brokerage connection: %w", err)
	}
	return nil
}

// SaveLegacySingleBrokerageConnection stores encrypted Alpaca credentials using the old
// single-connection field. Used only by the legacy POST /brokerage/connect endpoint.
func (r *MongoProfileRepository) SaveLegacySingleBrokerageConnection(ctx context.Context, userID string, conn models.BrokerageConnection) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	encAPIKey, err := EncryptToken(conn.APIKey)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt brokerage api key: %w", err)
	}
	encSecretKey, err := EncryptToken(conn.SecretKey)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt brokerage secret key: %w", err)
	}

	doc := brokerageConnectionDoc{
		APIKey:      encAPIKey,
		SecretKey:   encSecretKey,
		BaseURL:     conn.BaseURL,
		Connected:   true,
		ConnectedAt: conn.ConnectedAt,
	}

	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"brokerage_connection": doc}}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: save legacy brokerage connection: %w", err)
	}
	return nil
}

// ClearLegacySingleBrokerageConnection removes the legacy brokerage_connection field.
// Used only by the legacy DELETE /brokerage/connect endpoint.
func (r *MongoProfileRepository) ClearLegacySingleBrokerageConnection(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$unset": bson.M{"brokerage_connection": ""}}
	if _, err := r.collection.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("mongo profile repo: clear legacy brokerage connection: %w", err)
	}
	return nil
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

// SavePortfolioConnection stores the encrypted portfolio aggregator credentials for the user.
// Uses $set to replace any existing portfolio_connection — one aggregator per user.
func (r *MongoProfileRepository) SavePortfolioConnection(ctx context.Context, userID string, conn models.PortfolioConnection) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	encUserID, err := EncryptToken(conn.ProviderUserID)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt portfolio provider user id: %w", err)
	}
	encSecret, err := EncryptToken(conn.ProviderUserSecret)
	if err != nil {
		return fmt.Errorf("mongo profile repo: encrypt portfolio provider user secret: %w", err)
	}

	doc := portfolioConnectionDoc{
		Provider:           conn.Provider,
		ProviderUserID:     encUserID,
		ProviderUserSecret: encSecret,
		ConnectedAt:        conn.ConnectedAt,
	}

	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"portfolio_connection": doc}}
	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("mongo profile repo: save portfolio connection: %w", err)
	}
	return nil
}

// GetPortfolioConnection returns the portfolio aggregator connection with decrypted credentials.
// Returns nil, nil when no connection exists.
func (r *MongoProfileRepository) GetPortfolioConnection(ctx context.Context, userID string) (*models.PortfolioConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var doc profileDocument
	err := r.collection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: get portfolio connection: %w", err)
	}
	if doc.PortfolioConn == nil {
		return nil, nil
	}

	providerUserID, err := DecryptToken(doc.PortfolioConn.ProviderUserID)
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: decrypt portfolio provider user id: %w", err)
	}
	providerUserSecret, err := DecryptToken(doc.PortfolioConn.ProviderUserSecret)
	if err != nil {
		return nil, fmt.Errorf("mongo profile repo: decrypt portfolio provider user secret: %w", err)
	}

	return &models.PortfolioConnection{
		Provider:           doc.PortfolioConn.Provider,
		ProviderUserID:     providerUserID,
		ProviderUserSecret: providerUserSecret,
		ConnectedAt:        doc.PortfolioConn.ConnectedAt,
	}, nil
}

// ClearPortfolioConnection removes the portfolio_connection field from the user document.
func (r *MongoProfileRepository) ClearPortfolioConnection(ctx context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	update := bson.M{"$unset": bson.M{"portfolio_connection": ""}}
	if _, err := r.collection.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("mongo profile repo: clear portfolio connection: %w", err)
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
		IncludeCashContext:        doc.IncludeCashContext,
		NotificationEmail:         doc.NotificationEmail,
		Phone:                     doc.Phone,
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

	// Populate Brokerages — credentials are never included in the public profile.
	if len(doc.BrokerageConns) > 0 {
		profile.Brokerages = make([]models.BrokerageStatus, len(doc.BrokerageConns))
		for i, c := range doc.BrokerageConns {
			cats := make([]models.AssetCategory, len(c.AssetCategories))
			for j, cat := range c.AssetCategories {
				cats[j] = models.AssetCategory(cat)
			}
			profile.Brokerages[i] = models.BrokerageStatus{
				ID:              c.ID,
				Name:            c.Name,
				AssetCategories: cats,
				Connected:       c.Connected,
				BaseURL:         c.BaseURL,
				ConnectedAt:     c.ConnectedAt.Format(time.RFC3339),
			}
		}
	} else if doc.BrokerageConn != nil && doc.BrokerageConn.Connected {
		// Backward-compat: synthesize single-element slice from legacy field.
		profile.Brokerages = []models.BrokerageStatus{{
			ID:              "default",
			AssetCategories: []models.AssetCategory{models.AssetCategoryDefault},
			Connected:       doc.BrokerageConn.Connected,
			BaseURL:         doc.BrokerageConn.BaseURL,
			ConnectedAt:     doc.BrokerageConn.ConnectedAt.Format(time.RFC3339),
		}}
	}

	if doc.PortfolioConn != nil {
		profile.PortfolioAggregator = &models.PortfolioConnectionStatus{
			Provider:    doc.PortfolioConn.Provider,
			Connected:   true,
			ConnectedAt: doc.PortfolioConn.ConnectedAt.Format(time.RFC3339),
		}
	}
	return profile
}

func fromProfile(p *models.UserProfile) *profileDocument {
	// PlaidConnections and BrokerageConn are intentionally omitted — $set must never overwrite them.
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
		IncludeCashContext:        p.IncludeCashContext,
		NotificationEmail:         p.NotificationEmail,
		Phone:                     p.Phone,
	}
}

