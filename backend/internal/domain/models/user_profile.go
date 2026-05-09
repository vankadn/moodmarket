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

// BrokerageConnection holds Alpaca credentials for a user's connected brokerage account.
// APIKey and SecretKey are stored AES-256-GCM encrypted in MongoDB; the infrastructure
// layer decrypts them before returning to callers — never stored or logged in plaintext.
type BrokerageConnection struct {
	APIKey      string    // encrypted at rest; decrypted by infrastructure layer before use
	SecretKey   string    // encrypted at rest; decrypted by infrastructure layer before use
	BaseURL     string
	Connected   bool
	ConnectedAt time.Time
}

// BrokerageStatus is the safe subset returned to API callers — credentials are never included.
type BrokerageStatus struct {
	Connected   bool   `json:"connected"`
	BaseURL     string `json:"base_url,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
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
	IncludeCashContext        bool                     `json:"include_cash_context"`          // user opted in to cash-context signal in Claude prompt
	Brokerage                 *BrokerageStatus         `json:"brokerage,omitempty"`           // populated by repository, never written back on save
	ConnectedAccounts         []PlaidConnectionSummary `json:"connected_accounts,omitempty"`  // institution + item_id only; populated by repository, never written back on save
}
