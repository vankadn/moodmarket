// infrastructure/news/polygon.go
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const polygonNewsURL = "https://api.polygon.io/v2/reference/news"

type polygonNewsProvider struct {
	apiKey       string
	articleLimit int
	httpClient   *http.Client

	mu          sync.Mutex
	cachedNews  []models.NewsItem
	cacheDate   string
}

func newPolygonNewsProvider(apiKey string, articleLimit int) *polygonNewsProvider {
	return &polygonNewsProvider{
		apiKey:       apiKey,
		articleLimit: articleLimit,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// GetDailyNews returns the cached headlines if fetched today, otherwise fetches from Polygon.
func (p *polygonNewsProvider) GetDailyNews(ctx context.Context) ([]models.NewsItem, error) {
	today := time.Now().Format("2006-01-02")

	p.mu.Lock()
	if p.cacheDate == today && p.cachedNews != nil {
		items := p.cachedNews
		p.mu.Unlock()
		log.Printf("[polygon-news] cache hit for %s (%d items)", today, len(items))
		return items, nil
	}
	p.mu.Unlock()

	items, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.cachedNews = items
	p.cacheDate = today
	p.mu.Unlock()

	log.Printf("[polygon-news] fetched %d headlines for %s", len(items), today)
	return items, nil
}

func (p *polygonNewsProvider) fetch(ctx context.Context) ([]models.NewsItem, error) {
	url := fmt.Sprintf("%s?ticker=SPY&limit=%d&apiKey=%s", polygonNewsURL, p.articleLimit, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("polygon-news: build request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polygon-news: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("polygon-news: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polygon-news: API %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Results []struct {
			Title        string `json:"title"`
			Description  string `json:"description"`
			PublishedUTC string `json:"published_utc"`
			Publisher    struct {
				Name string `json:"name"`
			} `json:"publisher"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("polygon-news: parse: %w", err)
	}

	items := make([]models.NewsItem, 0, len(result.Results))
	for _, r := range result.Results {
		publishedAt, _ := time.Parse(time.RFC3339, r.PublishedUTC)
		items = append(items, models.NewsItem{
			Headline:    r.Title,
			Summary:     r.Description,
			Source:      r.Publisher.Name,
			PublishedAt: publishedAt,
		})
	}
	return items, nil
}
