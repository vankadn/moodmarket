// domain/ports/fundamentals.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// FundamentalsProvider fetches company-level fundamental data used by value-style strategies.
// The Finnhub implementation lives in infrastructure/fundamentals — this interface is the
// only thing the application layer ever touches.
type FundamentalsProvider interface {
	GetFundamentals(ctx context.Context, ticker string) (*models.Fundamentals, error)
	GetEarningsSurprises(ctx context.Context, ticker string, limit int) ([]models.EarningsSurprise, error)
	GetInsiderActivity(ctx context.Context, ticker string) (*models.InsiderActivity, error)
}
