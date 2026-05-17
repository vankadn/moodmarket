package router

// URI constants — single source of truth for every path in the API.
// Use these in route registration (router.go), tests, and documentation.
// Naming: resource + action (if ambiguous) + URI postfix.
const (
	HealthURI = "/health"

	DevLoginURI = "/auth/dev-login"

	RecommendURI = "/recommend"
	InvestURI    = "/invest"

	ProfileURI          = "/users/profile"
	AutoInvestConfigsURI    = "/users/auto-invest/configs"
	AutoInvestConfigsByIDURI = "/users/auto-invest/configs/" // subtree — config_id is path suffix
	AutoInvestConfigURI     = "/users/auto-invest/config"
	CashContextURI      = "/users/cash-context"
	NotificationsURI    = "/users/notifications"
	ActivityURI             = "/users/activity"
	ActivityByStrategyURI   = "/users/activity/by-strategy"
	ActivityStrategyPnLURI  = "/users/activity/by-strategy/pnl"
	EvalSummaryURI          = "/users/eval/summary"
	EvalDecisionsURI        = "/users/eval/decisions"

	PlaidLinkTokenURI = "/plaid/link-token"
	PlaidExchangeURI  = "/plaid/exchange"
	PlaidAccountsURI  = "/plaid/accounts/" // subtree — item_id is path suffix

	BrokerageConnectURI       = "/brokerage/connect"
	BrokerageConnectionsURI   = "/brokerage/connections"
	BrokerageConnectionByIDURI = "/brokerage/connections/" // subtree — connection_id is path suffix

	PortfolioURI        = "/portfolio"
	PortfolioHistoryURI = "/portfolio/history"

	OrdersURI = "/orders/" // subtree — order_id is path suffix

	DocumentsURI       = "/documents"
	DocumentsUploadURI = "/documents/upload"
	DocumentsByIDURI   = "/documents/" // subtree — doc_id is path suffix

	DocsURI      = "/docs"
	DocsSlashURI = "/docs/" // subtree — covers /docs/ and /docs/openapi.yaml
)
