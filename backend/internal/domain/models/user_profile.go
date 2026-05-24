// domain/models/user_profile.go
package models

import "time"

type TimeHorizon string

const (
	TimeUnder1Year TimeHorizon = "under_1_year"
	TimeOneToFive  TimeHorizon = "one_to_five"
	TimeFiveToTen  TimeHorizon = "five_to_ten"
	TimeTenPlus    TimeHorizon = "ten_plus"
)

type ImmigrationStatus string

const (
	ImmigrationUSCitizen    ImmigrationStatus = "us_citizen"
	ImmigrationPermResident ImmigrationStatus = "permanent_resident"
	ImmigrationWorkVisa     ImmigrationStatus = "work_visa"
	ImmigrationOther        ImmigrationStatus = "other"
)

type RiskTolerance string

const (
	RiskConservative RiskTolerance = "conservative"
	RiskModerate     RiskTolerance = "moderate"
	RiskAggressive   RiskTolerance = "aggressive"
)

type InvestmentGoal string

const (
	GoalWealthBuilding   InvestmentGoal = "wealth_building"
	GoalRetirement       InvestmentGoal = "retirement"
	GoalEmergencyFund    InvestmentGoal = "emergency_fund"
	GoalShortTermSavings InvestmentGoal = "short_term_savings"
)

// AssetCategory classifies which asset types a brokerage connection handles for routing.
type AssetCategory string

const (
	AssetCategoryEquity  AssetCategory = "equity"
	AssetCategoryBond    AssetCategory = "bond"
	AssetCategoryDefault AssetCategory = "default" // catches any allocation not matched by a specific category
)

// BrokerageConnection holds Alpaca credentials for a user's connected brokerage account.
// APIKey and SecretKey are stored AES-256-GCM encrypted in MongoDB; the infrastructure
// layer decrypts them before returning to callers — never stored or logged in plaintext.
type BrokerageConnection struct {
	ID              string          // stable identifier; "default" for legacy single-connection users
	Name            string          // display label, e.g. "Bonds Account"
	AssetCategories []AssetCategory // which asset types this connection handles
	APIKey          string          // encrypted at rest; decrypted by infrastructure layer before use
	SecretKey       string          // encrypted at rest; decrypted by infrastructure layer before use
	BaseURL         string
	Connected       bool
	ConnectedAt     time.Time
}

// BrokerageStatus is the safe subset returned to API callers — credentials are never included.
type BrokerageStatus struct {
	ID              string          `json:"id"`
	Name            string          `json:"name,omitempty"`
	AssetCategories []AssetCategory `json:"asset_categories,omitempty"`
	Connected       bool            `json:"connected"`
	BaseURL         string          `json:"base_url,omitempty"`
	ConnectedAt     string          `json:"connected_at,omitempty"`
}

// AssetClassLimit constrains how much of the portfolio can be allocated to one asset class.
// AssetClass matches the values Claude uses in Allocation.AssetClass (e.g. "Crypto", "US Equity", "Bonds").
// Zero MinPct means no floor; zero MaxPct means no ceiling.
type AssetClassLimit struct {
	AssetClass string  `json:"asset_class"`
	MinPct     float64 `json:"min_pct,omitempty"`
	MaxPct     float64 `json:"max_pct,omitempty"`
}

// AllocationPreferences are the user's hard constraints on portfolio composition.
// Passed to Claude as rules it must not violate, and used by the rebalancing checker
// to detect when actual weights have drifted outside the user's defined bounds.
type AllocationPreferences struct {
	AssetClassLimits   []AssetClassLimit `json:"asset_class_limits,omitempty"`
	MaxSingleTickerPct float64           `json:"max_single_ticker_pct,omitempty"` // e.g. 15 = no ticker >15% of portfolio
}

type UserProfile struct {
	UserID                    string                   `json:"user_id"`
	FullName                  string                   `json:"full_name"`
	Salary                    float64                  `json:"salary"`
	MonthlySavings            float64                  `json:"monthly_savings"`
	RetirementContributionPct float64                  `json:"retirement_contribution_percent"`
	ExistingPortfolioValue    float64                  `json:"existing_portfolio_value"`
	TimeHorizon               TimeHorizon              `json:"time_horizon"`
	ImmigrationStatus         ImmigrationStatus        `json:"immigration_status"`
	RiskTolerance             RiskTolerance            `json:"risk_tolerance"`
	InvestmentGoal            InvestmentGoal           `json:"investment_goal"`
	HasEmergencyFund          bool                     `json:"has_emergency_fund"`
	IncludeCashContext        bool                     `json:"include_cash_context"`
	NotificationEmail         string                   `json:"notification_email,omitempty"`
	Phone                     string                   `json:"phone,omitempty"`
	AllocationPreferences     *AllocationPreferences   `json:"allocation_preferences,omitempty"`
	Brokerages                []BrokerageStatus          `json:"brokerages,omitempty"`
	ConnectedAccounts         []PlaidConnectionSummary   `json:"connected_accounts,omitempty"`
	PortfolioAggregator       *PortfolioConnectionStatus `json:"portfolio_aggregator,omitempty"`
}
