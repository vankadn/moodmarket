// domain/ports/news.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// NewsProvider is the port any market news source must implement.
// Implementations must degrade gracefully — a fetch failure must never
// block a recommendation.
type NewsProvider interface {
	GetDailyNews(ctx context.Context) ([]models.NewsItem, error)
}
