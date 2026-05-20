// api/handlers/portfolio_connect_handler.go
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

// portfolioConnector is a local interface so this handler never imports infrastructure directly.
type portfolioConnector interface {
	RegisterUser(ctx context.Context, userID string) (providerUserID, providerUserSecret string, err error)
	GenerateConnectURL(ctx context.Context, providerUserID, providerUserSecret string) (redirectURL string, err error)
	DeleteUser(ctx context.Context, providerUserID, providerUserSecret string) error
}

// portfolioConnectRepo is a local interface scoped to what this handler needs.
type portfolioConnectRepo interface {
	SavePortfolioConnection(ctx context.Context, userID string, conn models.PortfolioConnection) error
	GetPortfolioConnection(ctx context.Context, userID string) (*models.PortfolioConnection, error)
	ClearPortfolioConnection(ctx context.Context, userID string) error
}

// PortfolioConnectHandler handles POST /portfolio/connect and DELETE /portfolio/connect.
type PortfolioConnectHandler struct {
	connector   portfolioConnector
	profileRepo portfolioConnectRepo
	identity    ports.IdentityProvider
}

func NewPortfolioConnectHandler(connector portfolioConnector, profileRepo portfolioConnectRepo, identity ports.IdentityProvider) *PortfolioConnectHandler {
	return &PortfolioConnectHandler{connector: connector, profileRepo: profileRepo, identity: identity}
}

func (h *PortfolioConnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.connect(w, r)
	case http.MethodDelete:
		h.disconnect(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// connect registers the user with the portfolio aggregator and returns a redirect URL for broker linking.
// Idempotent: re-uses existing registration if already connected, generating a fresh connect URL.
func (h *PortfolioConnectHandler) connect(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	existing, err := h.profileRepo.GetPortfolioConnection(ctx, userID)
	if err != nil {
		log.Printf("[portfolio-connect] get existing connection for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var providerUserID, providerUserSecret string
	if existing != nil {
		// Already registered — re-use stored credentials to generate a fresh connect URL.
		providerUserID = existing.ProviderUserID
		providerUserSecret = existing.ProviderUserSecret
		log.Printf("[portfolio-connect] user=%s already registered, generating fresh connect URL", userID)
	} else {
		providerUserID, providerUserSecret, err = h.connector.RegisterUser(ctx, userID)
		if err != nil {
			log.Printf("[portfolio-connect] register user %s: %v", userID, err)
			http.Error(w, "failed to register with portfolio provider", http.StatusInternalServerError)
			return
		}
		// Persist credentials before generating the URL so a URL failure leaves no dangling registration.
		conn := models.PortfolioConnection{
			Provider:           "snaptrade",
			ProviderUserID:     providerUserID,
			ProviderUserSecret: providerUserSecret,
			ConnectedAt:        time.Now(),
		}
		if err := h.profileRepo.SavePortfolioConnection(ctx, userID, conn); err != nil {
			log.Printf("[portfolio-connect] save connection for user %s: %v", userID, err)
			// Registration succeeded but we couldn't persist — roll back to avoid an orphaned provider user.
			if delErr := h.connector.DeleteUser(ctx, providerUserID, providerUserSecret); delErr != nil {
				log.Printf("[portfolio-connect] rollback delete user %s: %v", userID, delErr)
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		log.Printf("[portfolio-connect] user=%s registered and credentials persisted", userID)
	}

	redirectURL, err := h.connector.GenerateConnectURL(ctx, providerUserID, providerUserSecret)
	if err != nil {
		log.Printf("[portfolio-connect] generate connect URL for user %s: %v", userID, err)
		// Roll back only a fresh registration — don't delete a pre-existing one.
		if existing == nil {
			if delErr := h.connector.DeleteUser(ctx, providerUserID, providerUserSecret); delErr != nil {
				log.Printf("[portfolio-connect] rollback delete user %s: %v", userID, delErr)
			}
			if clearErr := h.profileRepo.ClearPortfolioConnection(ctx, userID); clearErr != nil {
				log.Printf("[portfolio-connect] rollback clear connection for user %s: %v", userID, clearErr)
			}
		}
		http.Error(w, "failed to generate connection URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		RedirectURL string `json:"redirect_url"`
	}{RedirectURL: redirectURL})
}

// disconnect removes the user's portfolio aggregator connection.
func (h *PortfolioConnectHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	conn, err := h.profileRepo.GetPortfolioConnection(ctx, userID)
	if err != nil {
		log.Printf("[portfolio-connect] get connection for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if conn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Best-effort de-registration — continue with local clear even if provider call fails.
	if err := h.connector.DeleteUser(ctx, conn.ProviderUserID, conn.ProviderUserSecret); err != nil {
		log.Printf("[portfolio-connect] delete provider user for %s: %v — continuing with local clear", userID, err)
	}

	if err := h.profileRepo.ClearPortfolioConnection(ctx, userID); err != nil {
		log.Printf("[portfolio-connect] clear connection for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[portfolio-connect] user=%s disconnected portfolio aggregator", userID)
	w.WriteHeader(http.StatusNoContent)
}
