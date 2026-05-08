// domain/models/news.go
package models

import "time"

// NewsItem is a single market-relevant headline fetched from a news provider.
type NewsItem struct {
	Headline    string
	Summary     string
	Source      string
	PublishedAt time.Time
}
