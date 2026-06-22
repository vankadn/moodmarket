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
		{Headline: "Fed holds rates steady, signals two cuts possible in 2026", Source: "Reuters", PublishedAt: now},
		{Headline: "S&P 500 rises on strong jobs report, tech leads gains", Source: "Bloomberg", PublishedAt: now.Add(-1 * time.Hour)},
		{Headline: "Oil prices dip as OPEC+ output deal extended through Q3", Source: "CNBC", PublishedAt: now.Add(-2 * time.Hour)},
		{Headline: "NVIDIA beats earnings estimates, raises guidance on AI demand", Source: "Reuters", PublishedAt: now.Add(-3 * time.Hour)},
		{Headline: "Treasury yields fall as inflation data comes in below forecast", Source: "Bloomberg", PublishedAt: now.Add(-4 * time.Hour)},
		{Headline: "Apple announces expanded share buyback program", Source: "WSJ", PublishedAt: now.Add(-5 * time.Hour)},
		{Headline: "China manufacturing PMI contracts for second straight month", Source: "Reuters", PublishedAt: now.Add(-6 * time.Hour)},
		{Headline: "Dollar strengthens against euro on resilient US consumer spending", Source: "FT", PublishedAt: now.Add(-7 * time.Hour)},
		{Headline: "Microsoft Azure revenue growth accelerates on enterprise AI adoption", Source: "Bloomberg", PublishedAt: now.Add(-8 * time.Hour)},
		{Headline: "Gold hits six-month high as geopolitical tensions persist", Source: "CNBC", PublishedAt: now.Add(-9 * time.Hour)},
		{Headline: "Retail sales beat expectations, consumer confidence index rises", Source: "Reuters", PublishedAt: now.Add(-10 * time.Hour)},
		{Headline: "Biotech sector rallies after FDA approves new oncology treatment", Source: "Bloomberg", PublishedAt: now.Add(-11 * time.Hour)},
		{Headline: "Housing starts decline as mortgage rates remain elevated", Source: "WSJ", PublishedAt: now.Add(-12 * time.Hour)},
		{Headline: "European Central Bank cuts rates by 25bp, third reduction this year", Source: "FT", PublishedAt: now.Add(-13 * time.Hour)},
		{Headline: "Semiconductor supply chain stabilizes; analysts raise price targets", Source: "CNBC", PublishedAt: now.Add(-14 * time.Hour)},
	}, nil
}
