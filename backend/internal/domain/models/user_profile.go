package models

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

type UserProfile struct {
	UserID                    string            `json:"user_id"`
	FullName                  string            `json:"full_name"`
	Salary                    float64           `json:"salary"`
	MonthlySavings            float64           `json:"monthly_savings"`
	RetirementContributionPct float64           `json:"retirement_contribution_percent"`
	ExistingPortfolioValue    float64           `json:"existing_portfolio_value"`
	TimeHorizon               TimeHorizon       `json:"time_horizon"`
	ImmigrationStatus         ImmigrationStatus `json:"immigration_status"`
	RiskTolerance             RiskTolerance     `json:"risk_tolerance"`
	InvestmentGoal            InvestmentGoal    `json:"investment_goal"`
	HasEmergencyFund          bool              `json:"has_emergency_fund"`
}
