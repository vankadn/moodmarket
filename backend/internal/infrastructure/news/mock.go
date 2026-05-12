// infrastructure/news/mock.go
package news

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockNewsProvider struct{}

func newMockNewsProvider() *mockNewsProvider { return &mockNewsProvider{} }

func (m *mockNewsProvider) GetDailyNews(_ context.Context) ([]models.NewsItem, error) {
	now := time.Now()
	return []models.NewsItem{
		{
			Headline:    "Loud holds rates steady, signals two cuts possible in 2026",
			Source:      "Reuters",
			PublishedAt: now,
		},
		{
			Headline:    "S&P 500 rises on strong jobs report, tech leads gains",
			Source:      "Bloomberg",
			PublishedAt: now.Add(-2 * time.Hour),
		},
		{
			Headline:    "Oil prices dip as OPEC+ output deal extended through Q3",
			Source:      "CNBC",
			PublishedAt: now.Add(-4 * time.Hour),
		},
	}, nil
}
