package transformer

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/SportsNewsCrawler/internal/domain"
)

const PulseLiveName = "pulselive"

// PageInfo contains pagination metadata from the API response.
type PageInfo struct {
	Page       int `json:"page"`
	NumPages   int `json:"numPages"`
	PageSize   int `json:"pageSize"`
	NumEntries int `json:"numEntries"`
}

type PulseLiveArticle struct {
	ID           int    `json:"id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Summary      string `json:"summary"`
	Body         string `json:"body"`
	Date         string `json:"date"`
	LastModified int64  `json:"lastModified"`
	CanonicalURL string `json:"canonicalUrl"`
	Tags         []struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
	} `json:"tags"`
	LeadMedia struct {
		ImageURL string `json:"imageUrl"`
	} `json:"leadMedia"`
}

type PulseLiveResponse struct {
	PageInfo PageInfo           `json:"pageInfo"`
	Content  []PulseLiveArticle `json:"content"`
}

type PulseLiveTransformer struct{}

func NewPulseLiveTransformer() *PulseLiveTransformer {
	return &PulseLiveTransformer{}
}

func (t *PulseLiveTransformer) Transform(reader io.Reader) ([]domain.Article, error) {
	var pulseResp PulseLiveResponse
	if err := json.NewDecoder(reader).Decode(&pulseResp); err != nil {
		return nil, fmt.Errorf("failed to decode pulse live response: %w", err)
	}

	articles := make([]domain.Article, 0, len(pulseResp.Content))
	for _, pa := range pulseResp.Content {
		articles = append(articles, t.normalize(pa))
	}

	return articles, nil
}

func (t *PulseLiveTransformer) normalize(pa PulseLiveArticle) domain.Article {
	// Use description if available, fallback to summary
	desc := pa.Description
	if desc == "" {
		desc = pa.Summary
	}

	// Parse publication date (prefer 'date' field over 'lastModified')
	pubDate := time.Unix(pa.LastModified/1000, 0)
	if pa.Date != "" {
		if parsedDate, err := time.Parse(time.RFC3339, pa.Date); err == nil {
			pubDate = parsedDate
		}
	}

	// Convert tags
	tags := make([]domain.Tag, len(pa.Tags))
	for i, tag := range pa.Tags {
		tags[i] = domain.Tag{
			ID:    tag.ID,
			Label: tag.Label,
		}
	}

	return domain.Article{
		ID:          fmt.Sprintf("%s_%d", PulseLiveName, pa.ID),
		ExternalID:  fmt.Sprintf("%d", pa.ID),
		Source:      PulseLiveName,
		Type:        pa.Type,
		Title:       pa.Title,
		Description: desc,
		Summary:     pa.Summary,
		Body:        pa.Body,
		PublishedAt: pubDate,
		UpdatedAt:   time.Unix(pa.LastModified/1000, 0),
		URL:         pa.CanonicalURL,
		ImageURL:    pa.LeadMedia.ImageURL,
		Tags:        tags,
	}
}
