// api/handlers/portfolio_accounts_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type portfolioAccountsRepo interface {
	GetPortfolioConnection(ctx context.Context, userID string) (*models.PortfolioConnection, error)
}

type portfolioAccountsAggregator interface {
	ListAccounts(ctx context.Context, providerUserID, providerUserSecret string) ([]models.LinkedAccount, error)
}

type portfolioAccountsHandler struct {
	repo       portfolioAccountsRepo
	aggregator portfolioAccountsAggregator
	identity   ports.IdentityProvider
}

func NewPortfolioAccountsHandler(repo portfolioAccountsRepo, aggregator portfolioAccountsAggregator, identity ports.IdentityProvider) http.Handler {
	return &portfolioAccountsHandler{repo: repo, aggregator: aggregator, identity: identity}
}

func (h *portfolioAccountsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	conn, err := h.repo.GetPortfolioConnection(ctx, userID)
	if err != nil {
		log.Printf("[portfolio-accounts] get connection for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	accounts := []models.LinkedAccount{}
	if conn != nil {
		accounts, err = h.aggregator.ListAccounts(ctx, conn.ProviderUserID, conn.ProviderUserSecret)
		if err != nil {
			log.Printf("[portfolio-accounts] list accounts for user %s: %v", userID, err)
			http.Error(w, "failed to list linked accounts", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct { //nolint:errcheck
		Accounts []models.LinkedAccount `json:"accounts"`
	}{Accounts: accounts})
}
