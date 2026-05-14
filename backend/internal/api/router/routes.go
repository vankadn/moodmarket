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
	AutoInvestConfigURI = "/users/auto-invest/config"
	CashContextURI      = "/users/cash-context"
	ActivityURI         = "/users/activity"

	PlaidLinkTokenURI = "/plaid/link-token"
	PlaidExchangeURI  = "/plaid/exchange"
	PlaidAccountsURI  = "/plaid/accounts/" // subtree — item_id is path suffix

	BrokerageConnectURI = "/brokerage/connect"

	OrdersURI = "/orders/" // subtree — order_id is path suffix

	DocumentsURI       = "/documents"
	DocumentsUploadURI = "/documents/upload"
	DocumentsByIDURI   = "/documents/" // subtree — doc_id is path suffix

	DocsURI      = "/docs"
	DocsSlashURI = "/docs/" // subtree — covers /docs/ and /docs/openapi.yaml
)
