package transformer

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
)

type DummyTransformer struct{}

func NewDummyTransformer() *DummyTransformer {
	return &DummyTransformer{}
}

// Dummy payload structure
type DummyArticle struct {
	ID        string `json:"id"`
	Headline  string `json:"headline"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type DummyResponse struct {
	Items []DummyArticle `json:"items"`
}

func (t *DummyTransformer) Transform(reader io.Reader) ([]domain.Article, *domain.PageInfo, error) {
	var resp DummyResponse
	if err := json.NewDecoder(reader).Decode(&resp); err != nil {
		return nil, nil, fmt.Errorf("failed to decode dummy response: %w", err)
	}

	articles := make([]domain.Article, 0, len(resp.Items))
	for _, item := range resp.Items {
		ts, _ := time.Parse(time.RFC3339, item.Timestamp)
		articles = append(articles, domain.Article{
			ID:          "dummy_" + item.ID,
			ExternalID:  item.ID,
			Source:      "dummy",
			Title:       item.Headline,
			Summary:     item.Content[:10] + "...",
			Body:        item.Content,
			PublishedAt: ts,
			UpdatedAt:   ts,
			URL:         "http://dummy/" + item.ID,
		})
	}

	// Mock PageInfo for pagination testing
	pageInfo := &domain.PageInfo{
		Page:       0,
		NumPages:   1, // Only 1 page for dummy
		PageSize:   20,
		NumEntries: len(articles),
	}

	return articles, pageInfo, nil
}
