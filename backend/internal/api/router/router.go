package router

import (
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
	"github.com/krishnarajivvns/investiq/internal/middleware"
)

// Handlers holds every pre-constructed http.Handler / http.HandlerFunc.
// main.go builds each handler and populates this struct; router.Build wires them to paths.
// Wrapping a method-style handler: http.HandlerFunc(h.Method).
type Handlers struct {
	DevLogin              http.Handler
	Recommend             http.Handler
	Invest                http.Handler
	Profile               http.Handler
	Plaid                 http.Handler
	AutoInvest            http.Handler
	AutoInvestConfigs     http.Handler
	CashContext           http.Handler
	Activity              http.Handler // wrap with http.HandlerFunc if needed
	ActivityByStrategy    http.Handler
	ActivityStrategyPnL   http.Handler
	Brokerage             http.Handler
	BrokerageConnections  http.Handler
	Notifications         http.Handler
	Portfolio             http.Handler
	Order                 http.Handler // wrap with http.HandlerFunc if needed
	Document              http.Handler
	Docs                  http.Handler
	Eval                  http.Handler
}

// Build returns the top-level http.Handler for the server.
//
// Two-tier mux:
//   - top: /health and /docs/* registered without auth (Railway healthchecks, Swagger UI)
//   - protected mux: every other route wrapped in CORS + UserIdentity middleware
func Build(h Handlers, authProvider ports.AuthProvider) http.Handler {
	protected := http.NewServeMux()
	protected.Handle(DevLoginURI, h.DevLogin)
	protected.Handle(RecommendURI, h.Recommend)
	protected.Handle(InvestURI, h.Invest)
	protected.Handle(ProfileURI, h.Profile)
	protected.Handle(PlaidLinkTokenURI, h.Plaid)
	protected.Handle(PlaidExchangeURI, h.Plaid)
	protected.Handle(PlaidAccountsURI, h.Plaid)
	protected.Handle(AutoInvestConfigsByIDURI, h.AutoInvestConfigs) // subtree for /:id — must be before base
	protected.Handle(AutoInvestConfigsURI, h.AutoInvestConfigs)
	protected.Handle(AutoInvestConfigURI, h.AutoInvest)
	protected.Handle(CashContextURI, h.CashContext)
	protected.Handle(NotificationsURI, h.Notifications)
	protected.Handle(ActivityStrategyPnLURI, h.ActivityStrategyPnL) // most specific — register first
	protected.Handle(ActivityByStrategyURI, h.ActivityByStrategy)
	protected.Handle(ActivityURI, h.Activity)
	protected.Handle(EvalSummaryURI, h.Eval)
	protected.Handle(EvalDecisionsURI, h.Eval)
	protected.Handle(BrokerageConnectURI, h.Brokerage)
	protected.Handle(BrokerageConnectionsURI, h.BrokerageConnections)
	protected.Handle(BrokerageConnectionByIDURI, h.BrokerageConnections)
	protected.Handle(PortfolioHistoryURI, h.Portfolio) // must register before PortfolioURI (more specific)
	protected.Handle(PortfolioURI, h.Portfolio)
	protected.Handle(OrdersURI, h.Order)
	protected.Handle(DocumentsUploadURI, h.Document) // must be registered before DocumentsByIDURI
	protected.Handle(DocumentsURI, h.Document)
	protected.Handle(DocumentsByIDURI, h.Document)

	top := http.NewServeMux()
	top.HandleFunc(HealthURI, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	top.Handle(DocsSlashURI, h.Docs)
	top.Handle(DocsURI, http.RedirectHandler(DocsSlashURI, http.StatusMovedPermanently))
	top.Handle("/", middleware.CORS(middleware.UserIdentity(authProvider, protected)))

	return top
}
